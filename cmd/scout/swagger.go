package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(swaggerCmd)

	swaggerCmd.Flags().Bool("endpoints-only", false, "only list endpoints, skip schemas")
	swaggerCmd.Flags().Bool("raw", false, "output raw spec JSON as-is")
}

var swaggerCmd = &cobra.Command{
	Use:   "swagger <url>",
	Short: "Detect and extract Swagger/OpenAPI spec from a page",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]
		endpointsOnly, _ := cmd.Flags().GetBool("endpoints-only")
		rawFlag, _ := cmd.Flags().GetBool("raw")

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		spec, err := browser.ExtractSwagger(targetURL,
			scout.WithSwaggerEndpointsOnly(endpointsOnly),
			scout.WithSwaggerRaw(rawFlag),
		)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		output, _ := cmd.Flags().GetString("output")

		// Raw mode: output the raw spec JSON
		if rawFlag && spec.Raw != nil {
			if output != "" {
				if _, err := writeOutput(cmd, spec.Raw, output); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved raw spec to %s\n", output)
				return nil
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(spec.Raw))
			return nil
		}

		// JSON format
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if output != "" {
				data, err := json.MarshalIndent(spec, "", "  ")
				if err != nil {
					return fmt.Errorf("scout: marshal: %w", err)
				}
				if _, err := writeOutput(cmd, data, output); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved spec to %s\n", output)
				return nil
			}
			return enc.Encode(spec)
		}

		// Text summary
		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "API:      %s\n", spec.Info.Title)
		_, _ = fmt.Fprintf(w, "Version:  %s (OpenAPI %s)\n", spec.Info.Version, spec.Version)
		if spec.Info.Description != "" {
			_, _ = fmt.Fprintf(w, "Desc:     %s\n", truncate(spec.Info.Description, 80))
		}
		if spec.SpecURL != "" {
			_, _ = fmt.Fprintf(w, "Spec URL: %s\n", spec.SpecURL)
		}
		for _, s := range spec.Servers {
			_, _ = fmt.Fprintf(w, "Server:   %s", s.URL)
			if s.Description != "" {
				_, _ = fmt.Fprintf(w, " (%s)", s.Description)
			}
			_, _ = fmt.Fprintln(w)
		}

		if len(spec.Security) > 0 {
			_, _ = fmt.Fprintln(w, "\nSecurity:")
			for _, sec := range spec.Security {
				_, _ = fmt.Fprintf(w, "  %s: type=%s", sec.Name, sec.Type)
				if sec.In != "" {
					_, _ = fmt.Fprintf(w, " in=%s", sec.In)
				}
				if sec.Scheme != "" {
					_, _ = fmt.Fprintf(w, " scheme=%s", sec.Scheme)
				}
				_, _ = fmt.Fprintln(w)
			}
		}

		if len(spec.Paths) > 0 {
			_, _ = fmt.Fprintf(w, "\nEndpoints (%d):\n", len(spec.Paths))
			for _, p := range spec.Paths {
				_, _ = fmt.Fprintf(w, "  %-7s %s", p.Method, p.Path)
				if p.Summary != "" {
					_, _ = fmt.Fprintf(w, "  — %s", p.Summary)
				}
				if len(p.Tags) > 0 {
					_, _ = fmt.Fprintf(w, "  [%s]", strings.Join(p.Tags, ", "))
				}
				_, _ = fmt.Fprintln(w)
			}
		}

		if len(spec.Schemas) > 0 {
			_, _ = fmt.Fprintf(w, "\nSchemas (%d):\n", len(spec.Schemas))
			for name := range spec.Schemas {
				_, _ = fmt.Fprintf(w, "  %s\n", name)
			}
		}

		// Save to file if requested
		if output != "" {
			data, err := json.MarshalIndent(spec, "", "  ")
			if err != nil {
				return fmt.Errorf("scout: marshal: %w", err)
			}
			if _, err := writeOutput(cmd, data, output); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(w, "\nSaved to %s\n", output)
		}

		return nil
	},
}
