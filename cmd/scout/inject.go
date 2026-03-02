package main

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(injectCmd)

	injectCmd.Flags().StringArray("code", nil, "raw JavaScript code to inject (repeatable)")
	injectCmd.Flags().StringArray("file", nil, "JavaScript file to inject (repeatable)")
	injectCmd.Flags().String("dir", "", "directory of .js files to inject")
}

var injectCmd = &cobra.Command{
	Use:   "inject <url>",
	Short: "Open a page with injected JavaScript",
	Long:  "Opens a page with custom JavaScript injected via EvalOnNewDocument before any page scripts run.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		codes, _ := cmd.Flags().GetStringArray("code")
		files, _ := cmd.Flags().GetStringArray("file")
		dir, _ := cmd.Flags().GetString("dir")

		if len(codes) == 0 && len(files) == 0 && dir == "" {
			return fmt.Errorf("scout: inject: at least one of --code, --file, or --dir is required")
		}

		opts := baseOpts(cmd)

		if len(codes) > 0 {
			opts = append(opts, scout.WithInjectCode(codes...))
		}

		if len(files) > 0 {
			opts = append(opts, scout.WithInjectJS(files...))
		}

		if dir != "" {
			opts = append(opts, scout.WithInjectDir(dir))
		}

		browser, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: inject: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: inject: navigate: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: inject: wait load: %w", err)
		}

		title, err := page.Title()
		if err != nil {
			return fmt.Errorf("scout: inject: get title: %w", err)
		}

		pageURL, err := page.URL()
		if err != nil {
			return fmt.Errorf("scout: inject: get url: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\nURL:   %s\n", title, pageURL)

		return nil
	},
}
