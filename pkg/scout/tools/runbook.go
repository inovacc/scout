package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/runbook"
)

// RunbookPlanInput dry-runs a runbook against a live page: resolves
// every selector, classifies each step as ok/warn/fail, and reports
// without executing any side-effects.
type RunbookPlanInput struct {
	File string `json:"file"          jsonschema:"absolute path to the runbook JSON/YAML file"`
}

// RunbookPlanOutput re-exports the engine ExecutionPlan.
type RunbookPlanOutput = runbook.ExecutionPlan

// RunbookPlan loads the runbook file and dry-runs it via runbook.Plan.
func RunbookPlan(_ context.Context, b *scout.Browser, in RunbookPlanInput) (*RunbookPlanOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: runbook-plan: nil browser")
	}
	if in.File == "" {
		return nil, fmt.Errorf("tools: runbook-plan: file required")
	}
	r, err := runbook.LoadFile(in.File)
	if err != nil {
		return nil, fmt.Errorf("tools: runbook-plan: load %s: %w", in.File, err)
	}
	return runbook.Plan(b, r)
}

// RunbookApplyInput executes a runbook against the live browser.
type RunbookApplyInput struct {
	File string `json:"file" jsonschema:"absolute path to the runbook JSON/YAML file"`
}

// RunbookApplyOutput re-exports the engine result.
type RunbookApplyOutput = runbook.Result

// RunbookApply loads the runbook file and executes it via runbook.Apply.
func RunbookApply(ctx context.Context, b *scout.Browser, in RunbookApplyInput) (*RunbookApplyOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: runbook-apply: nil browser")
	}
	if in.File == "" {
		return nil, fmt.Errorf("tools: runbook-apply: file required")
	}
	r, err := runbook.LoadFile(in.File)
	if err != nil {
		return nil, fmt.Errorf("tools: runbook-apply: load %s: %w", in.File, err)
	}
	return runbook.Apply(ctx, b, r)
}
