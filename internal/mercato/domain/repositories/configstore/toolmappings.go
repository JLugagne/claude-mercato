package configstore

import "github.com/JLugagne/agents-mercato/internal/mercato/domain"

type ToolMappingStore interface {
	LoadToolMappings(path string) (domain.ToolMapping, error)
	SaveToolMappings(path string, m domain.ToolMapping) error
	ToolMappingsExist(path string) bool
	DefaultToolMappings() domain.ToolMapping
}
