package main

import (
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mapCmd)

	mapCmd.Flags().Int("limit", 1000, "maximum URLs to discover")
	mapCmd.Flags().Bool("include-subdomains", false, "include subdomain URLs")
	mapCmd.Flags().StringSlice("include-paths", nil, "only include URLs with these path prefixes")
	mapCmd.Flags().StringSlice("exclude-paths", nil, "exclude URLs with these path prefixes")
	mapCmd.Flags().String("search", "", "filter URLs containing this term")
	mapCmd.Flags().Bool("no-sitemap", false, "skip sitemap.xml parsing")
	mapCmd.Flags().Duration("delay", 200*time.Millisecond, "delay between page visits")
	mapCmd.Flags().Int("max-depth", 2, "link-follow depth for on-page discovery")
}

var mapCmd = &cobra.Command{
	Use:   "map <url>",
	Short: "Discover all URLs on a site via sitemap + link harvesting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		includeSubs, _ := cmd.Flags().GetBool("include-subdomains")
		includePaths, _ := cmd.Flags().GetStringSlice("include-paths")
		excludePaths, _ := cmd.Flags().GetStringSlice("exclude-paths")
		search, _ := cmd.Flags().GetString("search")
		noSitemap, _ := cmd.Flags().GetBool("no-sitemap")
		delay, _ := cmd.Flags().GetDuration("delay")
		maxDepth, _ := cmd.Flags().GetInt("max-depth")

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		var opts []scout.MapOption
		opts = append(opts, scout.WithMapLimit(limit))
		opts = append(opts, scout.WithMapDelay(delay))
		opts = append(opts, scout.WithMapMaxDepth(maxDepth))
		if includeSubs {
			opts = append(opts, scout.WithMapSubdomains())
		}
		if len(includePaths) > 0 {
			opts = append(opts, scout.WithMapIncludePaths(includePaths...))
		}
		if len(excludePaths) > 0 {
			opts = append(opts, scout.WithMapExcludePaths(excludePaths...))
		}
		if search != "" {
			opts = append(opts, scout.WithMapSearch(search))
		}
		if noSitemap {
			opts = append(opts, scout.WithMapSitemap(false))
		}

		urls, err := browser.Map(args[0], opts...)
		if err != nil {
			return fmt.Errorf("scout: map: %w", err)
		}

		for _, u := range urls {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), u)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nDiscovered %d URLs\n", len(urls))
		return nil
	},
}
