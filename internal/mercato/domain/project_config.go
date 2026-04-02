package domain

// ProjectConfig represents the per-project .mct.yml override file.
type ProjectConfig struct {
	Tools map[string]bool `yaml:"tools"`
}

// MergeTools returns project tools if non-nil, otherwise global tools.
// When project is non-nil it is used as-is (no key-by-key merge).
func MergeTools(global, project map[string]bool) map[string]bool {
	if project != nil {
		return project
	}
	return global
}
