package configstore

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type ToolMappingStore interface {
	LoadToolMappings(path string) (domain.ToolMapping, error)
	SaveToolMappings(path string, m domain.ToolMapping) error
	ToolMappingsExist(path string) bool
	DefaultToolMappings() domain.ToolMapping
}
