package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- TestMctRefParse ---

func TestMctRefParse(t *testing.T) {
	cases := []struct {
		ref         MctRef
		wantMarket  string
		wantRelPath string
		wantErr     bool
	}{
		{
			ref:         "market/path/to/file.md",
			wantMarket:  "market",
			wantRelPath: "path/to/file.md",
		},
		{
			ref:         "market/file.md",
			wantMarket:  "market",
			wantRelPath: "file.md",
		},
		{
			ref:     "noslash",
			wantErr: true,
		},
		{
			ref:     "market/",
			wantErr: true,
		},
		{
			ref:     "/nomarket",
			wantErr: true,
		},
		{
			ref:     "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.ref), func(t *testing.T) {
			market, relPath, err := tc.ref.Parse()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for ref=%q, got nil", tc.ref)
				}
				if !IsDomainError(err) {
					t.Errorf("expected DomainError, got: %T %v", err, err)
				}
				if !strings.Contains(err.Error(), "INVALID_REF") {
					t.Errorf("expected INVALID_REF error code, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if market != tc.wantMarket {
				t.Errorf("expected market=%q, got %q", tc.wantMarket, market)
			}
			if relPath != tc.wantRelPath {
				t.Errorf("expected relPath=%q, got %q", tc.wantRelPath, relPath)
			}
		})
	}
}

// --- TestMctRefMarket ---

func TestMctRefMarket(t *testing.T) {
	t.Run("valid ref returns market segment", func(t *testing.T) {
		ref := MctRef("mkt/agents/foo.md")
		got := ref.Market()
		if got != "mkt" {
			t.Errorf("expected %q, got %q", "mkt", got)
		}
	})

	t.Run("invalid ref returns empty string", func(t *testing.T) {
		ref := MctRef("noslash")
		got := ref.Market()
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

// --- TestMctRefRelPath ---

func TestMctRefRelPath(t *testing.T) {
	t.Run("valid ref returns relative path", func(t *testing.T) {
		ref := MctRef("mkt/agents/foo.md")
		got := ref.RelPath()
		if got != "agents/foo.md" {
			t.Errorf("expected %q, got %q", "agents/foo.md", got)
		}
	})

	t.Run("invalid ref returns full ref as fallback", func(t *testing.T) {
		ref := MctRef("noslash")
		got := ref.RelPath()
		if got != "noslash" {
			t.Errorf("expected %q (full ref fallback), got %q", "noslash", got)
		}
	})
}

// --- TestEntryStateString ---

func TestEntryStateString(t *testing.T) {
	cases := []struct {
		state EntryState
		want  string
	}{
		{StateClean, "clean"},
		{StateUpdateAvailable, "update_available"},
		{StateDrift, "drift"},
		{StateUpdateAndDrift, "update_and_drift"},
		{StateDeleted, "deleted"},
		{StateNewInRegistry, "new"},
		{StateOrphaned, "orphaned"},
		{StateUnknown, "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.state.String()
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// --- TestEntryStateMarshalJSON ---

func TestEntryStateMarshalJSON(t *testing.T) {
	t.Run("StateClean marshals to \"clean\"", func(t *testing.T) {
		b, err := json.Marshal(StateClean)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(b) != `"clean"` {
			t.Errorf("expected %q, got %q", `"clean"`, string(b))
		}
	})

	t.Run("StateUpdateAvailable marshals to \"update_available\"", func(t *testing.T) {
		b, err := json.Marshal(StateUpdateAvailable)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(b) != `"update_available"` {
			t.Errorf("expected %q, got %q", `"update_available"`, string(b))
		}
	})
}
