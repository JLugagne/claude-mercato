package transform

import (
	"testing"
)

func TestExtractBody(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "with frontmatter",
			content: "---\ndescription: test\n---\nHello world",
			want:    "Hello world",
		},
		{
			name:    "no frontmatter",
			content: "Just plain content",
			want:    "Just plain content",
		},
		{
			name:    "empty body after frontmatter",
			content: "---\nfoo: bar\n---\n",
			want:    "",
		},
		{
			name:    "multiline body",
			content: "---\nfoo: bar\n---\nLine 1\nLine 2\nLine 3",
			want:    "Line 1\nLine 2\nLine 3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(extractBody([]byte(tt.content)))
			if got != tt.want {
				t.Errorf("extractBody() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildFrontmatter(t *testing.T) {
	fields := [][2]string{
		{"alwaysApply", "false"},
		{"description", "my skill"},
	}
	got := string(buildFrontmatter(fields))
	want := "---\nalwaysApply: false\ndescription: my skill\n---\n"
	if got != want {
		t.Errorf("buildFrontmatter() = %q, want %q", got, want)
	}
}

func TestParseDescription(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "has description",
			content: "---\ndescription: My great skill\nauthor: test\n---\nBody",
			want:    "My great skill",
		},
		{
			name:    "no frontmatter",
			content: "Just plain text",
			want:    "",
		},
		{
			name:    "empty description",
			content: "---\nauthor: test\n---\nBody",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDescription([]byte(tt.content))
			if got != tt.want {
				t.Errorf("parseDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}
