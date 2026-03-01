// Package recipe is deprecated. Use package runbook instead.
//
// This package provides type aliases and wrapper functions that forward to the
// runbook package for backward compatibility.
package recipe

import (
	"context"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/runbook"
)

// Deprecated: Use runbook.Runbook instead.
type Recipe = runbook.Runbook

// Deprecated: Use runbook.ItemSpec instead.
type ItemSpec = runbook.ItemSpec

// Deprecated: Use runbook.Pagination instead.
type Pagination = runbook.Pagination

// Deprecated: Use runbook.Step instead.
type Step = runbook.Step

// Deprecated: Use runbook.Output instead.
type Output = runbook.Output

// Deprecated: Use runbook.Result instead.
type Result = runbook.Result

// Deprecated: Use runbook.SelectorScore instead.
type SelectorScore = runbook.SelectorScore

// Deprecated: Use runbook.ValidationResult instead.
type ValidationResult = runbook.ValidationResult

// Deprecated: Use runbook.ValidationError instead.
type ValidationError = runbook.ValidationError

// Deprecated: Use runbook.SiteAnalysis instead.
type SiteAnalysis = runbook.SiteAnalysis

// Deprecated: Use runbook.ContainerCandidate instead.
type ContainerCandidate = runbook.ContainerCandidate

// Deprecated: Use runbook.FieldCandidate instead.
type FieldCandidate = runbook.FieldCandidate

// Deprecated: Use runbook.FormCandidate instead.
type FormCandidate = runbook.FormCandidate

// Deprecated: Use runbook.FormFieldCandidate instead.
type FormFieldCandidate = runbook.FormFieldCandidate

// Deprecated: Use runbook.PaginationCandidate instead.
type PaginationCandidate = runbook.PaginationCandidate

// Deprecated: Use runbook.InteractableElement instead.
type InteractableElement = runbook.InteractableElement

// Deprecated: Use runbook.GenerateOption instead.
type GenerateOption = runbook.GenerateOption

// Deprecated: Use runbook.AnalyzeOption instead.
type AnalyzeOption = runbook.AnalyzeOption

// Deprecated: Use runbook.AIRunbookOption instead.
type AIRecipeOption = runbook.AIRunbookOption

// Deprecated: Use runbook.InteractiveConfig instead.
type InteractiveConfig = runbook.InteractiveConfig

// Deprecated: Use runbook.FlowStep instead.
type FlowStep = runbook.FlowStep

// Deprecated: Use runbook.FormInfo instead.
type FormInfo = runbook.FormInfo

// Deprecated: Use runbook.LLMValidation instead.
type LLMValidation = runbook.LLMValidation

// Deprecated: Use runbook.LoadFile instead.
func LoadFile(path string) (*Recipe, error) { return runbook.LoadFile(path) }

// Deprecated: Use runbook.Parse instead.
func Parse(data []byte) (*Recipe, error) { return runbook.Parse(data) }

// Deprecated: Use runbook.Apply instead.
func Run(ctx context.Context, browser *scout.Browser, r *Recipe) (*Result, error) {
	return runbook.Apply(ctx, browser, r)
}

// Deprecated: Use runbook.ValidateRunbook instead.
func ValidateRecipe(browser *scout.Browser, r *Recipe) (*ValidationResult, error) {
	return runbook.ValidateRunbook(browser, r)
}

// Deprecated: Use runbook.ScoreSelector instead.
func ScoreSelector(sel string) SelectorScore { return runbook.ScoreSelector(sel) }

// Deprecated: Use runbook.ScoreRunbookSelectors instead.
func ScoreRecipeSelectors(r *Recipe) map[string]SelectorScore {
	return runbook.ScoreRunbookSelectors(r)
}

// Deprecated: Use runbook.FixRunbook instead.
func FixRecipe(browser *scout.Browser, r *Recipe) (*Recipe, []string, error) {
	return runbook.FixRunbook(browser, r)
}

// Deprecated: Use runbook.SampleExtract instead.
func SampleExtract(browser *scout.Browser, r *Recipe) ([]map[string]any, error) {
	return runbook.SampleExtract(browser, r)
}

// Deprecated: Use runbook.GenerateRunbook instead.
func GenerateRecipe(analysis *SiteAnalysis, opts ...GenerateOption) (*Recipe, error) {
	return runbook.GenerateRunbook(analysis, opts...)
}

// Deprecated: Use runbook.GenerateFlowRunbook instead.
func GenerateFlowRecipe(steps []FlowStep, name string) (*Recipe, error) {
	return runbook.GenerateFlowRunbook(steps, name)
}

// Deprecated: Use runbook.GenerateWithAI instead.
func GenerateWithAI(browser *scout.Browser, url string, opts ...AIRecipeOption) (*Recipe, error) {
	return runbook.GenerateWithAI(browser, url, opts...)
}

// Deprecated: Use runbook.AnalyzeSite instead.
func AnalyzeSite(ctx context.Context, browser *scout.Browser, url string, opts ...AnalyzeOption) (*SiteAnalysis, error) {
	return runbook.AnalyzeSite(ctx, browser, url, opts...)
}

// Deprecated: Use runbook.InteractiveCreate instead.
func InteractiveCreate(cfg InteractiveConfig) (*Recipe, error) {
	return runbook.InteractiveCreate(cfg)
}

// Deprecated: Use runbook.DetectFlow instead.
func DetectFlow(browser *scout.Browser, urls []string) ([]FlowStep, error) {
	return runbook.DetectFlow(browser, urls)
}

// Deprecated: Use runbook.ValidateWithLLM instead.
func ValidateWithLLM(provider scout.LLMProvider, r *Recipe, sampleItems []map[string]any) (*LLMValidation, error) {
	return runbook.ValidateWithLLM(provider, r, sampleItems)
}

// Deprecated: Use runbook.RefineSelectors instead.
func RefineSelectors(provider scout.LLMProvider, html string, selectors map[string]string) (map[string]string, error) {
	return runbook.RefineSelectors(provider, html, selectors)
}

// Deprecated: Use runbook.SelectorHealthCheck instead.
func SelectorHealthCheck(page *scout.Page, selectors map[string]string) map[string]int {
	return runbook.SelectorHealthCheck(page, selectors)
}

// Deprecated: Use runbook.WithAI instead.
func WithAI(provider scout.LLMProvider) AIRecipeOption { return runbook.WithAI(provider) }

// Deprecated: Use runbook.WithGoal instead.
func WithGoal(goal string) AIRecipeOption { return runbook.WithGoal(goal) }

// Deprecated: Use runbook.WithAIModel instead.
func WithAIModel(model string) AIRecipeOption { return runbook.WithAIModel(model) }

// Deprecated: Use runbook.WithGenerateType instead.
func WithGenerateType(t string) GenerateOption { return runbook.WithGenerateType(t) }

// Deprecated: Use runbook.WithGenerateFields instead.
func WithGenerateFields(fields ...string) GenerateOption { return runbook.WithGenerateFields(fields...) }

// Deprecated: Use runbook.WithGenerateMaxPages instead.
func WithGenerateMaxPages(n int) GenerateOption { return runbook.WithGenerateMaxPages(n) }

// Deprecated: Use runbook.WithMaxContainers instead.
func WithMaxContainers(n int) AnalyzeOption { return runbook.WithMaxContainers(n) }
