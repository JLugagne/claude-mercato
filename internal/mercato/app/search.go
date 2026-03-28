package app

import (
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/JLugagne/bm25"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

func (a *App) Search(query string, opts service.SearchOpts) ([]service.SearchResult, error) {
	entries, err := a.buildCorpus()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	docs := make([]string, len(entries))
	for i, e := range entries {
		docs[i] = buildDoc(e)
	}

	vocab := buildVocab(docs)

	index, err := bm25.NewBM25Okapi(docs, tokenize, 1.5, 0.75, nil)
	if err != nil {
		return nil, err
	}

	queryTokens := tokenize(strings.ToLower(query))
	queryTokens = expandFuzzy(queryTokens, vocab)

	scores, err := index.GetScores(queryTokens)
	if err != nil {
		return nil, err
	}

	type hit struct {
		index int
		score float64
	}
	var hits []hit
	for i, s := range scores {
		if s > 0 {
			hits = append(hits, hit{i, s})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })

	limit := opts.Limit
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}

	var results []service.SearchResult
	for _, h := range hits {
		e := entries[h.index]

		if !opts.IncludeDeleted && e.Deleted {
			continue
		}
		if opts.Type != "" && e.Type != opts.Type {
			continue
		}
		if opts.Market != "" && e.Market != opts.Market {
			continue
		}
		if opts.Category != "" && !strings.HasPrefix(e.Category, opts.Category) {
			continue
		}
		if opts.Installed && !e.Installed {
			continue
		}
		if opts.NotInstalled && e.Installed {
			continue
		}

		results = append(results, service.SearchResult{
			Entry: e,
			Score: h.score,
		})
	}

	return results, nil
}

func (a *App) DumpIndex() ([]domain.Entry, error) {
	return a.buildCorpus()
}

func (a *App) BenchIndex() (service.BenchResult, error) {
	t0 := time.Now()

	entries, err := a.buildCorpus()
	if err != nil {
		return service.BenchResult{}, err
	}
	scanDone := time.Now()

	docs := make([]string, len(entries))
	for i, e := range entries {
		docs[i] = buildDoc(e)
	}

	vocab := buildVocab(docs)

	if len(docs) == 0 {
		return service.BenchResult{
			Scan:  scanDone.Sub(t0),
			Total: time.Since(t0),
		}, nil
	}

	_, err = bm25.NewBM25Okapi(docs, tokenize, 1.5, 0.75, nil)
	indexDone := time.Now()
	if err != nil {
		return service.BenchResult{}, err
	}

	return service.BenchResult{
		Scan:    scanDone.Sub(t0),
		Index:   indexDone.Sub(scanDone),
		Total:   indexDone.Sub(t0),
		Entries: len(entries),
		Vocab:   len(vocab),
	}, nil
}

func (a *App) buildCorpus() ([]domain.Entry, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}
	checksums, err := a.state.LoadChecksums(a.cacheDir)
	if err != nil {
		return nil, err
	}

	var entries []domain.Entry
	for _, mc := range cfg.Markets {
		clonePath := a.clonePath(mc.Name)
		mfiles, err := a.git.ReadMarketFiles(clonePath, mc.Branch)
		if err != nil {
			continue
		}

		// Build readme index from the batch results.
		readmeByProfile := make(map[string]readmeInfo)
		for _, mf := range mfiles {
			if !isReadme(mf.Path) {
				continue
			}
			ri := readmeInfo{content: string(mf.Content)}
			if rfm, err := domain.ParseReadmeFrontmatter(mf.Content); err == nil {
				ri.tags = rfm.MctTags
				ri.description = rfm.Description
			}
			readmeByProfile[profilePrefix(mf.Path)] = ri
		}

		for _, mf := range mfiles {
			if isReadme(mf.Path) {
				continue
			}
			fm, err := domain.ParseFrontmatter(mf.Content)
			if err != nil {
				continue
			}
			ref := domain.MctRef(mc.Name + "/" + mf.Path)
			_, installed := checksums.Entries[ref]

			entry := domain.Entry{
				Ref:            ref,
				Market:         mc.Name,
				RelPath:        mf.Path,
				Filename:       filepath.Base(mf.Path),
				Category:       inferCategory(mf.Path),
				Type:           fm.Type,
				Description:    fm.Description,
				Author:         fm.Author,
				Version:        mf.Version,
				Installed:      installed,
				BreakingChange: fm.BreakingChange,
				Deprecated:     fm.Deprecated,
				RequiresSkills: fm.RequiresSkills,
			}

			if ri, ok := readmeByProfile[profilePrefix(mf.Path)]; ok {
				entry.ReadmeContext = ri.content
				entry.MctTags = ri.tags
				entry.ProfileDescription = ri.description
			}

			entries = append(entries, entry)
		}
	}
	return entries, nil
}

type readmeInfo struct {
	content     string
	tags        []string
	description string
}

// profilePrefix returns the first two path segments (the profile root).
// e.g. "dev/go/skills/foo.md" -> "dev/go", "dev/go/README.md" -> "dev/go"
func profilePrefix(path string) string {
	parts := strings.SplitN(filepath.ToSlash(path), "/", 3)
	if len(parts) < 2 {
		return path
	}
	return parts[0] + "/" + parts[1]
}

func buildDoc(entry domain.Entry) string {
	slug := strings.ReplaceAll(strings.TrimSuffix(entry.Filename, ".md"), "-", " ")
	cat := strings.ReplaceAll(entry.Category, "/", " ")
	tags := strings.Join(entry.MctTags, " ")

	parts := []string{
		slug, slug, slug,
		tags, tags,
		entry.Description,
		cat,
		string(entry.Type),
	}
	if entry.ReadmeContext != "" {
		parts = append(parts, entry.ReadmeContext)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func inferCategory(relPath string) string {
	return profilePrefix(relPath)
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			tokens = append(tokens, w)
		}
	}
	if len(tokens) == 0 {
		tokens = append(tokens, lower)
	}
	return tokens
}

func buildVocab(docs []string) []string {
	seen := make(map[string]struct{})
	for _, doc := range docs {
		for _, tok := range tokenize(doc) {
			seen[tok] = struct{}{}
		}
	}
	vocab := make([]string, 0, len(seen))
	for term := range seen {
		vocab = append(vocab, term)
	}
	return vocab
}

// expandFuzzy adds fuzzy matches for query tokens that have no exact match in
// the vocabulary. For each unknown token, it finds the closest term within
// edit distance 2 and appends it to the query.
func expandFuzzy(queryTokens []string, vocab []string) []string {
	vocabSet := make(map[string]struct{}, len(vocab))
	for _, v := range vocab {
		vocabSet[v] = struct{}{}
	}

	expanded := make([]string, 0, len(queryTokens)*2)
	for _, qt := range queryTokens {
		expanded = append(expanded, qt)
		if _, exists := vocabSet[qt]; exists {
			continue // exact match exists
		}
		bestTerm := ""
		bestDist := 3
		for _, v := range vocab {
			d := levenshtein(qt, v)
			if d < bestDist {
				bestDist = d
				bestTerm = v
			}
		}
		if bestTerm != "" {
			expanded = append(expanded, bestTerm)
		}
	}
	return expanded
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	// Early exit: if length difference > 2, skip expensive computation
	if la-lb > 2 || lb-la > 2 {
		return 3
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
