package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(formCmd)
	formCmd.AddCommand(formDetectCmd, formFillCmd, formSubmitCmd)

	formDetectCmd.Flags().String("url", "", "URL to navigate to")
	formDetectCmd.Flags().String("selector", "", "CSS selector to narrow to a specific form")

	formFillCmd.Flags().String("url", "", "URL to navigate to")
	formFillCmd.Flags().String("selector", "", "CSS selector for the form")
	formFillCmd.Flags().Bool("submit", false, "submit after filling")

	formSubmitCmd.Flags().String("url", "", "URL to navigate to")
	formSubmitCmd.Flags().String("selector", "", "CSS selector for the form")
}

var formCmd = &cobra.Command{
	Use:   "form",
	Short: "Detect, fill, and submit forms",
}

var formDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect forms on a page",
	RunE: func(cmd *cobra.Command, _ []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")
		if urlFlag == "" {
			return fmt.Errorf("scout: --url is required")
		}

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

		selector, _ := cmd.Flags().GetString("selector")
		if selector != "" {
			form, err := page.DetectForm(selector)
			if err != nil {
				return fmt.Errorf("scout: detect form: %w", err)
			}
			return outputForm(cmd, form)
		}

		forms, err := page.DetectForms()
		if err != nil {
			return fmt.Errorf("scout: detect forms: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(forms) //nolint:musttag
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d form(s)\n\n", len(forms))
		for i, f := range forms {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Form %d: action=%s method=%s\n", i+1, f.Action, f.Method)
			for _, field := range f.Fields {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-20s type=%-10s value=%q\n", field.Name, field.Type, field.Value)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}

		return nil
	},
}

var formFillCmd = &cobra.Command{
	Use:   "fill <json-data>",
	Short: "Fill a form with JSON data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")
		if urlFlag == "" {
			return fmt.Errorf("scout: --url is required")
		}

		var data map[string]string
		if err := json.Unmarshal([]byte(args[0]), &data); err != nil {
			return fmt.Errorf("scout: invalid JSON: %w", err)
		}

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

		selector, _ := cmd.Flags().GetString("selector")
		if selector == "" {
			selector = "form"
		}

		form, err := page.DetectForm(selector)
		if err != nil {
			return fmt.Errorf("scout: detect form: %w", err)
		}

		if err := form.Fill(data); err != nil {
			return fmt.Errorf("scout: fill form: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "form filled")

		submitFlag, _ := cmd.Flags().GetBool("submit")
		if submitFlag {
			if err := form.Submit(); err != nil {
				return fmt.Errorf("scout: submit form: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "form submitted")
		}

		return nil
	},
}

var formSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a form",
	RunE: func(cmd *cobra.Command, _ []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")
		if urlFlag == "" {
			return fmt.Errorf("scout: --url is required")
		}

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

		selector, _ := cmd.Flags().GetString("selector")
		if selector == "" {
			selector = "form"
		}

		form, err := page.DetectForm(selector)
		if err != nil {
			return fmt.Errorf("scout: detect form: %w", err)
		}

		if err := form.Submit(); err != nil {
			return fmt.Errorf("scout: submit form: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "form submitted")
		return nil
	},
}

func outputForm(cmd *cobra.Command, form *scout.Form) error {
	format, _ := cmd.Flags().GetString("format")
	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")

		return enc.Encode(form) //nolint:musttag
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Form: action=%s method=%s\n", form.Action, form.Method)
	for _, field := range form.Fields {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-20s type=%-10s value=%q\n", field.Name, field.Type, field.Value)
	}

	return nil
}
