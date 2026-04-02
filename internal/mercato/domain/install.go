package domain

import "slices"

// InstallDatabase is the top-level structure persisted as installed.json.
type InstallDatabase struct {
	Markets []InstalledMarket `json:"markets"`
}

// InstalledMarket groups packages from one market.
type InstalledMarket struct {
	Market   string             `json:"market"`
	Packages []InstalledPackage `json:"packages"`
}

// InstalledPackage tracks a single installed profile/skill with its locations.
type InstalledPackage struct {
	Profile       string            `json:"profile"`
	Version       string            `json:"version"`
	Files         InstalledFiles    `json:"files"`
	Locations     []string          `json:"locations"`
	ToolChecksums map[string]string `json:"tool_checksums,omitempty"`
}

// InstalledFiles lists the leaf names of installed skills and agents.
type InstalledFiles struct {
	Skills []string `json:"skills,omitempty"`
	Agents []string `json:"agents,omitempty"`
}

// FindPackage returns a pointer to the installed package for the given market
// and profile, or nil if not found.
func (db *InstallDatabase) FindPackage(market, profile string) *InstalledPackage {
	m := db.FindMarket(market)
	if m == nil {
		return nil
	}
	for i := range m.Packages {
		if m.Packages[i].Profile == profile {
			return &m.Packages[i]
		}
	}
	return nil
}

// FindMarket returns a pointer to the installed market entry, or nil if not found.
func (db *InstallDatabase) FindMarket(market string) *InstalledMarket {
	for i := range db.Markets {
		if db.Markets[i].Market == market {
			return &db.Markets[i]
		}
	}
	return nil
}

// AddOrUpdatePackage upserts a package in the database. If the market doesn't
// exist it is created. If the package doesn't exist it is created. The location
// is appended only if not already present.
func (db *InstallDatabase) AddOrUpdatePackage(market, profile, version string, files InstalledFiles, location string) {
	m := db.FindMarket(market)
	if m == nil {
		db.Markets = append(db.Markets, InstalledMarket{Market: market})
		m = &db.Markets[len(db.Markets)-1]
	}

	var pkg *InstalledPackage
	for i := range m.Packages {
		if m.Packages[i].Profile == profile {
			pkg = &m.Packages[i]
			break
		}
	}
	if pkg == nil {
		m.Packages = append(m.Packages, InstalledPackage{Profile: profile})
		pkg = &m.Packages[len(m.Packages)-1]
	}

	pkg.Version = version

	// Merge files into existing rather than replacing, so adding a skill
	// to a profile that already has agents doesn't lose the agents.
	for _, s := range files.Skills {
		if !slices.Contains(pkg.Files.Skills, s) {
			pkg.Files.Skills = append(pkg.Files.Skills, s)
		}
	}
	for _, a := range files.Agents {
		if !slices.Contains(pkg.Files.Agents, a) {
			pkg.Files.Agents = append(pkg.Files.Agents, a)
		}
	}

	if !slices.Contains(pkg.Locations, location) {
		pkg.Locations = append(pkg.Locations, location)
	}
}

// RemoveLocation removes a location from a package. If the package has no
// remaining locations it is removed. If the market has no remaining packages
// it is removed.
func (db *InstallDatabase) RemoveLocation(market, profile, location string) {
	mi := -1
	for i := range db.Markets {
		if db.Markets[i].Market == market {
			mi = i
			break
		}
	}
	if mi < 0 {
		return
	}

	pi := -1
	for i := range db.Markets[mi].Packages {
		if db.Markets[mi].Packages[i].Profile == profile {
			pi = i
			break
		}
	}
	if pi < 0 {
		return
	}

	pkg := &db.Markets[mi].Packages[pi]
	locs := pkg.Locations[:0]
	for _, loc := range pkg.Locations {
		if loc != location {
			locs = append(locs, loc)
		}
	}
	pkg.Locations = locs

	if len(pkg.Locations) == 0 {
		db.Markets[mi].Packages = append(db.Markets[mi].Packages[:pi], db.Markets[mi].Packages[pi+1:]...)
	}

	if len(db.Markets[mi].Packages) == 0 {
		db.Markets = append(db.Markets[:mi], db.Markets[mi+1:]...)
	}
}

// CleanStaleLocations removes locations where the directory no longer exists.
// It returns the list of removed paths and cleans up empty packages and markets.
func (db *InstallDatabase) CleanStaleLocations(dirExists func(string) bool) []string {
	var removed []string

	for mi := len(db.Markets) - 1; mi >= 0; mi-- {
		m := &db.Markets[mi]
		for pi := len(m.Packages) - 1; pi >= 0; pi-- {
			pkg := &m.Packages[pi]
			locs := pkg.Locations[:0]
			for _, loc := range pkg.Locations {
				if dirExists(loc) {
					locs = append(locs, loc)
				} else {
					removed = append(removed, loc)
				}
			}
			pkg.Locations = locs

			if len(pkg.Locations) == 0 {
				m.Packages = append(m.Packages[:pi], m.Packages[pi+1:]...)
			}
		}

		if len(m.Packages) == 0 {
			db.Markets = append(db.Markets[:mi], db.Markets[mi+1:]...)
		}
	}

	return removed
}
