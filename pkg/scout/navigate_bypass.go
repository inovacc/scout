package scout

import (
	"fmt"
	"time"
)

// NavigateWithBypass navigates to the URL and automatically solves any detected
// bot protection challenges. If solver is nil, falls back to normal navigation.
func NavigateWithBypass(page *Page, url string, solver *ChallengeSolver) error {
	if page == nil || page.page == nil {
		return fmt.Errorf("scout: navigate bypass: nil page")
	}

	if err := page.Navigate(url); err != nil {
		return err
	}

	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("scout: navigate bypass: wait load: %w", err)
	}

	if solver == nil {
		return nil
	}

	// Brief stabilization before detection.
	_ = page.WaitStable(500 * time.Millisecond)

	return solver.SolveAll(page)
}

// WithAutoBypass sets a ChallengeSolver that is automatically applied after
// every NewPage navigation. When set, NewPage will detect and attempt to solve
// bot protection challenges on the loaded page.
func WithAutoBypass(solver *ChallengeSolver) Option {
	return func(o *options) { o.autoBypass = solver }
}
