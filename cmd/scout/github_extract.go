package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	githubCmd.AddCommand(githubExtractRepoCmd)
	githubCmd.AddCommand(githubExtractIssuesCmd)
	githubCmd.AddCommand(githubExtractPRsCmd)
	githubCmd.AddCommand(githubExtractReleasesCmd)

	githubExtractRepoCmd.Flags().Bool("readme", false, "include README HTML")

	githubExtractIssuesCmd.Flags().String("state", "open", "filter by state: open, closed, all")
	githubExtractIssuesCmd.Flags().Int("limit", 25, "max items to return")
	githubExtractIssuesCmd.Flags().Bool("body", false, "include issue body")

	githubExtractPRsCmd.Flags().String("state", "open", "filter by state: open, closed, all")
	githubExtractPRsCmd.Flags().Int("limit", 25, "max items to return")
	githubExtractPRsCmd.Flags().Bool("body", false, "include PR body")

	githubExtractReleasesCmd.Flags().Int("limit", 10, "max releases to return")
	githubExtractReleasesCmd.Flags().Bool("body", false, "include release body")
}

var githubExtractRepoCmd = &cobra.Command{
	Use:   "extract-repo <owner/name>",
	Short: "Extract detailed repository metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, name, err := parseOwnerName(args[0])
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		var opts []scout.GitHubExtractOption
		if readme, _ := cmd.Flags().GetBool("readme"); readme {
			opts = append(opts, scout.WithGitHubReadme())
		}

		repo, err := browser.GitHubExtractRepoInfo(owner, name, opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(repo)
		}

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "%s/%s\n", repo.Owner, repo.Name)
		_, _ = fmt.Fprintf(w, "  %s\n", repo.Description)
		_, _ = fmt.Fprintf(w, "  Language: %s  Stars: %d  Forks: %d\n", repo.Language, repo.Stars, repo.Forks)
		if repo.License != "" {
			_, _ = fmt.Fprintf(w, "  License: %s\n", repo.License)
		}
		if len(repo.Topics) > 0 {
			_, _ = fmt.Fprintf(w, "  Topics: %s\n", strings.Join(repo.Topics, ", "))
		}
		if repo.LastUpdated != "" {
			_, _ = fmt.Fprintf(w, "  Last updated: %s\n", repo.LastUpdated)
		}
		if repo.ReadmeHTML != "" {
			_, _ = fmt.Fprintf(w, "\n--- README ---\n%s\n", repo.ReadmeHTML)
		}

		return writeOutputIfSet(cmd, repo, "github-repo.json")
	},
}

var githubExtractIssuesCmd = &cobra.Command{
	Use:   "extract-issues <owner/name>",
	Short: "Extract issues with unified format (includes comments count)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, name, err := parseOwnerName(args[0])
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		var opts []scout.GitHubExtractOption
		if state, _ := cmd.Flags().GetString("state"); state != "" {
			opts = append(opts, scout.WithGitHubExtractState(state))
		}
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			opts = append(opts, scout.WithGitHubExtractMaxItems(limit))
		}
		if body, _ := cmd.Flags().GetBool("body"); body {
			opts = append(opts, scout.WithGitHubExtractBody())
		}

		issues, err := browser.GitHubExtractIssues(owner, name, opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(issues)
		}

		w := cmd.OutOrStdout()
		for _, issue := range issues {
			labels := ""
			if len(issue.Labels) > 0 {
				labels = " [" + strings.Join(issue.Labels, ", ") + "]"
			}
			_, _ = fmt.Fprintf(w, "#%d %s (%s) by %s%s  comments:%d  %s\n",
				issue.Number, issue.Title, issue.State, issue.Author, labels, issue.Comments, issue.CreatedAt)
			if issue.Body != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", truncate(issue.Body, 200))
			}
		}

		return writeOutputIfSet(cmd, issues, "github-issues.json")
	},
}

var githubExtractPRsCmd = &cobra.Command{
	Use:   "extract-prs <owner/name>",
	Short: "Extract pull requests with unified format",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, name, err := parseOwnerName(args[0])
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		var opts []scout.GitHubExtractOption
		if state, _ := cmd.Flags().GetString("state"); state != "" {
			opts = append(opts, scout.WithGitHubExtractState(state))
		}
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			opts = append(opts, scout.WithGitHubExtractMaxItems(limit))
		}
		if body, _ := cmd.Flags().GetBool("body"); body {
			opts = append(opts, scout.WithGitHubExtractBody())
		}

		prs, err := browser.GitHubExtractPRs(owner, name, opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(prs)
		}

		w := cmd.OutOrStdout()
		for _, pr := range prs {
			labels := ""
			if len(pr.Labels) > 0 {
				labels = " [" + strings.Join(pr.Labels, ", ") + "]"
			}
			_, _ = fmt.Fprintf(w, "#%d %s (%s) by %s%s  comments:%d  %s\n",
				pr.Number, pr.Title, pr.State, pr.Author, labels, pr.Comments, pr.CreatedAt)
			if pr.Body != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", truncate(pr.Body, 200))
			}
		}

		return writeOutputIfSet(cmd, prs, "github-prs.json")
	},
}

var githubExtractReleasesCmd = &cobra.Command{
	Use:   "extract-releases <owner/name>",
	Short: "Extract releases with asset names and author",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, name, err := parseOwnerName(args[0])
		if err != nil {
			return err
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		var opts []scout.GitHubExtractOption
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			opts = append(opts, scout.WithGitHubExtractMaxItems(limit))
		}
		if body, _ := cmd.Flags().GetBool("body"); body {
			opts = append(opts, scout.WithGitHubExtractBody())
		}

		releases, err := browser.GitHubExtractReleases(owner, name, opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(releases)
		}

		w := cmd.OutOrStdout()
		for _, rel := range releases {
			assets := ""
			if len(rel.Assets) > 0 {
				assets = fmt.Sprintf("  assets: %s", strings.Join(rel.Assets, ", "))
			}
			_, _ = fmt.Fprintf(w, "%s  %s  by %s  %s%s\n", rel.Tag, rel.Name, rel.Author, rel.PublishedAt, assets)
			if rel.Body != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", truncate(rel.Body, 200))
			}
		}

		return writeOutputIfSet(cmd, releases, "github-releases.json")
	},
}

// writeOutputIfSet writes JSON to --output file if the flag is set.
func writeOutputIfSet(cmd *cobra.Command, v interface{}, defaultName string) error {
	outFile, _ := cmd.Flags().GetString("output")
	if outFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: marshal json: %w", err)
	}
	dest, writeErr := writeOutput(cmd, data, defaultName)
	if writeErr != nil {
		return writeErr
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Written to %s\n", dest)
	return nil
}
