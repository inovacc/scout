package scout

import (
	"fmt"
	"sync"
)

// BatchResult holds the result of processing a single URL.
type BatchResult struct {
	URL   string
	Error error
	Data  any // user-defined result from handler
}

// BatchOption configures batch scraping.
type BatchOption func(*batchOptions)

type batchOptions struct {
	concurrency int
	rateLimiter *RateLimiter
	onProgress  func(done, total int)
}

func batchDefaults() *batchOptions {
	return &batchOptions{
		concurrency: 3,
	}
}

// WithBatchConcurrency sets parallel page count. Default: 3.
func WithBatchConcurrency(n int) BatchOption {
	return func(o *batchOptions) {
		if n < 1 {
			n = 1
		}

		o.concurrency = n
	}
}

// WithBatchRateLimit applies a rate limiter to batch requests.
func WithBatchRateLimit(rl *RateLimiter) BatchOption {
	return func(o *batchOptions) { o.rateLimiter = rl }
}

// WithBatchProgress sets a progress callback.
func WithBatchProgress(fn func(done, total int)) BatchOption {
	return func(o *batchOptions) { o.onProgress = fn }
}

// BatchHandler processes a single URL. Return data and error.
type BatchHandler func(page *Page, url string) (any, error)

// BatchScrape processes multiple URLs concurrently with error isolation.
// Results are returned in the same order as input URLs.
func (b *Browser) BatchScrape(urls []string, handler BatchHandler, opts ...BatchOption) []BatchResult {
	o := batchDefaults()
	for _, fn := range opts {
		fn(o)
	}

	results := make([]BatchResult, len(urls))
	sem := make(chan struct{}, o.concurrency)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
	)

	for i, u := range urls {
		wg.Add(1)

		go func(idx int, rawURL string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			result := BatchResult{URL: rawURL}

			if o.rateLimiter != nil {
				o.rateLimiter.Wait()
			}

			page, err := b.NewPage(rawURL)
			if err != nil {
				result.Error = fmt.Errorf("scout: batch: %w", err)
				results[idx] = result
				mu.Lock()
				done++
				if o.onProgress != nil {
					o.onProgress(done, len(urls))
				}
				mu.Unlock()

				return
			}

			defer func() { _ = page.Close() }()

			if err := page.WaitLoad(); err != nil {
				result.Error = fmt.Errorf("scout: batch: %w", err)
				results[idx] = result
				mu.Lock()
				done++
				if o.onProgress != nil {
					o.onProgress(done, len(urls))
				}
				mu.Unlock()

				return
			}

			data, err := handler(page, rawURL)
			if err != nil {
				result.Error = fmt.Errorf("scout: batch: %w", err)
			}

			result.Data = data
			results[idx] = result

			mu.Lock()
			done++
			if o.onProgress != nil {
				o.onProgress(done, len(urls))
			}
			mu.Unlock()
		}(i, u)
	}

	wg.Wait()

	return results
}
