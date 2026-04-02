package tui

import "testing"

func TestProfileDisplayName(t *testing.T) {
	cases := []struct {
		category string
		want     string
	}{
		{"skills/lint", "lint"},
		{"dev/go-hexagonal", "go-hexagonal"},
		{"skills-catalog/test", "test"},
		{"standalone", "standalone"},
		{"a/b/c", "c"},
	}
	for _, tc := range cases {
		t.Run(tc.category, func(t *testing.T) {
			got := profileDisplayName(tc.category)
			if got != tc.want {
				t.Errorf("profileDisplayName(%q) = %q, want %q", tc.category, got, tc.want)
			}
		})
	}
}
