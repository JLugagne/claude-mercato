package domain

import "testing"

func TestMergeTools_ProjectOverridesGlobal(t *testing.T) {
	global := map[string]bool{"claude": true, "cursor": true}
	project := map[string]bool{"claude": false}

	got := MergeTools(global, project)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got["claude"] != false {
		t.Errorf("claude = %v, want false", got["claude"])
	}
}

func TestMergeTools_NoOverride(t *testing.T) {
	global := map[string]bool{"claude": true, "cursor": true}

	got := MergeTools(global, nil)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if !got["claude"] {
		t.Error("claude = false, want true")
	}
	if !got["cursor"] {
		t.Error("cursor = false, want true")
	}
}

func TestMergeTools_BothNil(t *testing.T) {
	got := MergeTools(nil, nil)

	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestMergeTools_PartialProject(t *testing.T) {
	global := map[string]bool{"claude": true, "cursor": true, "copilot": false}
	project := map[string]bool{"claude": true}

	got := MergeTools(global, project)

	// Project is used as-is, NOT merged key by key.
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (project used as-is)", len(got))
	}
	if !got["claude"] {
		t.Error("claude = false, want true")
	}
	if _, exists := got["cursor"]; exists {
		t.Error("cursor should not exist in result (project used as-is)")
	}
}
