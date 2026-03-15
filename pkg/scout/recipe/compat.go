// Deprecated: Package recipe is deprecated. Use package runbook instead.
// Will be removed after 2026-04-15.
//
// This package provides type aliases and wrapper functions that forward to the
// runbook package for backward compatibility.
package recipe

import (
	"context"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/runbook"
)

// Deprecated: Use runbook.Runbook instead. Will be removed after 2026-04-15.
type Recipe = runbook.Runbook

// Deprecated: Use runbook.ItemSpec instead. Will be removed after 2026-04-15.
type ItemSpec = runbook.ItemSpec

// Deprecated: Use runbook.Pagination instead. Will be removed after 2026-04-15.
type Pagination = runbook.Pagination

// Deprecated: Use runbook.Step instead. Will be removed after 2026-04-15.
type Step = runbook.Step

// Deprecated: Use runbook.Output instead. Will be removed after 2026-04-15.
type Output = runbook.Output

// Deprecated: Use runbook.Result instead. Will be removed after 2026-04-15.
type Result = runbook.Result

// Deprecated: Use runbook.SelectorScore instead. Will be removed after 2026-04-15.
type SelectorScore = runbook.SelectorScore

// Deprecated: Use runbook.ValidationResult instead. Will be removed after 2026-04-15.
type ValidationResult = runbook.ValidationResult

// Deprecated: Use runbook.ValidationError instead. Will be removed after 2026-04-15.
type ValidationError = runbook.ValidationError

// Deprecated: Use runbook.SiteAnalysis instead. Will be removed after 2026-04-15.
type SiteAnalysis = runbook.SiteAnalysis

// Deprecated: Use runbook.ContainerCandidate instead. Will be removed after 2026-04-15.
type ContainerCandidate = runbook.ContainerCandidate

// Deprecated: Use runbook.FieldCandidate instead. Will be removed after 2026-04-15.
type FieldCandidate = runbook.FieldCandidate

// Deprecated: Use runbook.FormCandidate instead. Will be removed after 2026-04-15.
type FormCandidate = runbook.FormCandidate

// Deprecated: Use runbook.FormFieldCandidate instead. Will be removed after 2026-04-15.
type FormFieldCandidate = runbook.FormFieldCandidate

// Deprecated: Use runbook.PaginationCandidate instead. Will be removed after 2026-04-15.
type PaginationCandidate = runbook.PaginationCandidate

// Deprecated: Use runbook.InteractableElement instead. Will be removed after 2026-04-15.
type InteractableElement = runbook.InteractableElement

// Deprecated: Use runbook.GenerateOption instead. Will be removed after 2026-04-15.
type GenerateOption = runbook.GenerateOption

// Deprecated: Use runbook.AnalyzeOption instead. Will be removed after 2026-04-15.
type AnalyzeOption = runbook.AnalyzeOption

// Deprecated: Use runbook.AIRunbookOption instead. Will be removed after 2026-04-15.
type AIRecipeOption = runbook.AIRunbookOption

// Deprecated: Use runbook.InteractiveConfig instead. Will be removed after 2026-04-15.
type InteractiveConfig = runbook.InteractiveConfig

// Deprecated: Use runbook.FlowStep instead. Will be removed after 2026-04-15.
type FlowStep = runbook.FlowStep

// Deprecated: Use runbook.FormInfo instead. Will be removed after 2026-04-15.
type FormInfo = runbook.FormInfo

// Deprecated: Use runbook.LLMValidation instead. Will be removed after 2026-04-15.
type LLMValidation = runbook.LLMValidation

// Deprecated: Use runbook.LoadFile instead. Will be removed after 2026-04-15.
func LoadFile(path string) (*Recipe, error) { return runbook.LoadFile(path) }

// Deprecated: Use runbook.Parse instead. Will be removed after 2026-04-15.
func Parse(data []byte) (*Recipe, error) { return runbook.Parse(data) }

// Deprecated: Use runbook.Apply instead. Will be removed after 2026-04-15.
func Run(ctx context.Context, browser *scout.Browser, r *Recipe) (*Result, error) {
	return runbook.Apply(ctx, browser, r)
}

// Deprecated: Use runbook.ValidateRunbook instead. Will be removed after 2026-04-15.
func ValidateRecipe(browser *scout.Browser, r *Recipe) (*ValidationResult, error) {
	return runbook.ValidateRunbook(browser, r)
}

// Deprecated: Use runbook.ScoreSelector instead. Will be removed after 2026-04-15.
func ScoreSelector(sel string) SelectorScore { return runbook.ScoreSelector(sel) }

// Deprecated: Use runbook.ScoreRunbookSelectors instead. Will be removed after 2026-04-15.
func ScoreRecipeSelectors(r *Recipe) map[string]SelectorScore {
	return runbook.ScoreRunbookSelectors(r)
}

// Deprecated: Use runbook.FixRunbook instead. Will be removed after 2026-04-15.
func FixRecipe(browser *scout.Browser, r *Recipe) (*Recipe, []string, error) {
	return runbook.FixRunbook(browser, r)
}

// Deprecated: Use runbook.SampleExtract instead. Will be removed after 2026-04-15.
func SampleExtract(browser *scout.Browser, r *Recipe) ([]map[string]any, error) {
	return runbook.SampleExtract(browser, r)
}

// Deprecated: Use runbook.GenerateRunbook instead. Will be removed after 2026-04-15.
func GenerateRecipe(analysis *SiteAnalysis, opts ...GenerateOption) (*Recipe, error) {
	return runbook.GenerateRunbook(analysis, opts...)
}

// Deprecated: Use runbook.GenerateFlowRunbook instead. Will be removed after 2026-04-15.
func GenerateFlowRecipe(steps []FlowStep, name string) (*Recipe, error) {
	return runbook.GenerateFlowRunbook(steps, name)
}

// Deprecated: Use runbook.GenerateWithAI instead. Will be removed after 2026-04-15.
func GenerateWithAI(browser *scout.Browser, url string, opts ...AIRecipeOption) (*Recipe, error) {
	return runbook.GenerateWithAI(browser, url, opts...)
}

// Deprecated: Use runbook.AnalyzeSite instead. Will be removed after 2026-04-15.
func AnalyzeSite(ctx context.Context, browser *scout.Browser, url string, opts ...AnalyzeOption) (*SiteAnalysis, error) {
	return runbook.AnalyzeSite(ctx, browser, url, opts...)
}

// Deprecated: Use runbook.InteractiveCreate instead. Will be removed after 2026-04-15.
func InteractiveCreate(cfg InteractiveConfig) (*Recipe, error) {
	return runbook.InteractiveCreate(cfg)
}

// Deprecated: Use runbook.DetectFlow instead. Will be removed after 2026-04-15.
func DetectFlow(browser *scout.Browser, urls []string) ([]FlowStep, error) {
	return runbook.DetectFlow(browser, urls)
}

// Deprecated: Use runbook.ValidateWithLLM instead. Will be removed after 2026-04-15.
func ValidateWithLLM(provider scout.LLMProvider, r *Recipe, sampleItems []map[string]any) (*LLMValidation, error) {
	return runbook.ValidateWithLLM(provider, r, sampleItems)
}

// Deprecated: Use runbook.RefineSelectors instead. Will be removed after 2026-04-15.
func RefineSelectors(provider scout.LLMProvider, html string, selectors map[string]string) (map[string]string, error) {
	return runbook.RefineSelectors(provider, html, selectors)
}

// Deprecated: Use runbook.SelectorHealthCheck instead. Will be removed after 2026-04-15.
func SelectorHealthCheck(page *scout.Page, selectors map[string]string) map[string]int {
	return runbook.SelectorHealthCheck(page, selectors)
}

// Deprecated: Use runbook.WithAI instead. Will be removed after 2026-04-15.
func WithAI(provider scout.LLMProvider) AIRecipeOption { return runbook.WithAI(provider) }

// Deprecated: Use runbook.WithGoal instead. Will be removed after 2026-04-15.
func WithGoal(goal string) AIRecipeOption { return runbook.WithGoal(goal) }

// Deprecated: Use runbook.WithAIModel instead. Will be removed after 2026-04-15.
func WithAIModel(model string) AIRecipeOption { return runbook.WithAIModel(model) }

// Deprecated: Use runbook.WithGenerateType instead. Will be removed after 2026-04-15.
func WithGenerateType(t string) GenerateOption { return runbook.WithGenerateType(t) }

// Deprecated: Use runbook.WithGenerateFields instead. Will be removed after 2026-04-15.
func WithGenerateFields(fields ...string) GenerateOption {
	return runbook.WithGenerateFields(fields...)
}

// Deprecated: Use runbook.WithGenerateMaxPages instead. Will be removed after 2026-04-15.
func WithGenerateMaxPages(n int) GenerateOption { return runbook.WithGenerateMaxPages(n) }

// Deprecated: Use runbook.WithMaxContainers instead. Will be removed after 2026-04-15.
func WithMaxContainers(n int) AnalyzeOption { return runbook.WithMaxContainers(n) }
