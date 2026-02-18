package scout

import (
	"context"
	"fmt"
	"time"
)

const defaultReviewPrompt = `Review the following AI-generated extraction for accuracy, completeness, and correctness.
Check for:
1. Factual errors or hallucinations not supported by the source content
2. Missing important information from the original page
3. Formatting or structural issues
4. Any misinterpretations of the source material

Provide your review with corrections if needed. If the extraction is accurate, confirm it and note any minor improvements.`

// LLMJobResult holds the results of an extract+review pipeline.
type LLMJobResult struct {
	JobID         string `json:"job_id,omitempty"`
	ExtractResult string `json:"extract_result"`
	ReviewResult  string `json:"review_result,omitempty"`
	Reviewed      bool   `json:"reviewed"`
}

// WithLLMReview sets a review provider that validates the extraction output.
func WithLLMReview(provider LLMProvider) LLMOption {
	return func(o *llmOptions) { o.reviewProvider = provider }
}

// WithLLMReviewModel overrides the review provider's default model.
func WithLLMReviewModel(model string) LLMOption {
	return func(o *llmOptions) { o.reviewModel = model }
}

// WithLLMReviewPrompt overrides the default review system prompt.
func WithLLMReviewPrompt(prompt string) LLMOption {
	return func(o *llmOptions) { o.reviewPrompt = prompt }
}

// WithLLMWorkspace sets a workspace for persisting jobs to disk.
func WithLLMWorkspace(ws *LLMWorkspace) LLMOption {
	return func(o *llmOptions) { o.workspace = ws }
}

// WithLLMSessionID sets the session ID for job tracking.
func WithLLMSessionID(id string) LLMOption {
	return func(o *llmOptions) { o.sessionID = id }
}

// WithLLMMetadata adds a key-value metadata pair to the job.
func WithLLMMetadata(key, value string) LLMOption {
	return func(o *llmOptions) {
		if o.metadata == nil {
			o.metadata = make(map[string]string)
		}
		o.metadata[key] = value
	}
}

// ExtractWithLLMReview extracts page content with an LLM, then optionally reviews
// the output with a second LLM provider. Results are persisted to the workspace if set.
func (p *Page) ExtractWithLLMReview(prompt string, opts ...LLMOption) (*LLMJobResult, error) {
	o := defaultLLMOptions()
	for _, fn := range opts {
		fn(o)
	}

	if o.provider == nil {
		return nil, fmt.Errorf("scout: extract-llm-review: no LLM provider set (use WithLLMProvider)")
	}

	// Get page markdown
	var md string
	var err error
	if o.mainOnly {
		md, err = p.MarkdownContent()
	} else {
		md, err = p.Markdown()
	}
	if err != nil {
		return nil, fmt.Errorf("scout: extract-llm-review: get markdown: %w", err)
	}

	// Resolve page URL for job tracking
	pageURL := p.page.MustInfo().URL

	// Create job if workspace is set
	var job *LLMJob
	if o.workspace != nil {
		sessionID := o.sessionID
		if sessionID == "" {
			sess, err := o.workspace.CurrentSession()
			if err != nil {
				// Auto-create default session
				sess, err = o.workspace.CreateSession("default", nil)
				if err != nil {
					return nil, fmt.Errorf("scout: extract-llm-review: create default session: %w", err)
				}
			}
			sessionID = sess.ID
		}

		job, err = o.workspace.CreateJob(sessionID, pageURL, prompt, o.metadata)
		if err != nil {
			return nil, fmt.Errorf("scout: extract-llm-review: create job: %w", err)
		}

		job.ExtractProvider = o.provider.Name()
		job.ExtractModel = o.model
		if job.ExtractModel == "" {
			job.ExtractModel = "(default)"
		}
	}

	// --- Extract phase ---
	if job != nil {
		job.Status = JobStatusExtracting
		job.ExtractStarted = time.Now().UTC()
		_ = o.workspace.UpdateJob(job)
	}

	userPrompt := prompt + "\n\n---\n\n" + md

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	extractResult, err := o.provider.Complete(ctx, o.systemPrompt, userPrompt)
	if err != nil {
		if job != nil {
			job.Status = JobStatusFailed
			job.Error = err.Error()
			_ = o.workspace.UpdateJob(job)
		}
		return nil, fmt.Errorf("scout: extract-llm-review: extract: %s: %w", o.provider.Name(), err)
	}

	if job != nil {
		job.ExtractResult = extractResult
		job.ExtractFinished = time.Now().UTC()
		_ = o.workspace.UpdateJob(job)
	}

	result := &LLMJobResult{
		ExtractResult: extractResult,
	}
	if job != nil {
		result.JobID = job.ID
	}

	// --- Review phase (only if review provider is set) ---
	if o.reviewProvider == nil {
		if job != nil {
			job.Status = JobStatusCompleted
			_ = o.workspace.UpdateJob(job)
		}
		return result, nil
	}

	if job != nil {
		job.Status = JobStatusReviewing
		job.ReviewProvider = o.reviewProvider.Name()
		job.ReviewModel = o.reviewModel
		if job.ReviewModel == "" {
			job.ReviewModel = "(default)"
		}
		job.ReviewStarted = time.Now().UTC()
		_ = o.workspace.UpdateJob(job)
	}

	reviewSystemPrompt := o.reviewPrompt
	if reviewSystemPrompt == "" {
		reviewSystemPrompt = defaultReviewPrompt
	}

	// Build review user prompt: original prompt + source content + extraction result
	reviewUserPrompt := fmt.Sprintf(
		"Original extraction prompt: %s\n\n"+
			"--- Source page content ---\n\n%s\n\n"+
			"--- AI extraction result ---\n\n%s",
		prompt, md, extractResult,
	)

	reviewCtx, reviewCancel := context.WithTimeout(context.Background(), o.timeout)
	defer reviewCancel()

	reviewResult, err := o.reviewProvider.Complete(reviewCtx, reviewSystemPrompt, reviewUserPrompt)
	if err != nil {
		if job != nil {
			job.Status = JobStatusFailed
			job.Error = fmt.Sprintf("review failed: %v", err)
			_ = o.workspace.UpdateJob(job)
		}
		return nil, fmt.Errorf("scout: extract-llm-review: review: %s: %w", o.reviewProvider.Name(), err)
	}

	result.ReviewResult = reviewResult
	result.Reviewed = true

	if job != nil {
		job.ReviewResult = reviewResult
		job.ReviewPrompt = reviewSystemPrompt
		job.ReviewFinished = time.Now().UTC()
		job.Status = JobStatusCompleted
		_ = o.workspace.UpdateJob(job)
	}

	return result, nil
}
