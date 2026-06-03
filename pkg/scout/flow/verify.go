package flow

import (
	"context"
	"fmt"
)

// VerifyReport is the parity result of re-running a flow vs a golden capture.
type VerifyReport struct {
	OK    bool
	Steps []StepDiff
}

// StepDiff compares one replayed step to its golden entry.
type StepDiff struct {
	ID             string
	ExpectedStatus int
	ActualStatus   int
	StatusMatch    bool
}

// Verify re-runs f and diffs each step's status against golden.Entries by index.
// Expect assertions are ignored during verify (we want to observe drift, not abort).
func Verify(ctx context.Context, f *FlowSpec, golden *Capture, opts RunOptions) (*VerifyReport, error) {
	clone := *f
	clone.Steps = make([]FlowStep, len(f.Steps))
	for i, s := range f.Steps {
		s.Expect = nil
		clone.Steps[i] = s
	}
	res, err := Run(ctx, &clone, opts)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: verify: %w", err)
	}

	rep := &VerifyReport{OK: true, Steps: make([]StepDiff, 0, len(res.Steps))}
	for i, sr := range res.Steps {
		var exp int
		if i < len(golden.Entries) {
			exp = golden.Entries[i].Status
		}
		match := exp == 0 || exp == sr.Status
		if !match {
			rep.OK = false
		}
		rep.Steps = append(rep.Steps, StepDiff{ID: sr.ID, ExpectedStatus: exp, ActualStatus: sr.Status, StatusMatch: match})
	}
	return rep, nil
}
