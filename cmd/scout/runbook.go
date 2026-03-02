package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/runbook"
	"github.com/inovacc/scout/pkg/scout/runbooks"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runbookCmd)
	runbookCmd.AddCommand(runbookApplyCmd, runbookValidateCmd, runbookCreateCmd, runbookPlanCmd, runbookFixCmd, runbookSampleCmd, runbookFlowCmd, runbookPresetsCmd, runbookRunPresetCmd)

	runbookPresetsCmd.Flags().String("service", "", "filter presets by service name")
	runbookPresetsCmd.Flags().Bool("json", false, "output as JSON")

	runbookRunPresetCmd.Flags().StringSlice("var", nil, "variable in key=value format (repeatable)")
	runbookRunPresetCmd.Flags().StringP("output", "o", "", "output file for results")

	runbookApplyCmd.Flags().StringP("file", "f", "", "runbook JSON file path")
	runbookApplyCmd.Flags().StringP("output", "o", "", "output file for results")
	_ = runbookApplyCmd.MarkFlagRequired("file")

	runbookValidateCmd.Flags().StringP("file", "f", "", "runbook JSON file path")
	_ = runbookValidateCmd.MarkFlagRequired("file")

	runbookCreateCmd.Flags().StringP("output", "o", "", "output file for generated runbook")
	runbookCreateCmd.Flags().BoolP("interactive", "i", false, "interactive step-by-step runbook creation")
	runbookCreateCmd.Flags().String("type", "", "force runbook type (extract or automate)")
	runbookCreateCmd.Flags().Int("max-pages", 5, "max pages for pagination")
	runbookCreateCmd.Flags().Bool("ai", false, "use AI-assisted runbook generation via LLM")
	runbookCreateCmd.Flags().String("goal", "", "describe the goal for AI generation (used with --ai)")
	runbookCreateCmd.Flags().String("provider", "ollama", "LLM provider: ollama, openai, anthropic, openrouter, deepseek, gemini")
	runbookCreateCmd.Flags().String("model", "", "LLM model name (provider-specific default)")
	runbookCreateCmd.Flags().String("api-key", "", "API key for remote LLM providers")
	runbookCreateCmd.Flags().String("api-base", "", "custom API base URL for OpenAI-compatible endpoints")

	runbookPlanCmd.Flags().StringP("file", "f", "", "runbook JSON file path")
	runbookPlanCmd.Flags().String("format", "text", "output format (text or json)")
	runbookPlanCmd.Flags().Bool("validate-ai", false, "run LLM validation after selector checks")
	runbookPlanCmd.Flags().String("provider", "ollama", "LLM provider for --validate-ai: ollama, openai, anthropic, openrouter, deepseek, gemini")
	runbookPlanCmd.Flags().String("model", "", "LLM model name for --validate-ai")
	runbookPlanCmd.Flags().String("api-key", "", "API key for --validate-ai remote LLM providers")
	runbookPlanCmd.Flags().String("api-base", "", "custom API base URL for --validate-ai")
	_ = runbookPlanCmd.MarkFlagRequired("file")

	runbookFlowCmd.Flags().StringP("output", "o", "", "output file for generated runbook")
	runbookFlowCmd.Flags().String("name", "", "runbook name (default: flow-runbook)")

	runbookFixCmd.Flags().StringP("file", "f", "", "runbook JSON file path")
	runbookFixCmd.Flags().StringP("output", "o", "", "output file for fixed runbook")
	_ = runbookFixCmd.MarkFlagRequired("file")

	runbookSampleCmd.Flags().StringP("file", "f", "", "runbook JSON file path")
	runbookSampleCmd.Flags().String("format", "json", "output format (json)")
	_ = runbookSampleCmd.MarkFlagRequired("file")
}

var runbookCmd = &cobra.Command{
	Use:   "runbook",
	Short: "Plan, apply, or validate declarative runbooks",
}

var runbookApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Execute a runbook file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		output, _ := cmd.Flags().GetString("output")

		r, err := runbook.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}

		defer func() { _ = browser.Close() }()

		result, err := runbook.Apply(context.Background(), browser, r)
		if err != nil {
			return err
		}

		// Save screenshots
		if len(result.Screenshots) > 0 {
			dir := filepath.Dir(file)
			for name, data := range result.Screenshots {
				path := filepath.Join(dir, name+".png")
				if err := os.WriteFile(path, data, 0o644); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "warning: save screenshot %s: %v\n", name, err)
				} else {
					_, _ = fmt.Fprintf(os.Stderr, "screenshot: %s\n", path)
				}
			}
		}

		// Output results
		out := struct {
			Items     []map[string]string `json:"items,omitempty"`
			Variables map[string]any      `json:"variables,omitempty"`
		}{
			Items:     result.Items,
			Variables: result.Variables,
		}

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal results: %w", err)
		}

		if output != "" {
			return os.WriteFile(output, data, 0o644)
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	},
}

var runbookValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a runbook file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")

		r, err := runbook.LoadFile(file)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(os.Stdout, "valid %s runbook: %s (type=%s)\n", r.Version, r.Name, r.Type)

		return nil
	},
}

var runbookPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Dry-run a runbook: navigate to the URL and validate all selectors",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		format, _ := cmd.Flags().GetString("format")

		r, err := runbook.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}

		defer func() { _ = browser.Close() }()

		plan, err := runbook.Plan(browser, r)
		if err != nil {
			return err
		}

		if format == "json" {
			data, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal plan: %w", err)
			}

			_, _ = fmt.Fprintln(os.Stdout, string(data))

			return nil
		}

		// Text output (terraform-style).
		_, _ = fmt.Fprint(os.Stdout, plan.String())

		// Optional LLM validation.
		validateAI, _ := cmd.Flags().GetBool("validate-ai")
		if validateAI {
			providerName, _ := cmd.Flags().GetString("provider")
			model, _ := cmd.Flags().GetString("model")
			apiKey, _ := cmd.Flags().GetString("api-key")
			apiBase, _ := cmd.Flags().GetString("api-base")

			provider, provErr := createProviderFull(providerName, model, apiKey, apiBase, "")
			if provErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: LLM provider setup failed: %v\n", provErr)
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "running LLM validation with %s...\n", provider.Name())

				// Try to get sample items for context.
				var samples []map[string]any

				if r.Type == "extract" {
					items, sampleErr := runbook.SampleExtract(browser, r)
					if sampleErr == nil {
						samples = items
					}
				}

				llmResult, llmErr := runbook.ValidateWithLLM(provider, r, samples)
				if llmErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "warning: LLM validation failed: %v\n", llmErr)
				} else {
					if llmResult.Valid {
						_, _ = fmt.Fprintln(os.Stdout, "LLM: runbook looks good")
					} else {
						_, _ = fmt.Fprintln(os.Stdout, "LLM: runbook has issues")
					}

					for _, s := range llmResult.Suggestions {
						_, _ = fmt.Fprintf(os.Stdout, "  suggestion: %s\n", s)
					}

					for _, f := range llmResult.MissingFields {
						_, _ = fmt.Fprintf(os.Stdout, "  missing field: %s\n", f)
					}

					for _, fs := range llmResult.FragileSelectors {
						_, _ = fmt.Fprintf(os.Stdout, "  fragile: %s\n", fs)
					}
				}
			}
		}

		return nil
	},
}

var runbookCreateCmd = &cobra.Command{
	Use:   "create <url>",
	Short: "Analyze a site and generate a runbook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		output, _ := cmd.Flags().GetString("output")
		forceType, _ := cmd.Flags().GetString("type")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		useAI, _ := cmd.Flags().GetBool("ai")
		goal, _ := cmd.Flags().GetString("goal")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}

		defer func() { _ = browser.Close() }()

		interactive, _ := cmd.Flags().GetBool("interactive")

		var r *runbook.Runbook

		if interactive {
			r, err = runbook.InteractiveCreate(runbook.InteractiveConfig{
				Browser: browser,
				URL:     url,
				Writer:  os.Stderr,
				Reader:  os.Stdin,
			})
			if err != nil {
				return err
			}
		} else if useAI {
			_, _ = fmt.Fprintf(os.Stderr, "generating AI runbook for %s...\n", url)

			providerName, _ := cmd.Flags().GetString("provider")
			model, _ := cmd.Flags().GetString("model")
			apiKey, _ := cmd.Flags().GetString("api-key")
			apiBase, _ := cmd.Flags().GetString("api-base")

			provider, provErr := createProviderFull(providerName, model, apiKey, apiBase, "")
			if provErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: LLM provider setup failed (%v), falling back to rule-based\n", provErr)
			}

			var aiOpts []runbook.AIRunbookOption
			if provider != nil {
				aiOpts = append(aiOpts, runbook.WithAI(provider))
			}

			if goal != "" {
				aiOpts = append(aiOpts, runbook.WithGoal(goal))
			}

			r, err = runbook.GenerateWithAI(browser, url, aiOpts...)
			if err != nil {
				return err
			}
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "analyzing %s...\n", url)

			analysis, analysisErr := runbook.AnalyzeSite(context.Background(), browser, url)
			if analysisErr != nil {
				return analysisErr
			}

			_, _ = fmt.Fprintf(os.Stderr, "detected page type: %s\n", analysis.PageType)
			_, _ = fmt.Fprintf(os.Stderr, "containers: %d, forms: %d\n", len(analysis.Containers), len(analysis.Forms))

			var genOpts []runbook.GenerateOption
			if forceType != "" {
				genOpts = append(genOpts, runbook.WithGenerateType(forceType))
			}

			if maxPages > 0 {
				genOpts = append(genOpts, runbook.WithGenerateMaxPages(maxPages))
			}

			r, err = runbook.GenerateRunbook(analysis, genOpts...)
			if err != nil {
				return err
			}
		}

		// Print selector resilience warnings.
		for _, w := range r.Warnings {
			_, _ = fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}

		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal runbook: %w", err)
		}

		if output != "" {
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(os.Stderr, "runbook written to %s\n", output)

			return nil
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	},
}

var runbookFixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Re-analyze a page and fix broken selectors in a runbook",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		output, _ := cmd.Flags().GetString("output")

		r, err := runbook.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}

		defer func() { _ = browser.Close() }()

		fixed, changes, err := runbook.FixRunbook(browser, r)
		if err != nil {
			return err
		}

		if len(changes) == 0 {
			_, _ = fmt.Fprintln(os.Stderr, "all selectors are healthy, no fixes needed")
		} else {
			for _, c := range changes {
				_, _ = fmt.Fprintf(os.Stderr, "fixed: %s\n", c)
			}
		}

		data, err := json.MarshalIndent(fixed, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal runbook: %w", err)
		}

		if output != "" {
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(os.Stderr, "fixed runbook written to %s\n", output)

			return nil
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	},
}

var runbookSampleCmd = &cobra.Command{
	Use:   "sample",
	Short: "Run a runbook on the first page only and show sample extracted items",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")

		r, err := runbook.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}

		defer func() { _ = browser.Close() }()

		items, err := runbook.SampleExtract(browser, r)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(os.Stderr, "extracted %d sample items\n", len(items))

		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal sample: %w", err)
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	},
}

var runbookPresetsCmd = &cobra.Command{
	Use:   "presets",
	Short: "List available runbook presets",
	RunE: func(cmd *cobra.Command, _ []string) error {
		service, _ := cmd.Flags().GetString("service")
		asJSON, _ := cmd.Flags().GetBool("json")

		all := runbooks.All()

		var filtered []runbooks.Preset

		for _, p := range all {
			if service == "" || p.Service == service {
				filtered = append(filtered, p)
			}
		}

		if asJSON {
			data, err := json.MarshalIndent(filtered, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal presets: %w", err)
			}

			_, _ = fmt.Fprintln(os.Stdout, string(data))

			return nil
		}

		for _, p := range filtered {
			_, _ = fmt.Fprintf(os.Stdout, "%-25s %-12s %s\n", p.ID, p.Service, p.Description)
		}

		return nil
	},
}

var runbookRunPresetCmd = &cobra.Command{
	Use:   "run-preset <id>",
	Short: "Run a built-in runbook preset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		vars, _ := cmd.Flags().GetStringSlice("var")
		output, _ := cmd.Flags().GetString("output")

		r, err := runbooks.Load(id)
		if err != nil {
			return err
		}

		varMap := make(map[string]string)

		for _, v := range vars {
			k, val, ok := strings.Cut(v, "=")
			if !ok {
				return fmt.Errorf("invalid --var format %q (expected key=value)", v)
			}

			varMap[k] = val
		}

		applyRunbookVars(r, varMap)

		if unresolved := findUnresolvedRunbookVars(r); len(unresolved) > 0 {
			return fmt.Errorf("unresolved variables: %s (use --var key=value)", strings.Join(unresolved, ", "))
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}

		defer func() { _ = browser.Close() }()

		result, err := runbook.Apply(context.Background(), browser, r)
		if err != nil {
			return err
		}

		out := struct {
			Items     []map[string]string `json:"items,omitempty"`
			Variables map[string]any      `json:"variables,omitempty"`
		}{
			Items:     result.Items,
			Variables: result.Variables,
		}

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal results: %w", err)
		}

		if output != "" {
			return os.WriteFile(output, data, 0o644)
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	},
}

// applyRunbookVars replaces {{key}} placeholders in runbook URLs and step fields.
func applyRunbookVars(r *runbook.Runbook, vars map[string]string) {
	for k, v := range vars {
		placeholder := "{{" + k + "}}"

		r.URL = strings.ReplaceAll(r.URL, placeholder, v)
		for i := range r.Steps {
			r.Steps[i].URL = strings.ReplaceAll(r.Steps[i].URL, placeholder, v)
			r.Steps[i].Text = strings.ReplaceAll(r.Steps[i].Text, placeholder, v)
		}
	}
}

// findUnresolvedRunbookVars returns placeholder names that were not substituted.
func findUnresolvedRunbookVars(r *runbook.Runbook) []string {
	seen := make(map[string]bool)
	scan := func(s string) {
		for {
			start := strings.Index(s, "{{")
			if start < 0 {
				return
			}

			end := strings.Index(s[start:], "}}")
			if end < 0 {
				return
			}

			name := s[start+2 : start+end]
			if name != "" && !seen[name] {
				seen[name] = true
			}

			s = s[start+end+2:]
		}
	}
	scan(r.URL)

	for _, step := range r.Steps {
		scan(step.URL)
		scan(step.Text)
	}

	var names []string
	for name := range seen {
		names = append(names, name)
	}

	return names
}

var runbookFlowCmd = &cobra.Command{
	Use:   "flow <url1> [url2] ...",
	Short: "Detect a multi-page flow and generate a runbook",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		name, _ := cmd.Flags().GetString("name")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}

		defer func() { _ = browser.Close() }()

		_, _ = fmt.Fprintf(os.Stderr, "detecting flow across %d URL(s)...\n", len(args))

		steps, err := runbook.DetectFlow(browser, args)
		if err != nil {
			return err
		}

		for i, step := range steps {
			_, _ = fmt.Fprintf(os.Stderr, "  step %d: %s (type=%s, login=%v, search=%v)\n",
				i+1, step.URL, step.PageType, step.IsLogin, step.IsSearch)
		}

		r, err := runbook.GenerateFlowRunbook(steps, name)
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal runbook: %w", err)
		}

		if output != "" {
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(os.Stderr, "flow runbook written to %s\n", output)

			return nil
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	},
}
