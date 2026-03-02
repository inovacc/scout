package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ResearchSource represents a single source used in a research result.
type ResearchSource struct {
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Relevance float64 `json:"relevance"`
}

// ResearchResult holds the output of a research query.
type ResearchResult struct {
	Query             string           `json:"query"`
	Summary           string           `json:"summary"`
	Sources           []ResearchSource `json:"sources"`
	FollowUpQuestions []string         `json:"follow_up_questions,omitempty"`
	Duration          time.Duration    `json:"duration"`
	Depth             int              `json:"depth"`
}

// ResearchOption configures ResearchAgent behavior.
type ResearchOption func(*researchOpts)

type researchOpts struct {
	maxSources  int
	maxDepth    int
	fetchMode   string
	timeout     time.Duration
	concurrency int
	engine      SearchEngine
	mainOnly    bool
	cache       *ResearchCache
	prior       *ResearchResult
}

func researchDefaults() *researchOpts {
	return &researchOpts{
		maxSources:  5,
		maxDepth:    1,
		fetchMode:   "markdown",
		timeout:     2 * time.Minute,
		concurrency: 3,
		engine:      Google,
		mainOnly:    true,
	}
}

// WithResearchMaxSources sets the maximum number of sources to fetch. Default: 5.
func WithResearchMaxSources(n int) ResearchOption {
	return func(o *researchOpts) { o.maxSources = n }
}

// WithResearchDepth sets the maximum depth for deep research iterations. Default: 1.
func WithResearchDepth(d int) ResearchOption {
	return func(o *researchOpts) { o.maxDepth = d }
}

// WithResearchTimeout sets the overall research timeout. Default: 2m.
func WithResearchTimeout(d time.Duration) ResearchOption {
	return func(o *researchOpts) { o.timeout = d }
}

// WithResearchFetchMode sets the fetch mode for source pages. Default: "markdown".
func WithResearchFetchMode(mode string) ResearchOption {
	return func(o *researchOpts) { o.fetchMode = mode }
}

// WithResearchConcurrency sets fetch parallelism. Default: 3.
func WithResearchConcurrency(n int) ResearchOption {
	return func(o *researchOpts) { o.concurrency = n }
}

// WithResearchEngine sets the search engine. Default: Google.
func WithResearchEngine(e SearchEngine) ResearchOption {
	return func(o *researchOpts) { o.engine = e }
}

// WithResearchMainContent enables main content extraction. Default: true.
func WithResearchMainContent(b bool) ResearchOption {
	return func(o *researchOpts) { o.mainOnly = b }
}

// ResearchAgent orchestrates multi-source research using WebSearch + WebFetch + LLM.
type ResearchAgent struct {
	browser  *Browser
	provider LLMProvider
	opts     researchOpts
}

// NewResearchAgent creates a new research agent with the given browser, LLM provider, and options.
func NewResearchAgent(browser *Browser, provider LLMProvider, opts ...ResearchOption) *ResearchAgent {
	o := researchDefaults()
	for _, fn := range opts {
		fn(o)
	}

	return &ResearchAgent{
		browser:  browser,
		provider: provider,
		opts:     *o,
	}
}

// Research performs a single-depth research query: search, fetch, and summarize.
func (ra *ResearchAgent) Research(ctx context.Context, query string) (*ResearchResult, error) {
	if ra.browser == nil {
		return nil, fmt.Errorf("scout: research: browser is nil")
	}

	if ra.provider == nil {
		return nil, fmt.Errorf("scout: research: LLM provider is nil")
	}

	// Check cache first.
	if ra.opts.cache != nil {
		if cached, ok := ra.opts.cache.Get(query); ok {
			return cached, nil
		}
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, ra.opts.timeout)
	defer cancel()

	// Step 1: Search
	searchOpts := []WebSearchOption{
		WithWebSearchEngine(ra.opts.engine),
		WithWebSearchMaxFetch(ra.opts.maxSources),
		WithWebSearchFetch(ra.opts.fetchMode),
		WithWebSearchConcurrency(ra.opts.concurrency),
	}
	if ra.opts.mainOnly {
		searchOpts = append(searchOpts, WithWebSearchMainContent())
	}

	searchResult, err := ra.browser.WebSearch(query, searchOpts...) //nolint:contextcheck
	if err != nil {
		return nil, fmt.Errorf("scout: research: search: %w", err)
	}

	// Step 2: Build sources from search results
	sources := ra.buildSources(searchResult)

	// Step 3: Synthesize with LLM
	prompt := ra.buildPrompt(query, sources)
	systemPrompt := researchSystemPrompt()

	llmResp, err := ra.provider.Complete(ctx, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("scout: research: llm: %w", err)
	}

	// Step 4: Parse LLM response
	result := ra.parseResponse(query, llmResp, sources)
	result.Duration = time.Since(start)
	result.Depth = 0

	// Merge with prior results if provided.
	if ra.opts.prior != nil {
		result.Sources = deduplicateSources(append(ra.opts.prior.Sources, result.Sources...))
	}

	// Store in cache.
	if ra.opts.cache != nil {
		ra.opts.cache.Put(query, result)
	}

	return result, nil
}

// DeepResearch performs multi-depth research: initial research + follow-up iterations.
func (ra *ResearchAgent) DeepResearch(ctx context.Context, query string) (*ResearchResult, error) {
	if ra.browser == nil {
		return nil, fmt.Errorf("scout: research: browser is nil")
	}

	if ra.provider == nil {
		return nil, fmt.Errorf("scout: research: LLM provider is nil")
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, ra.opts.timeout)
	defer cancel()

	// Initial research
	initial, err := ra.Research(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("scout: deep-research: initial: %w", err)
	}

	allSources := make([]ResearchSource, len(initial.Sources))
	copy(allSources, initial.Sources)
	allSummaries := []string{initial.Summary}

	// Follow-up iterations
	followUps := initial.FollowUpQuestions
	for depth := 1; depth < ra.opts.maxDepth && len(followUps) > 0; depth++ {
		if ctx.Err() != nil {
			break
		}

		// Research each follow-up (limit to 2 per depth to stay within time budget)
		limit := min(2, len(followUps))

		var nextFollowUps []string

		for i := range limit {
			if ctx.Err() != nil {
				break
			}

			sub, subErr := ra.Research(ctx, followUps[i])
			if subErr != nil {
				continue
			}

			allSummaries = append(allSummaries, sub.Summary)
			allSources = append(allSources, sub.Sources...) //nolint:makezero
			nextFollowUps = append(nextFollowUps, sub.FollowUpQuestions...)
		}

		followUps = nextFollowUps
	}

	// Deduplicate sources by URL
	allSources = deduplicateSources(allSources)

	// Final synthesis
	finalPrompt := buildFinalSynthesisPrompt(query, allSummaries, allSources)
	finalSystem := "You are a research synthesizer. Combine the following research findings into one comprehensive, well-organized summary. " +
		"Cite sources by their URL. Suggest 3-5 follow-up questions for further investigation. " +
		"Respond in JSON format with fields: summary (string), follow_up_questions (array of strings)."

	finalResp, err := ra.provider.Complete(ctx, finalSystem, finalPrompt)
	if err != nil {
		// Fall back to concatenated summaries
		return &ResearchResult{
			Query:             query,
			Summary:           strings.Join(allSummaries, "\n\n"),
			Sources:           allSources,
			FollowUpQuestions: followUps,
			Duration:          time.Since(start),
			Depth:             ra.opts.maxDepth,
		}, nil
	}

	result := &ResearchResult{
		Query:   query,
		Sources: allSources,
		Depth:   ra.opts.maxDepth,
	}

	var parsed struct {
		Summary           string   `json:"summary"`
		FollowUpQuestions []string `json:"follow_up_questions"`
	}
	if err := json.Unmarshal([]byte(finalResp), &parsed); err == nil && parsed.Summary != "" {
		result.Summary = parsed.Summary
		result.FollowUpQuestions = parsed.FollowUpQuestions
	} else {
		result.Summary = finalResp
	}

	result.Duration = time.Since(start)

	return result, nil
}

// buildSources extracts ResearchSource entries from search results.
func (ra *ResearchAgent) buildSources(searchResult *WebSearchResult) []ResearchSource {
	var sources []ResearchSource

	for _, item := range searchResult.Results {
		content := item.Snippet
		if item.Content != nil && item.Content.Markdown != "" {
			content = item.Content.Markdown
		}

		if content == "" {
			continue
		}

		sources = append(sources, ResearchSource{
			URL:       item.URL,
			Title:     item.Title,
			Content:   content,
			Relevance: 1.0 / float64(item.Position),
		})
		if len(sources) >= ra.opts.maxSources {
			break
		}
	}

	return sources
}

// buildPrompt constructs the user prompt for the LLM with query and source content.
func (ra *ResearchAgent) buildPrompt(query string, sources []ResearchSource) string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "Research query: %s\n\n", query)
	_, _ = fmt.Fprintf(&b, "Sources (%d):\n\n", len(sources))

	for i, src := range sources {
		_, _ = fmt.Fprintf(&b, "--- Source %d: %s ---\n", i+1, src.URL)
		if src.Title != "" {
			_, _ = fmt.Fprintf(&b, "Title: %s\n", src.Title)
		}
		// Truncate very long content to stay within token limits
		content := src.Content
		if len(content) > 8000 {
			content = content[:8000] + "\n[... truncated]"
		}

		_, _ = fmt.Fprintf(&b, "%s\n\n", content)
	}

	return b.String()
}

// BuildPrompt is exported for testing. It constructs the user prompt for the LLM.
func (ra *ResearchAgent) BuildPrompt(query string, sources []ResearchSource) string {
	return ra.buildPrompt(query, sources)
}

func researchSystemPrompt() string {
	return `You are a research assistant. Analyze the provided sources to answer the research query comprehensively.

Respond in JSON format with these fields:
- "summary": A comprehensive, well-organized summary answering the query. Cite sources by number (e.g., [1], [2]).
- "source_relevance": An array of objects with "url" (string) and "relevance" (float 0-1) indicating how relevant each source was.
- "follow_up_questions": An array of 3-5 follow-up questions for further investigation.

Be thorough, accurate, and cite your sources. If sources contradict each other, note the disagreement.`
}

// parseResponse extracts structured data from the LLM JSON response.
func (ra *ResearchAgent) parseResponse(query, llmResp string, sources []ResearchSource) *ResearchResult {
	result := &ResearchResult{
		Query:   query,
		Sources: sources,
	}

	var parsed struct {
		Summary           string   `json:"summary"`
		FollowUpQuestions []string `json:"follow_up_questions"`
		SourceRelevance   []struct {
			URL       string  `json:"url"`
			Relevance float64 `json:"relevance"`
		} `json:"source_relevance"`
	}

	if err := json.Unmarshal([]byte(llmResp), &parsed); err == nil && parsed.Summary != "" {
		result.Summary = parsed.Summary
		result.FollowUpQuestions = parsed.FollowUpQuestions

		// Update source relevance from LLM assessment
		relMap := make(map[string]float64)
		for _, sr := range parsed.SourceRelevance {
			relMap[sr.URL] = sr.Relevance
		}

		for i := range result.Sources {
			if r, ok := relMap[result.Sources[i].URL]; ok {
				result.Sources[i].Relevance = r
			}
		}
	} else {
		// LLM didn't return valid JSON — use raw response as summary
		result.Summary = llmResp
	}

	return result
}

// deduplicateSources removes duplicate sources by URL, keeping the first occurrence.
func deduplicateSources(sources []ResearchSource) []ResearchSource {
	seen := make(map[string]struct{})

	var out []ResearchSource

	for _, s := range sources {
		if _, ok := seen[s.URL]; ok {
			continue
		}

		seen[s.URL] = struct{}{}
		out = append(out, s)
	}

	return out
}

func buildFinalSynthesisPrompt(query string, summaries []string, sources []ResearchSource) string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "Original query: %s\n\n", query)

	_, _ = fmt.Fprintf(&b, "Research findings from %d rounds:\n\n", len(summaries))
	for i, s := range summaries {
		_, _ = fmt.Fprintf(&b, "--- Round %d ---\n%s\n\n", i+1, s)
	}

	_, _ = fmt.Fprintf(&b, "All sources (%d):\n", len(sources))
	for i, src := range sources {
		_, _ = fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, src.Title, src.URL)
	}

	return b.String()
}

// fetchSourcesConcurrent fetches sources concurrently with a semaphore.
// This is kept as a utility but the main flow uses WebSearch with built-in fetch.
func (ra *ResearchAgent) fetchSourcesConcurrent(urls []string) []ResearchSource { //nolint:unused
	results := make([]ResearchSource, len(urls))

	var wg sync.WaitGroup

	sem := make(chan struct{}, ra.opts.concurrency)

	for i, u := range urls {
		wg.Add(1)

		go func(idx int, url string) {
			defer wg.Done()

			sem <- struct{}{}

			defer func() { <-sem }()

			var fetchOpts []WebFetchOption

			fetchOpts = append(fetchOpts, WithFetchMode(ra.opts.fetchMode))
			if ra.opts.mainOnly {
				fetchOpts = append(fetchOpts, WithFetchMainContent())
			}

			fr, err := ra.browser.WebFetch(url, fetchOpts...)
			if err != nil {
				return
			}

			results[idx] = ResearchSource{
				URL:     fr.URL,
				Title:   fr.Title,
				Content: fr.Markdown,
			}
		}(i, u)
	}

	wg.Wait()

	var out []ResearchSource

	for _, r := range results {
		if r.URL != "" {
			out = append(out, r)
		}
	}

	return out
}
