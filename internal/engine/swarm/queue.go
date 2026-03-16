package swarm

import (
	"fmt"
	"net/url"
	"sort"
	"sync"
	"time"
)

// DomainQueue is a thread-safe, domain-partitioned priority queue.
// URLs are grouped by domain and ordered depth-first within each domain.
// Domains are served round-robin to distribute load.
type DomainQueue struct {
	mu         sync.Mutex
	partitions map[string][]*CrawlRequest // domain -> requests sorted by depth desc
	domains    []string                    // ordered domain list for round-robin
	robin      int                         // current round-robin index
	rateLimits map[string]time.Duration    // per-domain rate limit
	lastAccess map[string]time.Time        // per-domain last dequeue time
	defaultRL  time.Duration               // default rate limit
}

// NewDomainQueue creates a new domain-partitioned queue.
func NewDomainQueue(defaultRateLimit time.Duration) *DomainQueue {
	return &DomainQueue{
		partitions: make(map[string][]*CrawlRequest),
		domains:    nil,
		rateLimits: make(map[string]time.Duration),
		lastAccess: make(map[string]time.Time),
		defaultRL:  defaultRateLimit,
	}
}

// Enqueue adds crawl requests to the queue, partitioned by domain.
// Requests are inserted in depth-descending order (highest depth first)
// so that depth-first traversal within a domain is achieved by popping
// from the end.
func (q *DomainQueue) Enqueue(requests []*CrawlRequest) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, req := range requests {
		if req == nil {
			continue
		}
		domain := req.Domain
		if domain == "" {
			d, err := extractDomain(req.URL)
			if err != nil {
				return fmt.Errorf("scout: swarm: enqueue: %w", err)
			}
			domain = d
			req.Domain = domain
		}
		q.partitions[domain] = append(q.partitions[domain], req)
	}

	// Re-sort each affected partition: highest depth first so pop from tail = depth-first.
	for domain := range q.partitions {
		sort.Slice(q.partitions[domain], func(i, j int) bool {
			return q.partitions[domain][i].Depth > q.partitions[domain][j].Depth
		})
	}

	q.rebuildDomains()
	return nil
}

// Dequeue removes up to n requests from the queue using round-robin across domains.
// It respects per-domain rate limits, skipping domains that were accessed too recently.
func (q *DomainQueue) Dequeue(n int) []*CrawlRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.domains) == 0 || n <= 0 {
		return nil
	}

	now := time.Now()
	var result []*CrawlRequest
	tried := 0

	for len(result) < n && tried < len(q.domains) {
		if q.robin >= len(q.domains) {
			q.robin = 0
		}
		domain := q.domains[q.robin]

		// Check rate limit.
		rl := q.rateLimit(domain)
		if last, ok := q.lastAccess[domain]; ok && now.Sub(last) < rl {
			q.robin++
			tried++
			continue
		}

		partition := q.partitions[domain]
		if len(partition) == 0 {
			q.robin++
			tried++
			continue
		}

		// Pop from tail (depth-first: highest depth items are at the front,
		// but we want to process the deepest available, so pop from front).
		// Actually we sorted descending, so index 0 is deepest. Pop from front.
		req := partition[0]
		q.partitions[domain] = partition[1:]
		q.lastAccess[domain] = now
		result = append(result, req)

		tried = 0 // reset tried counter on success
		q.robin++
	}

	q.rebuildDomains()
	return result
}

// Len returns the total number of queued requests.
func (q *DomainQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	total := 0
	for _, p := range q.partitions {
		total += len(p)
	}
	return total
}

// Domains returns the current set of domains with queued requests.
func (q *DomainQueue) Domains() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]string, len(q.domains))
	copy(out, q.domains)
	return out
}

// SetRateLimit sets a per-domain rate limit.
func (q *DomainQueue) SetRateLimit(domain string, d time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rateLimits[domain] = d
}

func (q *DomainQueue) rateLimit(domain string) time.Duration {
	if rl, ok := q.rateLimits[domain]; ok {
		return rl
	}
	return q.defaultRL
}

// rebuildDomains rebuilds the ordered domain list, removing empty partitions.
// Must be called with q.mu held.
func (q *DomainQueue) rebuildDomains() {
	q.domains = q.domains[:0]
	for domain, partition := range q.partitions {
		if len(partition) > 0 {
			q.domains = append(q.domains, domain)
		} else {
			delete(q.partitions, domain)
		}
	}
	sort.Strings(q.domains)
	if q.robin >= len(q.domains) {
		q.robin = 0
	}
}

func extractDomain(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url %q: %w", rawURL, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host in url %q", rawURL)
	}
	return u.Hostname(), nil
}
