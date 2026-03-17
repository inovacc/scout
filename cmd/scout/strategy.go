package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/strategy"
	"github.com/spf13/cobra"

	// Register all scraper modes.
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/amazon"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/cloud"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/confluence"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/discord"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/gdrive"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/gmail"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/gmaps"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/grafana"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/jira"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/linkedin"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/notion"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/outlook"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/reddit"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/salesforce"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/sharepoint"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/slack"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/teams"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/twitter"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/youtube"
)

func init() {
	rootCmd.AddCommand(strategyCmd)
	strategyCmd.AddCommand(strategyRunCmd)
	strategyCmd.AddCommand(strategyValidateCmd)
	strategyCmd.AddCommand(strategyInitCmd)

	strategyRunCmd.Flags().StringP("file", "f", "", "strategy file path")
	strategyRunCmd.Flags().Bool("dry-run", false, "validate only, do not execute")
	strategyRunCmd.Flags().BoolP("verbose", "", false, "verbose progress output")

	strategyInitCmd.Flags().StringP("name", "n", "my-strategy", "strategy name")
	strategyInitCmd.Flags().StringP("output", "", "strategy.yaml", "output file path")
}

var strategyCmd = &cobra.Command{
	Use:   "strategy",
	Short: "Declarative multi-step browser automation workflows",
	Long:  "Load, validate, and execute YAML/JSON strategy files that compose auth, scraping, and output sinks.",
}

var strategyRunCmd = &cobra.Command{
	Use:   "run -f <strategy.yaml>",
	Short: "Execute a strategy file",
	Long:  "Load a strategy file, set up browser and auth, execute all steps, and write results to configured sinks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if file == "" && len(args) > 0 {
			file = args[0]
		}

		if file == "" {
			return fmt.Errorf("strategy file required: use -f <file> or pass as argument")
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		verbose, _ := cmd.Flags().GetBool("verbose")

		s, err := strategy.LoadFile(file)
		if err != nil {
			return fmt.Errorf("scout: strategy: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigCh
			signal.Stop(sigCh)
			cancel()
		}()

		opts := strategy.ExecuteOptions{
			DryRun: dryRun,
			Progress: func(step, phase, msg string) {
				if verbose || dryRun {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[%s/%s] %s\n", step, phase, msg)
				}
			},
			Logger: slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), nil)),
			ModeResolver: func(name string) (scraper.Mode, error) {
				mode, err := scraper.GetMode(name)
				if err != nil {
					// Fallback to plugin-provided modes.
					mgr := initPluginManager()
					if pluginMode, ok := mgr.GetMode(name); ok {
						return pluginMode, nil
					}

					return nil, err
				}

				return mode, nil
			},
		}

		if err := strategy.Execute(ctx, s, opts); err != nil {
			return fmt.Errorf("scout: strategy: %w", err)
		}

		if !dryRun {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "strategy %q completed successfully\n", s.Name)
		}

		return nil
	},
}

var strategyValidateCmd = &cobra.Command{
	Use:   "validate -f <strategy.yaml>",
	Short: "Validate a strategy file without executing",
	RunE: func(cmd *cobra.Command, args []string) error {
		file := ""
		if len(args) > 0 {
			file = args[0]
		}

		if file == "" {
			return fmt.Errorf("strategy file required: pass as argument")
		}

		s, err := strategy.LoadFile(file)
		if err != nil {
			return fmt.Errorf("scout: strategy: %w", err)
		}

		if err := strategy.Validate(s); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "strategy %q is valid (%d steps, %d sinks)\n",
			s.Name, len(s.Steps), len(s.Output.Sinks))

		return nil
	},
}

var strategyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a strategy template file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		output, _ := cmd.Flags().GetString("output")

		if _, err := os.Stat(output); err == nil {
			return fmt.Errorf("file %q already exists", output)
		}

		if err := os.WriteFile(output, []byte(strategy.Template), 0o600); err != nil {
			return fmt.Errorf("scout: strategy init: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", output)

		return nil
	},
}
