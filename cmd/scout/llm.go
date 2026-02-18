package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(extractAICmd)
	rootCmd.AddCommand(ollamaCmd)
	rootCmd.AddCommand(aiJobCmd)
	ollamaCmd.AddCommand(ollamaListCmd, ollamaPullCmd, ollamaStatusCmd)
	aiJobCmd.AddCommand(aiJobListCmd, aiJobShowCmd, aiJobSessionCmd)
	aiJobSessionCmd.AddCommand(aiJobSessionListCmd, aiJobSessionCreateCmd, aiJobSessionUseCmd)

	// extract-ai flags
	extractAICmd.Flags().String("url", "", "URL to extract from (required)")
	extractAICmd.Flags().String("prompt", "", "extraction prompt (required)")
	extractAICmd.Flags().String("provider", "ollama", "LLM provider: ollama, openai, anthropic, openrouter, deepseek, gemini")
	extractAICmd.Flags().String("model", "", "model name (default: provider-specific)")
	extractAICmd.Flags().String("api-key", "", "API key for remote providers (or use env: OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.)")
	extractAICmd.Flags().String("api-base", "", "custom API base URL (for OpenAI-compatible endpoints)")
	extractAICmd.Flags().String("schema", "", "path to JSON schema file for response validation")
	extractAICmd.Flags().Bool("main-only", true, "extract main content only")
	extractAICmd.Flags().String("system-prompt", "", "override system prompt")
	extractAICmd.Flags().Duration("timeout", 60*time.Second, "LLM request timeout")
	extractAICmd.Flags().String("ollama-host", "", "Ollama server URL")

	// Review flags
	extractAICmd.Flags().Bool("review", false, "enable review by a second LLM")
	extractAICmd.Flags().String("review-provider", "", "review LLM provider (defaults to --provider)")
	extractAICmd.Flags().String("review-model", "", "review LLM model")
	extractAICmd.Flags().String("review-api-key", "", "API key for review provider (or use env)")
	extractAICmd.Flags().String("review-api-base", "", "custom API base URL for review provider")
	extractAICmd.Flags().String("review-prompt", "", "override review system prompt")

	// Workspace flags
	extractAICmd.Flags().String("workspace", "", "workspace folder for session/job persistence")
	extractAICmd.Flags().String("session-id", "", "session ID (default: current session)")
	extractAICmd.Flags().StringSlice("meta", nil, "metadata key=value pairs (repeatable)")

	// ai-job flags
	aiJobListCmd.Flags().String("workspace", "", "workspace folder path (required)")
	aiJobListCmd.Flags().String("session-id", "", "filter by session ID")
	aiJobShowCmd.Flags().String("workspace", "", "workspace folder path (required)")
	aiJobSessionListCmd.Flags().String("workspace", "", "workspace folder path (required)")
	aiJobSessionCreateCmd.Flags().String("workspace", "", "workspace folder path (required)")
	aiJobSessionCreateCmd.Flags().String("name", "", "session name (required)")
	aiJobSessionUseCmd.Flags().String("workspace", "", "workspace folder path (required)")
}

var extractAICmd = &cobra.Command{
	Use:   "extract-ai",
	Short: "Extract structured data from a web page using an LLM",
	Long: `Navigate to a URL, convert the page to Markdown, and send it to an LLM
for intelligent extraction. Optionally review the output with a second LLM.

Supported providers: ollama, openai, anthropic, openrouter, deepseek, gemini.
Any OpenAI-compatible endpoint works via --api-base.

With --review, a second LLM validates the extraction for accuracy.
With --workspace, jobs are persisted to disk with full metadata.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")
		if urlFlag == "" {
			return fmt.Errorf("scout: --url is required")
		}

		prompt, _ := cmd.Flags().GetString("prompt")
		if prompt == "" {
			return fmt.Errorf("scout: --prompt is required")
		}

		providerName, _ := cmd.Flags().GetString("provider")
		modelFlag, _ := cmd.Flags().GetString("model")
		apiKey, _ := cmd.Flags().GetString("api-key")
		apiBase, _ := cmd.Flags().GetString("api-base")
		schemaFile, _ := cmd.Flags().GetString("schema")
		mainOnly, _ := cmd.Flags().GetBool("main-only")
		systemPrompt, _ := cmd.Flags().GetString("system-prompt")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		ollamaHost, _ := cmd.Flags().GetString("ollama-host")
		review, _ := cmd.Flags().GetBool("review")
		reviewProviderName, _ := cmd.Flags().GetString("review-provider")
		reviewModel, _ := cmd.Flags().GetString("review-model")
		reviewAPIKey, _ := cmd.Flags().GetString("review-api-key")
		reviewAPIBase, _ := cmd.Flags().GetString("review-api-base")
		reviewPrompt, _ := cmd.Flags().GetString("review-prompt")
		workspacePath, _ := cmd.Flags().GetString("workspace")
		sessionID, _ := cmd.Flags().GetString("session-id")
		metaSlice, _ := cmd.Flags().GetStringSlice("meta")

		provider, err := createProviderFull(providerName, modelFlag, apiKey, apiBase, ollamaHost)
		if err != nil {
			return fmt.Errorf("scout: create extract provider: %w", err)
		}

		var llmOpts []scout.LLMOption
		llmOpts = append(llmOpts, scout.WithLLMProvider(provider))
		if modelFlag != "" {
			llmOpts = append(llmOpts, scout.WithLLMModel(modelFlag))
		}
		if mainOnly {
			llmOpts = append(llmOpts, scout.WithLLMMainContent())
		}
		if systemPrompt != "" {
			llmOpts = append(llmOpts, scout.WithLLMSystemPrompt(systemPrompt))
		}
		llmOpts = append(llmOpts, scout.WithLLMTimeout(timeout))

		if schemaFile != "" {
			data, err := os.ReadFile(schemaFile)
			if err != nil {
				return fmt.Errorf("scout: read schema: %w", err)
			}
			llmOpts = append(llmOpts, scout.WithLLMSchema(json.RawMessage(data)))
		}

		// Review provider
		if review {
			if reviewProviderName == "" {
				reviewProviderName = providerName
			}
			rKey := reviewAPIKey
			if rKey == "" {
				rKey = apiKey
			}
			rBase := reviewAPIBase
			if rBase == "" {
				rBase = apiBase
			}
			reviewProv, err := createProviderFull(reviewProviderName, reviewModel, rKey, rBase, ollamaHost)
			if err != nil {
				return fmt.Errorf("scout: create review provider: %w", err)
			}
			llmOpts = append(llmOpts, scout.WithLLMReview(reviewProv))
			if reviewModel != "" {
				llmOpts = append(llmOpts, scout.WithLLMReviewModel(reviewModel))
			}
			if reviewPrompt != "" {
				llmOpts = append(llmOpts, scout.WithLLMReviewPrompt(reviewPrompt))
			}
		}

		// Workspace
		if workspacePath != "" {
			ws, err := scout.NewLLMWorkspace(workspacePath)
			if err != nil {
				return fmt.Errorf("scout: open workspace: %w", err)
			}
			llmOpts = append(llmOpts, scout.WithLLMWorkspace(ws))
			if sessionID != "" {
				llmOpts = append(llmOpts, scout.WithLLMSessionID(sessionID))
			}
		}

		// Metadata
		for _, kv := range metaSlice {
			k, v := splitMeta(kv)
			llmOpts = append(llmOpts, scout.WithLLMMetadata(k, v))
		}

		// Launch browser
		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(urlFlag)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		// Use review pipeline if --review is set or workspace is set
		if review || workspacePath != "" {
			result, err := page.ExtractWithLLMReview(prompt, llmOpts...)
			if err != nil {
				return err
			}

			return outputJobResult(cmd, result)
		}

		// Simple extraction (no review, no workspace)
		result, err := page.ExtractWithLLM(prompt, llmOpts...)
		if err != nil {
			return err
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile != "" {
			dest, err := writeOutput(cmd, []byte(result), "extract-ai.txt")
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Written to %s\n", dest)
			return nil
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), result)
		return nil
	},
}

func outputJobResult(cmd *cobra.Command, result *scout.LLMJobResult) error {
	w := cmd.OutOrStdout()

	if result.JobID != "" {
		_, _ = fmt.Fprintf(w, "Job: %s\n\n", result.JobID)
	}

	_, _ = fmt.Fprintf(w, "--- Extraction ---\n%s\n", result.ExtractResult)

	if result.Reviewed {
		_, _ = fmt.Fprintf(w, "\n--- Review ---\n%s\n", result.ReviewResult)
	}

	outFile, _ := cmd.Flags().GetString("output")
	if outFile != "" {
		data, _ := json.MarshalIndent(result, "", "  ")
		dest, err := writeOutput(cmd, data, "extract-ai.json")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "\nWritten to %s\n", dest)
	}

	return nil
}

// --- Ollama commands (unchanged) ---

var ollamaCmd = &cobra.Command{
	Use:   "ollama",
	Short: "Manage Ollama models and server",
}

var ollamaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List locally available Ollama models",
	RunE: func(cmd *cobra.Command, _ []string) error {
		provider, err := newOllamaFromFlags(cmd)
		if err != nil {
			return err
		}

		models, err := provider.ListModels(context.Background())
		if err != nil {
			return err
		}

		if len(models) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No models found. Use 'scout ollama pull <model>' to download one.")
			return nil
		}

		for _, m := range models {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), m)
		}

		return nil
	},
}

var ollamaPullCmd = &cobra.Command{
	Use:   "pull <model>",
	Short: "Download an Ollama model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := newOllamaFromFlags(cmd)
		if err != nil {
			return err
		}

		model := args[0]
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pulling %s...\n", model)

		err = provider.PullModel(context.Background(), model, func(status string, completed, total int64) {
			if total > 0 {
				pct := float64(completed) / float64(total) * 100
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\r%s: %.1f%%", status, pct)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\r%s", status)
			}
		})
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nDone.\n")
		return nil
	},
}

var ollamaStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Ollama server connection",
	RunE: func(cmd *cobra.Command, _ []string) error {
		provider, err := newOllamaFromFlags(cmd)
		if err != nil {
			return err
		}

		models, err := provider.ListModels(context.Background())
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Connection failed: %v\n", err)
			return nil
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Connected. %d model(s) available.\n", len(models))
		return nil
	},
}

// --- AI Job commands ---

var aiJobCmd = &cobra.Command{
	Use:   "ai-job",
	Short: "Manage AI extraction jobs and sessions",
	Long:  `View, track, and manage LLM extraction jobs persisted in a workspace folder.`,
}

var aiJobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all jobs in the workspace",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ws, err := workspaceFromFlags(cmd)
		if err != nil {
			return err
		}

		sessionID, _ := cmd.Flags().GetString("session-id")

		var refs []scout.JobRef
		if sessionID != "" {
			refs, err = ws.ListSessionJobs(sessionID)
		} else {
			refs, err = ws.ListJobs()
		}
		if err != nil {
			return err
		}

		if len(refs) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No jobs found.")
			return nil
		}

		for _, r := range refs {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %-12s  %s  %s\n",
				r.ID[:8], r.Status, r.CreatedAt.Format("2006-01-02 15:04"), truncate(r.URL, 50))
		}

		return nil
	},
}

var aiJobShowCmd = &cobra.Command{
	Use:   "show <job-id>",
	Short: "Show details of a specific job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspaceFromFlags(cmd)
		if err != nil {
			return err
		}

		job, err := ws.GetJob(args[0])
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(job, "", "  ")
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	},
}

var aiJobSessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage workspace sessions",
}

var aiJobSessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ws, err := workspaceFromFlags(cmd)
		if err != nil {
			return err
		}

		sessions, err := ws.ListSessions()
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
			return nil
		}

		cur, _ := ws.CurrentSession()
		for _, s := range sessions {
			marker := "  "
			if cur != nil && cur.ID == s.ID {
				marker = "* "
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s  %s  %s\n",
				marker, s.ID[:8], s.Name, s.CreatedAt.Format("2006-01-02 15:04"))
		}

		return nil
	},
}

var aiJobSessionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ws, err := workspaceFromFlags(cmd)
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("scout: --name is required")
		}

		sess, err := ws.CreateSession(name, nil)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created session %s (%s)\n", sess.ID[:8], sess.Name)
		return nil
	},
}

var aiJobSessionUseCmd = &cobra.Command{
	Use:   "use <session-id>",
	Short: "Set the current active session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspaceFromFlags(cmd)
		if err != nil {
			return err
		}

		// Support partial ID matching
		sessions, err := ws.ListSessions()
		if err != nil {
			return err
		}

		var matchID string
		for _, s := range sessions {
			if s.ID == args[0] || (len(args[0]) >= 4 && s.ID[:len(args[0])] == args[0]) {
				matchID = s.ID
				break
			}
		}

		if matchID == "" {
			return fmt.Errorf("scout: session %q not found", args[0])
		}

		if err := ws.SetCurrentSession(matchID); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Switched to session %s\n", matchID[:8])
		return nil
	},
}

// --- provider creation ---

func createProviderFull(name, model, apiKey, apiBase, ollamaHost string) (scout.LLMProvider, error) {
	// Resolve API key from environment if not provided
	if apiKey == "" {
		apiKey = resolveAPIKey(name)
	}

	switch name {
	case "ollama":
		var opts []scout.OllamaOption
		if ollamaHost != "" {
			opts = append(opts, scout.WithOllamaHost(ollamaHost))
		}
		if model != "" {
			opts = append(opts, scout.WithOllamaModel(model))
		}
		return scout.NewOllamaProvider(opts...)

	case "openai":
		opts := []scout.OpenAIOption{scout.WithOpenAIKey(apiKey)}
		if model != "" {
			opts = append(opts, scout.WithOpenAIModel(model))
		}
		if apiBase != "" {
			opts = append(opts, scout.WithOpenAIBaseURL(apiBase))
		}
		return scout.NewOpenAIProvider(opts...)

	case "anthropic":
		opts := []scout.AnthropicOption{scout.WithAnthropicKey(apiKey)}
		if model != "" {
			opts = append(opts, scout.WithAnthropicModel(model))
		}
		if apiBase != "" {
			opts = append(opts, scout.WithAnthropicBaseURL(apiBase))
		}
		return scout.NewAnthropicProvider(opts...)

	case "openrouter":
		var extra []scout.OpenAIOption
		if apiBase != "" {
			extra = append(extra, scout.WithOpenAIBaseURL(apiBase))
		}
		return scout.NewOpenRouterProvider(apiKey, model, extra...)

	case "deepseek":
		var extra []scout.OpenAIOption
		if apiBase != "" {
			extra = append(extra, scout.WithOpenAIBaseURL(apiBase))
		}
		return scout.NewDeepSeekProvider(apiKey, model, extra...)

	case "gemini":
		var extra []scout.OpenAIOption
		if apiBase != "" {
			extra = append(extra, scout.WithOpenAIBaseURL(apiBase))
		}
		return scout.NewGeminiProvider(apiKey, model, extra...)

	default:
		// Treat unknown providers as OpenAI-compatible with custom base URL
		if apiBase != "" {
			opts := []scout.OpenAIOption{
				scout.WithOpenAIKey(apiKey),
				scout.WithOpenAIBaseURL(apiBase),
			}
			if model != "" {
				opts = append(opts, scout.WithOpenAIModel(model))
			}
			return scout.NewOpenAIProvider(opts...)
		}
		return nil, fmt.Errorf("scout: unknown LLM provider %q (use --api-base for custom endpoints)", name)
	}
}

// createProvider kept for backward compatibility.
func createProvider(name, model, host string) (scout.LLMProvider, error) {
	return createProviderFull(name, model, "", "", host)
}

func resolveAPIKey(provider string) string {
	envVars := map[string][]string{
		"openai":     {"OPENAI_API_KEY"},
		"anthropic":  {"ANTHROPIC_API_KEY"},
		"openrouter": {"OPENROUTER_API_KEY"},
		"deepseek":   {"DEEPSEEK_API_KEY"},
		"gemini":     {"GEMINI_API_KEY", "GOOGLE_API_KEY"},
	}

	vars, ok := envVars[provider]
	if !ok {
		return ""
	}

	for _, v := range vars {
		if val := os.Getenv(v); val != "" {
			return val
		}
	}

	return ""
}

func newOllamaFromFlags(cmd *cobra.Command) (*scout.OllamaProvider, error) {
	host, _ := cmd.Flags().GetString("ollama-host")
	var opts []scout.OllamaOption
	if host != "" {
		opts = append(opts, scout.WithOllamaHost(host))
	}
	return scout.NewOllamaProvider(opts...)
}

func workspaceFromFlags(cmd *cobra.Command) (*scout.LLMWorkspace, error) {
	path, _ := cmd.Flags().GetString("workspace")
	if path == "" {
		return nil, fmt.Errorf("scout: --workspace is required")
	}
	return scout.NewLLMWorkspace(path)
}

func splitMeta(kv string) (string, string) {
	for i, c := range kv {
		if c == '=' {
			return kv[:i], kv[i+1:]
		}
	}
	return kv, ""
}
