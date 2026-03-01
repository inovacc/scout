package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/recipe"
	"github.com/inovacc/scout/recipes"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(recipeCmd)
	recipeCmd.AddCommand(recipeRunCmd, recipeValidateCmd, recipeCreateCmd, recipeTestCmd, recipeFixCmd, recipeSampleCmd, recipeFlowCmd, recipePresetsCmd, recipeRunPresetCmd)

	recipePresetsCmd.Flags().String("service", "", "filter presets by service name")
	recipePresetsCmd.Flags().Bool("json", false, "output as JSON")

	recipeRunPresetCmd.Flags().StringSlice("var", nil, "variable in key=value format (repeatable)")
	recipeRunPresetCmd.Flags().StringP("output", "o", "", "output file for results")

	recipeRunCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	recipeRunCmd.Flags().StringP("output", "o", "", "output file for results")
	_ = recipeRunCmd.MarkFlagRequired("file")

	recipeValidateCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	_ = recipeValidateCmd.MarkFlagRequired("file")

	recipeCreateCmd.Flags().StringP("output", "o", "", "output file for generated recipe")
	recipeCreateCmd.Flags().BoolP("interactive", "i", false, "interactive step-by-step recipe creation")
	recipeCreateCmd.Flags().String("type", "", "force recipe type (extract or automate)")
	recipeCreateCmd.Flags().Int("max-pages", 5, "max pages for pagination")
	recipeCreateCmd.Flags().Bool("ai", false, "use AI-assisted recipe generation via LLM")
	recipeCreateCmd.Flags().String("goal", "", "describe the goal for AI generation (used with --ai)")
	recipeCreateCmd.Flags().String("provider", "ollama", "LLM provider: ollama, openai, anthropic, openrouter, deepseek, gemini")
	recipeCreateCmd.Flags().String("model", "", "LLM model name (provider-specific default)")
	recipeCreateCmd.Flags().String("api-key", "", "API key for remote LLM providers")
	recipeCreateCmd.Flags().String("api-base", "", "custom API base URL for OpenAI-compatible endpoints")

	recipeTestCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	recipeTestCmd.Flags().String("format", "text", "output format (text or json)")
	recipeTestCmd.Flags().Bool("validate-ai", false, "run LLM validation after selector checks")
	recipeTestCmd.Flags().String("provider", "ollama", "LLM provider for --validate-ai: ollama, openai, anthropic, openrouter, deepseek, gemini")
	recipeTestCmd.Flags().String("model", "", "LLM model name for --validate-ai")
	recipeTestCmd.Flags().String("api-key", "", "API key for --validate-ai remote LLM providers")
	recipeTestCmd.Flags().String("api-base", "", "custom API base URL for --validate-ai")
	_ = recipeTestCmd.MarkFlagRequired("file")

	recipeFlowCmd.Flags().StringP("output", "o", "", "output file for generated recipe")
	recipeFlowCmd.Flags().String("name", "", "recipe name (default: flow-recipe)")

	recipeFixCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	recipeFixCmd.Flags().StringP("output", "o", "", "output file for fixed recipe")
	_ = recipeFixCmd.MarkFlagRequired("file")

	recipeSampleCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	recipeSampleCmd.Flags().String("format", "json", "output format (json)")
	_ = recipeSampleCmd.MarkFlagRequired("file")
}

var recipeCmd = &cobra.Command{
	Use:   "recipe",
	Short: "Run or validate declarative recipes",
}

var recipeRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute a recipe file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		output, _ := cmd.Flags().GetString("output")

		r, err := recipe.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}
		defer func() { _ = browser.Close() }()

		result, err := recipe.Run(context.Background(), browser, r)
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

var recipeValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a recipe file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")

		r, err := recipe.LoadFile(file)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(os.Stdout, "valid %s recipe: %s (type=%s)\n", r.Version, r.Name, r.Type)
		return nil
	},
}

var recipeTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Dry-run a recipe: navigate to the URL and validate all selectors",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		format, _ := cmd.Flags().GetString("format")

		r, err := recipe.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}
		defer func() { _ = browser.Close() }()

		result, err := recipe.ValidateRecipe(browser, r)
		if err != nil {
			return err
		}

		// Print selector resilience warnings.
		scores := recipe.ScoreRecipeSelectors(r)
		for name, s := range scores {
			if s.Tier == "fragile" {
				_, _ = fmt.Fprintf(os.Stderr, "warning: fragile selector for %s: %s (score: %.2f, consider using data-* attributes)\n",
					name, s.Selector, s.Score)
			}
		}

		if format == "json" {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal result: %w", err)
			}
			_, _ = fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		// Text output.
		if result.Valid {
			_, _ = fmt.Fprintf(os.Stdout, "PASS: all selectors matched (%d sample items)\n", result.SampleItems)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "FAIL: %d selector(s) did not match\n", len(result.Errors))
			for _, e := range result.Errors {
				_, _ = fmt.Fprintf(os.Stdout, "  %s (%s): %s\n", e.Field, e.Selector, e.Error)
			}
		}

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
					items, sampleErr := recipe.SampleExtract(browser, r)
					if sampleErr == nil {
						samples = items
					}
				}

				llmResult, llmErr := recipe.ValidateWithLLM(provider, r, samples)
				if llmErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "warning: LLM validation failed: %v\n", llmErr)
				} else {
					if llmResult.Valid {
						_, _ = fmt.Fprintln(os.Stdout, "LLM: recipe looks good")
					} else {
						_, _ = fmt.Fprintln(os.Stdout, "LLM: recipe has issues")
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

var recipeCreateCmd = &cobra.Command{
	Use:   "create <url>",
	Short: "Analyze a site and generate a recipe",
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

		var r *recipe.Recipe

		if interactive {
			r, err = recipe.InteractiveCreate(recipe.InteractiveConfig{
				Browser: browser,
				URL:     url,
				Writer:  os.Stderr,
				Reader:  os.Stdin,
			})
			if err != nil {
				return err
			}
		} else if useAI {
			_, _ = fmt.Fprintf(os.Stderr, "generating AI recipe for %s...\n", url)

			providerName, _ := cmd.Flags().GetString("provider")
			model, _ := cmd.Flags().GetString("model")
			apiKey, _ := cmd.Flags().GetString("api-key")
			apiBase, _ := cmd.Flags().GetString("api-base")

			provider, provErr := createProviderFull(providerName, model, apiKey, apiBase, "")
			if provErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: LLM provider setup failed (%v), falling back to rule-based\n", provErr)
			}

			var aiOpts []recipe.AIRecipeOption
			if provider != nil {
				aiOpts = append(aiOpts, recipe.WithAI(provider))
			}
			if goal != "" {
				aiOpts = append(aiOpts, recipe.WithGoal(goal))
			}

			r, err = recipe.GenerateWithAI(browser, url, aiOpts...)
			if err != nil {
				return err
			}
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "analyzing %s...\n", url)

			analysis, analysisErr := recipe.AnalyzeSite(context.Background(), browser, url)
			if analysisErr != nil {
				return analysisErr
			}

			_, _ = fmt.Fprintf(os.Stderr, "detected page type: %s\n", analysis.PageType)
			_, _ = fmt.Fprintf(os.Stderr, "containers: %d, forms: %d\n", len(analysis.Containers), len(analysis.Forms))

			var genOpts []recipe.GenerateOption
			if forceType != "" {
				genOpts = append(genOpts, recipe.WithGenerateType(forceType))
			}
			if maxPages > 0 {
				genOpts = append(genOpts, recipe.WithGenerateMaxPages(maxPages))
			}

			r, err = recipe.GenerateRecipe(analysis, genOpts...)
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
			return fmt.Errorf("marshal recipe: %w", err)
		}

		if output != "" {
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(os.Stderr, "recipe written to %s\n", output)
			return nil
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))
		return nil
	},
}

var recipeFixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Re-analyze a page and fix broken selectors in a recipe",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		output, _ := cmd.Flags().GetString("output")

		r, err := recipe.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}
		defer func() { _ = browser.Close() }()

		fixed, changes, err := recipe.FixRecipe(browser, r)
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
			return fmt.Errorf("marshal recipe: %w", err)
		}

		if output != "" {
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(os.Stderr, "fixed recipe written to %s\n", output)
			return nil
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))
		return nil
	},
}

var recipeSampleCmd = &cobra.Command{
	Use:   "sample",
	Short: "Run a recipe on the first page only and show sample extracted items",
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")

		r, err := recipe.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}
		defer func() { _ = browser.Close() }()

		items, err := recipe.SampleExtract(browser, r)
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

var recipePresetsCmd = &cobra.Command{
	Use:   "presets",
	Short: "List available recipe presets",
	RunE: func(cmd *cobra.Command, _ []string) error {
		service, _ := cmd.Flags().GetString("service")
		asJSON, _ := cmd.Flags().GetBool("json")

		all := recipes.All()
		var filtered []recipes.Preset
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

var recipeRunPresetCmd = &cobra.Command{
	Use:   "run-preset <id>",
	Short: "Run a built-in recipe preset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		vars, _ := cmd.Flags().GetStringSlice("var")
		output, _ := cmd.Flags().GetString("output")

		r, err := recipes.Load(id)
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
		applyVars(r, varMap)

		if unresolved := findUnresolvedVars(r); len(unresolved) > 0 {
			return fmt.Errorf("unresolved variables: %s (use --var key=value)", strings.Join(unresolved, ", "))
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}
		defer func() { _ = browser.Close() }()

		result, err := recipe.Run(context.Background(), browser, r)
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

// applyVars replaces {{key}} placeholders in recipe URLs and step fields.
func applyVars(r *recipe.Recipe, vars map[string]string) {
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		r.URL = strings.ReplaceAll(r.URL, placeholder, v)
		for i := range r.Steps {
			r.Steps[i].URL = strings.ReplaceAll(r.Steps[i].URL, placeholder, v)
			r.Steps[i].Text = strings.ReplaceAll(r.Steps[i].Text, placeholder, v)
		}
	}
}

// findUnresolvedVars returns placeholder names that were not substituted.
func findUnresolvedVars(r *recipe.Recipe) []string {
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

var recipeFlowCmd = &cobra.Command{
	Use:   "flow <url1> [url2] ...",
	Short: "Detect a multi-page flow and generate a recipe",
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

		steps, err := recipe.DetectFlow(browser, args)
		if err != nil {
			return err
		}

		for i, step := range steps {
			_, _ = fmt.Fprintf(os.Stderr, "  step %d: %s (type=%s, login=%v, search=%v)\n",
				i+1, step.URL, step.PageType, step.IsLogin, step.IsSearch)
		}

		r, err := recipe.GenerateFlowRecipe(steps, name)
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal recipe: %w", err)
		}

		if output != "" {
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(os.Stderr, "flow recipe written to %s\n", output)
			return nil
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))
		return nil
	},
}
