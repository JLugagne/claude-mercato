package service

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type ReadmeQueries interface {
	// Readme returns the README.md at the given path within a market.
	// path is relative to market root, e.g. "README.md", "skills/README.md", "agents/README.md".
	Readme(market, path string) (domain.ReadmeEntry, error)

	// ListReadmes returns all README.md files found in a market.
	ListReadmes(market string) ([]domain.ReadmeEntry, error)
}
