package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(githubCmd)

	githubCmd.AddCommand(githubRepoCmd)
	githubCmd.AddCommand(githubIssuesCmd)
	githubCmd.AddCommand(githubPRsCmd)
	githubCmd.AddCommand(githubUserCmd)
	githubCmd.AddCommand(githubReleasesCmd)
	githubCmd.AddCommand(githubTreeCmd)
	githubCmd.AddCommand(githubCodeCmd)

	githubIssuesCmd.Flags().String("state", "open", "filter by state: open, closed, all")
	githubIssuesCmd.Flags().Int("max", 30, "max items to return")
	githubIssuesCmd.Flags().Int("pages", 1, "max pages to fetch")
	githubIssuesCmd.Flags().Bool("body", false, "include issue body")

	githubPRsCmd.Flags().String("state", "open", "filter by state: open, closed, all")
	githubPRsCmd.Flags().Int("max", 30, "max items to return")
	githubPRsCmd.Flags().Int("pages", 1, "max pages to fetch")
	githubPRsCmd.Flags().Bool("body", false, "include PR body")

	githubCodeCmd.Flags().String("repo", "", "scope search to a specific repo (owner/name)")
	githubCodeCmd.Flags().Int("max", 30, "max results to return")
	githubCodeCmd.Flags().Int("pages", 1, "max pages to fetch")

	githubReleasesCmd.Flags().Int("max", 10, "max releases to return")

	githubTreeCmd.Flags().String("branch", "main", "branch name")

	githubRepoCmd.Flags().Bool("readme", false, "include README markdown")
}

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "Extract data from GitHub pages",
	Long:  "Scrape GitHub repository, issue, PR, user, and release data via browser automation.",
}

func parseOwnerName(arg string) (string, string, error) {
	parts := strings.SplitN(arg, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected format: owner/name")
	}

	return parts[0], parts[1], nil
}

var githubRepoCmd = &cobra.Command{
	Use:   "repo <owner/name>",
	Short: "Extract repository metadata",
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

		var opts []scout.GitHubOption
		if readme, _ := cmd.Flags().GetBool("readme"); readme {
			opts = append(opts, scout.WithGitHubBody())
		}

		repo, err := browser.GitHubRepo(owner, name, opts...)
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

		if repo.ReadmeMD != "" {
			_, _ = fmt.Fprintf(w, "\n--- README ---\n%s\n", repo.ReadmeMD)
		}

		return nil
	},
}

var githubIssuesCmd = &cobra.Command{
	Use:   "issues <owner/name>",
	Short: "List repository issues",
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

		var opts []scout.GitHubOption
		if state, _ := cmd.Flags().GetString("state"); state != "" {
			opts = append(opts, scout.WithGitHubState(state))
		}

		if maxItems, _ := cmd.Flags().GetInt("max"); maxItems > 0 {
			opts = append(opts, scout.WithGitHubMaxItems(maxItems))
		}

		if body, _ := cmd.Flags().GetBool("body"); body {
			opts = append(opts, scout.WithGitHubBody())
		}

		if pages, _ := cmd.Flags().GetInt("pages"); pages > 1 {
			opts = append(opts, scout.WithGitHubMaxPages(pages))
		}

		issues, err := browser.GitHubIssues(owner, name, opts...)
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

			_, _ = fmt.Fprintf(w, "#%d %s (%s) by %s%s  %s\n", issue.Number, issue.Title, issue.State, issue.Author, labels, issue.CreatedAt)
			if issue.Body != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", truncate(issue.Body, 200))
			}
		}

		return nil
	},
}

var githubPRsCmd = &cobra.Command{
	Use:   "prs <owner/name>",
	Short: "List repository pull requests",
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

		var opts []scout.GitHubOption
		if state, _ := cmd.Flags().GetString("state"); state != "" {
			opts = append(opts, scout.WithGitHubState(state))
		}

		if maxItems, _ := cmd.Flags().GetInt("max"); maxItems > 0 {
			opts = append(opts, scout.WithGitHubMaxItems(maxItems))
		}

		if body, _ := cmd.Flags().GetBool("body"); body {
			opts = append(opts, scout.WithGitHubBody())
		}

		if pages, _ := cmd.Flags().GetInt("pages"); pages > 1 {
			opts = append(opts, scout.WithGitHubMaxPages(pages))
		}

		prs, err := browser.GitHubPRs(owner, name, opts...)
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

			_, _ = fmt.Fprintf(w, "#%d %s (%s) by %s%s  %s\n", pr.Number, pr.Title, pr.State, pr.Author, labels, pr.CreatedAt)
			if pr.Body != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", truncate(pr.Body, 200))
			}
		}

		return nil
	},
}

var githubUserCmd = &cobra.Command{
	Use:   "user <username>",
	Short: "Extract GitHub user profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		user, err := browser.GitHubUser(username)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(user)
		}

		w := cmd.OutOrStdout()

		_, _ = fmt.Fprintf(w, "%s", user.Username)
		if user.DisplayName != "" {
			_, _ = fmt.Fprintf(w, " (%s)", user.DisplayName)
		}

		_, _ = fmt.Fprintln(w)
		if user.Bio != "" {
			_, _ = fmt.Fprintf(w, "  %s\n", user.Bio)
		}

		if user.Location != "" {
			_, _ = fmt.Fprintf(w, "  Location: %s\n", user.Location)
		}

		_, _ = fmt.Fprintf(w, "  Repos: %d  Followers: %d  Following: %d\n", user.Repos, user.Followers, user.Following)

		return nil
	},
}

var githubReleasesCmd = &cobra.Command{
	Use:   "releases <owner/name>",
	Short: "List repository releases",
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

		var opts []scout.GitHubOption
		if maxItems, _ := cmd.Flags().GetInt("max"); maxItems > 0 {
			opts = append(opts, scout.WithGitHubMaxItems(maxItems))
		}

		releases, err := browser.GitHubReleases(owner, name, opts...)
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
			_, _ = fmt.Fprintf(w, "%s  %s  (%d assets)  %s\n", rel.Tag, rel.Name, rel.Assets, rel.Date)
			if rel.Body != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", truncate(rel.Body, 200))
			}
		}

		return nil
	},
}

var githubCodeCmd = &cobra.Command{
	Use:   "code <query>",
	Short: "Search GitHub code",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		var opts []scout.GitHubOption

		if repo, _ := cmd.Flags().GetString("repo"); repo != "" {
			owner, name, parseErr := parseOwnerName(repo)
			if parseErr != nil {
				return fmt.Errorf("invalid --repo format: %w", parseErr)
			}

			opts = append(opts, scout.WithGitHubRepo(owner, name))
		}

		if maxItems, _ := cmd.Flags().GetInt("max"); maxItems > 0 {
			opts = append(opts, scout.WithGitHubMaxItems(maxItems))
		}

		if pages, _ := cmd.Flags().GetInt("pages"); pages > 1 {
			opts = append(opts, scout.WithGitHubMaxPages(pages))
		}

		results, err := browser.GitHubSearchCode(query, opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(results)
		}

		w := cmd.OutOrStdout()
		for _, r := range results {
			_, _ = fmt.Fprintf(w, "%s  %s\n", r.Repo, r.FilePath)
			if r.Snippet != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", truncate(r.Snippet, 200))
			}
		}

		return nil
	},
}

var githubTreeCmd = &cobra.Command{
	Use:   "tree <owner/name>",
	Short: "List repository file tree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, name, err := parseOwnerName(args[0])
		if err != nil {
			return err
		}

		branch, _ := cmd.Flags().GetString("branch")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		files, err := browser.GitHubTree(owner, name, branch)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(files)
		}

		w := cmd.OutOrStdout()
		for _, f := range files {
			_, _ = fmt.Fprintln(w, f)
		}

		return nil
	},
}
