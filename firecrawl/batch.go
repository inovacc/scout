package firecrawl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type batchParams struct {
	URLs    []string `json:"urls"`
	Formats []Format `json:"formats,omitempty"`
}

// BatchScrape starts an async batch scrape job for multiple URLs.
func (c *Client) BatchScrape(ctx context.Context, urls []string, formats ...Format) (*BatchJob, error) {
	params := &batchParams{URLs: urls, Formats: formats}

	data, err := c.post(ctx, "/batch/scrape", params)
	if err != nil {
		return nil, err
	}

	var job BatchJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal batch response: %w", err)
	}

	return &job, nil
}

// GetBatchStatus checks the status of a batch scrape job.
func (c *Client) GetBatchStatus(ctx context.Context, id string) (*BatchJob, error) {
	data, err := c.get(ctx, "/batch/scrape/"+id)
	if err != nil {
		return nil, err
	}

	var job BatchJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal batch status: %w", err)
	}

	return &job, nil
}

// WaitForBatch polls a batch scrape job until it completes.
func (c *Client) WaitForBatch(ctx context.Context, id string, interval time.Duration) (*BatchJob, error) {
	return poll[BatchJob](ctx, c, "/batch/scrape/"+id, interval)
}
