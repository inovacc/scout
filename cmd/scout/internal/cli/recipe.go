package cli

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
	recipeCmd.AddCommand(recipeRunCmd, recipeValidateCmd)

	recipeRunCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	recipeRunCmd.Flags().StringP("output", "o", "", "output file for results")
	recipeRunCmd.Flags().Bool("headless", true, "run browser in headless mode")
	_ = recipeRunCmd.MarkFlagRequired("file")

	recipeValidateCmd.Flags().StringP("file", "f", "", "recipe JSON file path")
	_ = recipeValidateCmd.MarkFlagRequired("file")
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
		headless, _ := cmd.Flags().GetBool("headless")

		r, err := recipe.LoadFile(file)
		if err != nil {
			return err
		}

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
		)
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
