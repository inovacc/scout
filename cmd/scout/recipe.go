package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/recipe"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(recipeCmd)
	recipeCmd.AddCommand(recipeRunCmd, recipeValidateCmd, recipeCreateCmd, recipeTestCmd)

	recipeRunCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	recipeRunCmd.Flags().StringP("output", "o", "", "output file for results")
	_ = recipeRunCmd.MarkFlagRequired("file")

	recipeValidateCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	_ = recipeValidateCmd.MarkFlagRequired("file")

	recipeCreateCmd.Flags().StringP("output", "o", "", "output file for generated recipe")
	recipeCreateCmd.Flags().String("type", "", "force recipe type (extract or automate)")
	recipeCreateCmd.Flags().Int("max-pages", 5, "max pages for pagination")

	recipeTestCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	recipeTestCmd.Flags().String("format", "text", "output format (text or json)")
	_ = recipeTestCmd.MarkFlagRequired("file")
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

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: browser launch: %w", err)
		}
		defer func() { _ = browser.Close() }()

		_, _ = fmt.Fprintf(os.Stderr, "analyzing %s...\n", url)

		analysis, err := recipe.AnalyzeSite(context.Background(), browser, url)
		if err != nil {
			return err
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

		r, err := recipe.GenerateRecipe(analysis, genOpts...)
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
			_, _ = fmt.Fprintf(os.Stderr, "recipe written to %s\n", output)
			return nil
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))
		return nil
	},
}
