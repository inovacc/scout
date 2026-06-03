package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/flow"
	"github.com/inovacc/scout/pkg/scout/vault"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var flowCmd = &cobra.Command{Use: "flow", Short: "Capture a browser flow, analyze it to a spec, and replay it without a browser"}

var flowCaptureCmd = &cobra.Command{
	Use:   "capture <url>",
	Short: "Record a flow's network traffic into a capture.json",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		filter, _ := cmd.Flags().GetStringSlice("filter")
		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()
		page, err := b.NewPage("about:blank")
		if err != nil {
			return err
		}
		capt, err := flow.CaptureFlow(cmd.Context(), page, flow.CaptureOptions{URL: args[0], Name: "capture", URLFilter: filter})
		if err != nil {
			return err
		}
		if out == "" {
			out = "capture.json"
		}
		if err := flow.SaveCapture(out, capt); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "captured %d entries -> %s\n", len(capt.Entries), out)
		return nil
	},
}

var flowAnalyzeCmd = &cobra.Command{
	Use:   "analyze <capture.json>",
	Short: "Turn a capture into a reviewed flow.yaml + report (LLM)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		name, _ := cmd.Flags().GetString("name")
		capt, err := flow.LoadCapture(args[0])
		if err != nil {
			return err
		}
		provider, err := flowProvider(cmd)
		if err != nil {
			return err
		}
		spec, report, err := flow.Analyze(capt, provider, flow.AnalyzeOptions{Name: name})
		if err != nil {
			return err
		}
		if out == "" {
			out = "flow.yaml"
		}
		if err := writeFlowSpec(out, spec); err != nil {
			return err
		}
		_, _ = fmt.Fprint(cmd.ErrOrStderr(), renderFlowReport(report))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (review it, then: scout flow run %s)\n", out, out)
		return nil
	},
}

var flowRunCmd = &cobra.Command{
	Use:   "run <flow.yaml>",
	Short: "Replay a flow deterministically (no browser)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile, _ := cmd.Flags().GetString("profile")
		f, err := flow.LoadFile(args[0])
		if err != nil {
			return err
		}
		if err := flow.Validate(f); err != nil {
			return err
		}
		opts := flow.RunOptions{}
		if profile == "" && f.Auth != nil {
			profile = f.Auth.Profile
		}
		if profile != "" {
			pass, perr := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
			if perr != nil {
				return perr
			}
			defer zeroBytesCLI(pass)
			v, verr := vault.Open(pass)
			if verr != nil {
				return verr
			}
			defer func() { _ = v.Close() }()
			h, herr := v.Use(profile)
			if herr != nil {
				return herr
			}
			defer func() { _ = h.Close() }()
			opts.Secrets = flow.NewVaultResolver(h)
		}
		res, err := flow.Run(cmd.Context(), f, opts)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ran %d steps\n", len(res.Steps))
		return nil
	},
}

var flowVerifyCmd = &cobra.Command{
	Use:   "verify <flow.yaml> --golden <capture.json>",
	Short: "Re-run a flow and diff status against the golden capture",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		goldenPath, _ := cmd.Flags().GetString("golden")
		if goldenPath == "" {
			return fmt.Errorf("scout: flow: --golden <capture.json> is required")
		}
		f, err := flow.LoadFile(args[0])
		if err != nil {
			return err
		}
		golden, err := flow.LoadCapture(goldenPath)
		if err != nil {
			return err
		}
		rep, err := flow.Verify(cmd.Context(), f, golden, flow.RunOptions{})
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), renderVerifyReport(rep))
		if !rep.OK {
			return fmt.Errorf("scout: flow: verify: parity drift detected")
		}
		return nil
	},
}

// renderFlowReport formats the analysis report. It MUST NOT print secret values
// (Report carries none — only indexes, var names, paths, confidence).
func renderFlowReport(r *flow.Report) string {
	var sb strings.Builder
	if r.Degraded {
		sb.WriteString("[degraded] LLM analysis failed — review EVERY step.\n")
	}
	if len(r.Dropped) > 0 {
		_, _ = fmt.Fprintf(&sb, "dropped entries: %v\n", r.Dropped)
	}
	for _, c := range r.Chains {
		_, _ = fmt.Fprintf(&sb, "chain: %s %s -> ${%s} into %s/%s (conf %.2g)\n", c.From, c.Path, c.Var, c.Into, c.Name, c.Confidence)
	}
	for _, n := range r.Notes {
		_, _ = fmt.Fprintf(&sb, "note: %s\n", n)
	}
	return sb.String()
}

// renderVerifyReport formats a parity report as one line per step.
func renderVerifyReport(r *flow.VerifyReport) string {
	var sb strings.Builder
	for _, s := range r.Steps {
		status := "OK"
		if !s.StatusMatch {
			status = "DRIFT"
		}
		_, _ = fmt.Fprintf(&sb, "%-6s %s: expected %d got %d\n", status, s.ID, s.ExpectedStatus, s.ActualStatus)
	}
	return sb.String()
}

// writeFlowSpec marshals a FlowSpec to YAML and writes it 0o600. A generated spec
// should be secret-free (only ${secret.*} refs + auth.profile), but the analyzer
// can still copy raw auth headers/URLs from a capture, so we restrict perms
// defensively until the hygiene guard parameterizes them.
func writeFlowSpec(path string, f *flow.FlowSpec) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("scout: flow: write spec: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: flow: write spec: %w", err)
	}
	return nil
}

// flowProvider builds the LLM provider for `flow analyze` from --llm, falling back
// to $SCOUT_LLM_PROVIDER then "ollama" (which constructs without a key; Analyze
// degrades safely if it is unreachable at run time).
func flowProvider(cmd *cobra.Command) (scout.LLMProvider, error) {
	name, _ := cmd.Flags().GetString("llm")
	if name == "" {
		name = os.Getenv("SCOUT_LLM_PROVIDER")
	}
	if name == "" {
		name = "ollama"
	}
	return createProviderFull(name, "", "", "", "")
}

func init() {
	flowCaptureCmd.Flags().StringP("out", "o", "", "output capture path (default capture.json)")
	flowCaptureCmd.Flags().StringSlice("filter", []string{"*api*", "*graphql*"}, "URL glob filters to keep")
	flowAnalyzeCmd.Flags().StringP("out", "o", "", "output flow spec path (default flow.yaml)")
	flowAnalyzeCmd.Flags().String("name", "captured-flow", "name for the generated flow")
	flowAnalyzeCmd.Flags().String("llm", "", "LLM provider (ollama|openai|anthropic; default from env or ollama)")
	flowRunCmd.Flags().String("profile", "", "vault profile id for auth (default: spec auth.profile)")
	flowVerifyCmd.Flags().String("golden", "", "golden capture.json to diff against")
	flowCmd.AddCommand(flowCaptureCmd, flowAnalyzeCmd, flowRunCmd, flowVerifyCmd)
	rootCmd.AddCommand(flowCmd)
}
