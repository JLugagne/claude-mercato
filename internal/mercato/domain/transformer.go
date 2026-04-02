package domain

// OutputStrategy determines how a tool organizes its configuration files.
type OutputStrategy string

const (
	// OutputStrategyDirectory tools use nested directory structures (e.g. Claude, Codex).
	OutputStrategyDirectory OutputStrategy = "directory"
	// OutputStrategyFlat tools use a single flat file (e.g. Cursor, Windsurf, Gemini, OpenCode).
	OutputStrategyFlat OutputStrategy = "flat"
)

// ToolTarget describes a supported AI coding tool and how it consumes configuration.
type ToolTarget struct {
	Name           string
	Enabled        bool
	DetectDir      string
	OutputStrategy OutputStrategy
	SupportsAgents bool
	SupportsSkills bool
	FileExtension  string
}

// ToolMapping holds model and tool name mappings used during transformation.
// Models maps source model names to per-tool equivalents (e.g. "claude-sonnet-4-20250514" -> {"cursor": "claude-sonnet-4-20250514"}).
// Tools maps source tool names to per-tool equivalents (e.g. "Bash" -> {"cursor": "terminal"}).
type ToolMapping struct {
	Models map[string]map[string]string
	Tools  map[string]map[string]string
}

// TransformResult holds the output of a single transformer invocation.
type TransformResult struct {
	ToolName   string
	Content    []byte
	OutputPath string
	Warnings   []string
	Skipped    bool
	SkipReason string
}

// Transformer is a port interface for converting mct entries into tool-specific formats.
type Transformer interface {
	// Transform converts an entry's content into the target tool's format.
	Transform(entry Entry, content []byte, mappings ToolMapping) TransformResult

	// ToolName returns the name of the target tool (e.g. "cursor", "windsurf").
	ToolName() string

	// SupportsEntry reports whether this transformer can handle the given entry type.
	SupportsEntry(entryType EntryType) bool

	// OutputPath returns the destination path for the transformed entry.
	OutputPath(entry Entry) string
}

// TransformerRegistry is a named collection of transformers keyed by tool name.
type TransformerRegistry map[string]Transformer

// Get returns the transformer for the given tool name and whether it exists.
func (r TransformerRegistry) Get(name string) (Transformer, bool) {
	t, ok := r[name]
	return t, ok
}

// EnabledTransformers returns the transformers whose tool names are enabled
// in the provided tools map. A tool is considered enabled if its key exists
// in the map and its value is true.
func (r TransformerRegistry) EnabledTransformers(tools map[string]bool) []Transformer {
	var result []Transformer
	for name, t := range r {
		if tools[name] {
			result = append(result, t)
		}
	}
	return result
}
