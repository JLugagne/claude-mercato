package service

// DoctorQueries exposes the read-only health audit. It never mutates disk
// or the install database, never performs network I/O, and is safe to run
// at any time.
type DoctorQueries interface {
	Doctor(opts DoctorOpts) (DoctorReport, error)
}

type DoctorOpts struct {
	Market string
}

// DoctorReport groups every health finding by category. Each slice is empty
// when the category has nothing to report.
type DoctorReport struct {
	ModifiedFiles      []DoctorFile     `json:"modified_files,omitempty"`
	LocallyDeleted     []DoctorFile     `json:"locally_deleted,omitempty"`
	StaleLocations     []DoctorLocation `json:"stale_locations,omitempty"`
	UpstreamRemoved    []DoctorFile     `json:"upstream_removed,omitempty"`
	OrphanedPackages   []DoctorPackage  `json:"orphaned_packages,omitempty"`
}

// DoctorFile identifies a single file finding (modified, locally deleted,
// upstream removed). Path is the local relative path under the project root.
type DoctorFile struct {
	Market   string `json:"market"`
	Profile  string `json:"profile"`
	Location string `json:"location"`
	Path     string `json:"path"`
	Kind     string `json:"kind,omitempty"` // for upstream removals: "skill" | "agent" | "command" | "hook"
}

// DoctorLocation identifies a project install path that no longer exists
// on disk.
type DoctorLocation struct {
	Market   string `json:"market"`
	Profile  string `json:"profile"`
	Location string `json:"location"`
}

// DoctorPackage flags a package whose market is no longer in the user's
// config — it was installed but the source is gone, so updates can never
// be reconciled.
type DoctorPackage struct {
	Market   string `json:"market"`
	Profile  string `json:"profile"`
	Location string `json:"location"`
}

// HasFindings reports whether the report contains at least one finding of
// any kind. Useful for shell-friendly exit codes.
func (r DoctorReport) HasFindings() bool {
	return len(r.ModifiedFiles)+len(r.LocallyDeleted)+len(r.StaleLocations)+len(r.UpstreamRemoved)+len(r.OrphanedPackages) > 0
}
