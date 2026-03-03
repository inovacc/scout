package engine

import (
	"fmt"
	"time"
)

// SolveFunc is a function that attempts to solve a specific challenge type on a page.
type SolveFunc func(page *Page, challenge ChallengeInfo) error

// SolverOption configures a ChallengeSolver.
type SolverOption func(*solverOptions)

type solverOptions struct {
	timeout   time.Duration
	providers []LLMProvider
	services  []CaptchaSolverService
}

func defaultSolverOptions() *solverOptions {
	return &solverOptions{
		timeout: 30 * time.Second,
	}
}

// WithSolverTimeout sets the maximum time for solving a single challenge.
func WithSolverTimeout(d time.Duration) SolverOption {
	return func(o *solverOptions) { o.timeout = d }
}

// WithSolverLLM adds an LLM provider for vision-based CAPTCHA solving.
func WithSolverLLM(provider LLMProvider) SolverOption {
	return func(o *solverOptions) { o.providers = append(o.providers, provider) }
}

// WithSolverService adds a third-party CAPTCHA solving service.
func WithSolverService(svc CaptchaSolverService) SolverOption {
	return func(o *solverOptions) { o.services = append(o.services, svc) }
}

// ChallengeSolver detects and attempts to bypass bot protection challenges.
type ChallengeSolver struct {
	browser *Browser
	opts    *solverOptions
	solvers map[ChallengeType]SolveFunc
}

// NewChallengeSolver creates a new solver with built-in handlers for common challenge types.
func NewChallengeSolver(browser *Browser, opts ...SolverOption) *ChallengeSolver {
	o := defaultSolverOptions()
	for _, fn := range opts {
		fn(o)
	}

	cs := &ChallengeSolver{
		browser: browser,
		opts:    o,
		solvers: make(map[ChallengeType]SolveFunc),
	}

	// Register built-in solvers.
	cs.solvers[ChallengeCloudflare] = solveCloudflareWait
	cs.solvers[ChallengeTurnstile] = solveTurnstile
	cs.solvers[ChallengeRecaptchaV2] = solveRecaptchaV2
	cs.solvers[ChallengeHCaptcha] = solveHCaptcha

	return cs
}

// Register adds or replaces a solver for the given challenge type.
func (cs *ChallengeSolver) Register(ct ChallengeType, fn SolveFunc) {
	if cs == nil {
		return
	}

	cs.solvers[ct] = fn
}

// Solve detects the highest-confidence challenge on the page and applies the
// appropriate solver. Returns nil if no challenge is detected.
func (cs *ChallengeSolver) Solve(page *Page) error {
	if cs == nil || cs.browser == nil {
		return fmt.Errorf("scout: challenge: nil solver or browser")
	}

	if page == nil || page.page == nil {
		return fmt.Errorf("scout: challenge: nil page")
	}

	challenge, err := page.DetectChallenge()
	if err != nil {
		return fmt.Errorf("scout: challenge: detect: %w", err)
	}

	if challenge == nil {
		return nil
	}

	return cs.solveOne(page, *challenge)
}

// SolveAll detects and solves all challenges iteratively, up to 3 retries.
func (cs *ChallengeSolver) SolveAll(page *Page) error {
	if cs == nil || cs.browser == nil {
		return fmt.Errorf("scout: challenge: nil solver or browser")
	}

	if page == nil || page.page == nil {
		return fmt.Errorf("scout: challenge: nil page")
	}

	const maxRetries = 3
	for range maxRetries {
		challenges, err := page.DetectChallenges()
		if err != nil {
			return fmt.Errorf("scout: challenge: detect: %w", err)
		}

		if len(challenges) == 0 {
			return nil
		}

		for _, ch := range challenges {
			if solveErr := cs.solveOne(page, ch); solveErr != nil {
				// Log but continue to next challenge; final check below.
				_ = solveErr
			}
		}

		// Brief pause before re-checking.
		time.Sleep(500 * time.Millisecond)
	}

	// Final check: are there still challenges?
	remaining, err := page.DetectChallenges()
	if err != nil {
		return fmt.Errorf("scout: challenge: final detect: %w", err)
	}

	if len(remaining) > 0 {
		return fmt.Errorf("scout: challenge: %d challenge(s) remain after %d retries", len(remaining), maxRetries)
	}

	return nil
}

// solveOne applies the registered solver for a single challenge.
func (cs *ChallengeSolver) solveOne(page *Page, ch ChallengeInfo) error {
	fn, ok := cs.solvers[ch.Type]
	if !ok {
		return fmt.Errorf("scout: challenge: no solver for %s", ch.Type)
	}

	return fn(page, ch)
}
