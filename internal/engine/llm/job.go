package llm

import "time"

// JobStatus represents the state of an LLM extraction job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusExtracting JobStatus = "extracting"
	JobStatusReviewing  JobStatus = "reviewing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// JobResult holds the results of an extract+review pipeline.
type JobResult struct {
	JobID         string `json:"job_id,omitempty"`
	ExtractResult string `json:"extract_result"`
	ReviewResult  string `json:"review_result,omitempty"`
	Reviewed      bool   `json:"reviewed"`
}

// Job tracks a single extraction+review job with full metadata.
type Job struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Status    JobStatus `json:"status"`

	// Input
	URL    string `json:"url"`
	Prompt string `json:"prompt"`

	// Extract phase
	ExtractProvider string `json:"extract_provider"`
	ExtractModel    string `json:"extract_model"`
	ExtractResult   string `json:"extract_result,omitempty"`

	// Review phase
	ReviewProvider string `json:"review_provider,omitempty"`
	ReviewModel    string `json:"review_model,omitempty"`
	ReviewPrompt   string `json:"review_prompt,omitempty"`
	ReviewResult   string `json:"review_result,omitempty"`

	// Timing
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	ExtractStarted  time.Time `json:"extract_started,omitempty"`  //nolint:modernize // omitzero may break JSON compat
	ExtractFinished time.Time `json:"extract_finished,omitempty"` //nolint:modernize // omitzero may break JSON compat
	ReviewStarted   time.Time `json:"review_started,omitempty"`   //nolint:modernize // omitzero may break JSON compat
	ReviewFinished  time.Time `json:"review_finished,omitempty"`  //nolint:modernize // omitzero may break JSON compat

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// JobRef is a lightweight reference stored in the jobs index.
type JobRef struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Status    JobStatus `json:"status"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// JobIndex is jobs/jobs.json — the index of all jobs and the current active job.
type JobIndex struct {
	Jobs    []JobRef `json:"jobs"`
	Current string   `json:"current"`
}

// DefaultReviewPrompt is the default system prompt for the review phase.
const DefaultReviewPrompt = `Review the following AI-generated extraction for accuracy, completeness, and correctness.
Check for:
1. Factual errors or hallucinations not supported by the source content
2. Missing important information from the original page
3. Formatting or structural issues
4. Any misinterpretations of the source material

Provide your review with corrections if needed. If the extraction is accurate, confirm it and note any minor improvements.`
