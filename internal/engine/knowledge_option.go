package engine

import "time"

// KnowledgeOption configures a Knowledge operation.
type KnowledgeOption func(*knowledgeOptions)

type knowledgeOptions struct {
	maxDepth    int
	maxPages    int
	concurrency int
	timeout     time.Duration
	outputDir   string
}

func knowledgeDefaults() *knowledgeOptions {
	return &knowledgeOptions{
		maxDepth:    3,
		maxPages:    100,
		concurrency: 1,
		timeout:     30 * time.Second,
	}
}

// WithKnowledgeDepth sets the BFS crawl depth. Default: 3.
func WithKnowledgeDepth(n int) KnowledgeOption {
	return func(o *knowledgeOptions) { o.maxDepth = n }
}

// WithKnowledgeMaxPages sets the maximum pages to visit. Default: 100.
func WithKnowledgeMaxPages(n int) KnowledgeOption {
	return func(o *knowledgeOptions) { o.maxPages = n }
}

// WithKnowledgeConcurrency sets concurrent page processing. Default: 1.
func WithKnowledgeConcurrency(n int) KnowledgeOption {
	if n < 1 {
		n = 1
	}

	return func(o *knowledgeOptions) { o.concurrency = n }
}

// WithKnowledgeTimeout sets per-page timeout. Default: 30s.
func WithKnowledgeTimeout(d time.Duration) KnowledgeOption {
	return func(o *knowledgeOptions) { o.timeout = d }
}

// WithKnowledgeOutput sets the output directory for streaming pages to disk.
func WithKnowledgeOutput(dir string) KnowledgeOption {
	return func(o *knowledgeOptions) { o.outputDir = dir }
}
