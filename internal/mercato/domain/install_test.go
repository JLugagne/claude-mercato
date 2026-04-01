package domain

import (
	"testing"
)

func TestFindPackage(t *testing.T) {
	db := InstallDatabase{
		Markets: []InstalledMarket{
			{
				Market: "acme",
				Packages: []InstalledPackage{
					{Profile: "web-dev", Version: "abc123"},
				},
			},
		},
	}

	t.Run("finds existing", func(t *testing.T) {
		pkg := db.FindPackage("acme", "web-dev")
		if pkg == nil {
			t.Fatal("expected to find package")
		}
		if pkg.Version != "abc123" {
			t.Fatalf("got version %q, want %q", pkg.Version, "abc123")
		}
	})

	t.Run("returns nil for missing market", func(t *testing.T) {
		pkg := db.FindPackage("nope", "web-dev")
		if pkg != nil {
			t.Fatal("expected nil for missing market")
		}
	})

	t.Run("returns nil for missing profile", func(t *testing.T) {
		pkg := db.FindPackage("acme", "nope")
		if pkg != nil {
			t.Fatal("expected nil for missing profile")
		}
	})
}

func TestFindMarket(t *testing.T) {
	db := InstallDatabase{
		Markets: []InstalledMarket{
			{Market: "acme"},
			{Market: "beta"},
		},
	}

	t.Run("finds existing", func(t *testing.T) {
		m := db.FindMarket("acme")
		if m == nil {
			t.Fatal("expected to find market")
		}
		if m.Market != "acme" {
			t.Fatalf("got market %q, want %q", m.Market, "acme")
		}
	})

	t.Run("returns nil for missing", func(t *testing.T) {
		m := db.FindMarket("nope")
		if m != nil {
			t.Fatal("expected nil for missing market")
		}
	})
}

func TestAddOrUpdatePackage(t *testing.T) {
	t.Run("new market and package", func(t *testing.T) {
		var db InstallDatabase
		db.AddOrUpdatePackage("acme", "web-dev", "v1", InstalledFiles{Skills: []string{"s1.md"}}, "/home/user/.claude")

		if len(db.Markets) != 1 {
			t.Fatalf("got %d markets, want 1", len(db.Markets))
		}
		if db.Markets[0].Market != "acme" {
			t.Fatalf("got market %q, want %q", db.Markets[0].Market, "acme")
		}
		pkg := db.FindPackage("acme", "web-dev")
		if pkg == nil {
			t.Fatal("expected to find package")
		}
		if pkg.Version != "v1" {
			t.Fatalf("got version %q, want %q", pkg.Version, "v1")
		}
		if len(pkg.Locations) != 1 || pkg.Locations[0] != "/home/user/.claude" {
			t.Fatalf("got locations %v, want [/home/user/.claude]", pkg.Locations)
		}
	})

	t.Run("existing market new package", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{{Profile: "old", Version: "v0"}}},
			},
		}
		db.AddOrUpdatePackage("acme", "web-dev", "v1", InstalledFiles{}, "/loc")

		if len(db.Markets) != 1 {
			t.Fatalf("got %d markets, want 1", len(db.Markets))
		}
		if len(db.Markets[0].Packages) != 2 {
			t.Fatalf("got %d packages, want 2", len(db.Markets[0].Packages))
		}
	})

	t.Run("existing package new location", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Version: "v1", Locations: []string{"/loc1"}},
				}},
			},
		}
		db.AddOrUpdatePackage("acme", "web-dev", "v2", InstalledFiles{Agents: []string{"a.md"}}, "/loc2")

		pkg := db.FindPackage("acme", "web-dev")
		if pkg.Version != "v2" {
			t.Fatalf("got version %q, want %q", pkg.Version, "v2")
		}
		if len(pkg.Locations) != 2 {
			t.Fatalf("got %d locations, want 2", len(pkg.Locations))
		}
	})

	t.Run("existing package existing location no duplicate", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Version: "v1", Locations: []string{"/loc1"}},
				}},
			},
		}
		db.AddOrUpdatePackage("acme", "web-dev", "v2", InstalledFiles{}, "/loc1")

		pkg := db.FindPackage("acme", "web-dev")
		if len(pkg.Locations) != 1 {
			t.Fatalf("got %d locations, want 1 (no duplicate)", len(pkg.Locations))
		}
	})
}

func TestRemoveLocation(t *testing.T) {
	t.Run("remove one of multiple locations", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Locations: []string{"/loc1", "/loc2"}},
				}},
			},
		}
		db.RemoveLocation("acme", "web-dev", "/loc1")

		pkg := db.FindPackage("acme", "web-dev")
		if pkg == nil {
			t.Fatal("package should still exist")
		}
		if len(pkg.Locations) != 1 || pkg.Locations[0] != "/loc2" {
			t.Fatalf("got locations %v, want [/loc2]", pkg.Locations)
		}
	})

	t.Run("remove last location removes package", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Locations: []string{"/loc1"}},
					{Profile: "other", Locations: []string{"/loc2"}},
				}},
			},
		}
		db.RemoveLocation("acme", "web-dev", "/loc1")

		pkg := db.FindPackage("acme", "web-dev")
		if pkg != nil {
			t.Fatal("package should have been removed")
		}
		if len(db.Markets[0].Packages) != 1 {
			t.Fatalf("got %d packages, want 1", len(db.Markets[0].Packages))
		}
	})

	t.Run("remove last package removes market", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Locations: []string{"/loc1"}},
				}},
			},
		}
		db.RemoveLocation("acme", "web-dev", "/loc1")

		if len(db.Markets) != 0 {
			t.Fatalf("got %d markets, want 0", len(db.Markets))
		}
	})
}

func TestCleanStaleLocations(t *testing.T) {
	t.Run("removes stale keeps valid", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Locations: []string{"/valid", "/stale"}},
				}},
			},
		}

		exists := func(path string) bool { return path == "/valid" }
		removed := db.CleanStaleLocations(exists)

		if len(removed) != 1 || removed[0] != "/stale" {
			t.Fatalf("got removed %v, want [/stale]", removed)
		}
		pkg := db.FindPackage("acme", "web-dev")
		if pkg == nil {
			t.Fatal("package should still exist")
		}
		if len(pkg.Locations) != 1 || pkg.Locations[0] != "/valid" {
			t.Fatalf("got locations %v, want [/valid]", pkg.Locations)
		}
	})

	t.Run("cleans empty packages and markets", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Locations: []string{"/stale1", "/stale2"}},
				}},
			},
		}

		exists := func(string) bool { return false }
		removed := db.CleanStaleLocations(exists)

		if len(removed) != 2 {
			t.Fatalf("got %d removed, want 2", len(removed))
		}
		if len(db.Markets) != 0 {
			t.Fatalf("got %d markets, want 0", len(db.Markets))
		}
	})

	t.Run("returns nil when nothing stale", func(t *testing.T) {
		db := InstallDatabase{
			Markets: []InstalledMarket{
				{Market: "acme", Packages: []InstalledPackage{
					{Profile: "web-dev", Locations: []string{"/valid"}},
				}},
			},
		}

		exists := func(string) bool { return true }
		removed := db.CleanStaleLocations(exists)

		if len(removed) != 0 {
			t.Fatalf("got %d removed, want 0", len(removed))
		}
		if len(db.Markets) != 1 {
			t.Fatalf("got %d markets, want 1", len(db.Markets))
		}
	})
}
