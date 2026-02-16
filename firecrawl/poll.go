package firecrawl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// poll repeatedly checks an async job status until it completes or ctx is cancelled.
func poll[T any](ctx context.Context, c *Client, path string, interval time.Duration) (*T, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		data, err := c.get(ctx, path)
		if err != nil {
			return nil, err
		}

		var result T
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("firecrawl: unmarshal poll response: %w", err)
		}

		// Check if the job has a status field indicating completion.
		var status struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(data, &status) == nil {
			switch status.Status {
			case "completed":
				return &result, nil
			case "failed":
				return nil, fmt.Errorf("firecrawl: job failed")
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
