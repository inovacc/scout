package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/internal/engine/llm"
)

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
	var (
		md  string
		err error
	)

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
		job.Status = llm.JobStatusExtracting
		job.ExtractStarted = time.Now().UTC()
		_ = o.workspace.UpdateJob(job)
	}

	// Enrich system prompt with page intelligence
	systemPrompt := o.systemPrompt
	if intel := p.pageIntelligenceContext(); intel != "" {
		systemPrompt = intel + "\n\n" + systemPrompt
	}

	userPrompt := prompt + "\n\n---\n\n" + md

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	extractResult, err := o.provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		if job != nil {
			job.Status = llm.JobStatusFailed
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
			job.Status = llm.JobStatusCompleted
			_ = o.workspace.UpdateJob(job)
		}

		return result, nil
	}

	if job != nil {
		job.Status = llm.JobStatusReviewing
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
		reviewSystemPrompt = llm.DefaultReviewPrompt
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
			job.Status = llm.JobStatusFailed
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
		job.Status = llm.JobStatusCompleted
		_ = o.workspace.UpdateJob(job)
	}

	return result, nil
}
