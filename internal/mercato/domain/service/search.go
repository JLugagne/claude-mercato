package service

import (
	"time"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

type SearchQueries interface {
	Search(query string, opts SearchOpts) ([]SearchResult, error)
	DumpIndex() ([]domain.Entry, error)
	BenchIndex() (BenchResult, error)
}

type BenchResult struct {
	Scan  time.Duration // time to read markets and parse entries
	Index time.Duration // time to build the BM25 search index
	Total time.Duration
	Entries int
	Vocab   int
}

type SearchOpts struct {
	Type           domain.EntryType
	Market         string
	Category       string
	Installed      bool
	NotInstalled   bool
	IncludeDeleted bool
	Limit          int
	JSON           bool
}

type SearchResult struct {
	Entry domain.Entry `json:"entry"`
	Score float64      `json:"score"`
}
