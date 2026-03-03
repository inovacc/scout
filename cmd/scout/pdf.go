package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(pdfFormCmd)
	pdfFormCmd.AddCommand(pdfFormFillCmd)
	pdfFormCmd.AddCommand(pdfFormFieldsCmd)

	pdfFormFillCmd.Flags().StringP("file", "f", "", "path to PDF file")
	pdfFormFillCmd.Flags().StringSliceP("field", "F", nil, "field=value pairs (repeatable)")
	pdfFormFillCmd.Flags().StringP("output", "o", "", "output path for filled PDF")

	pdfFormFieldsCmd.Flags().StringP("file", "f", "", "path to PDF file")
	pdfFormFieldsCmd.Flags().Bool("json", false, "output as JSON")
}

var pdfFormCmd = &cobra.Command{
	Use:   "pdf-form",
	Short: "PDF form operations (fill fields, detect fields)",
}

var pdfFormFieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "Detect fillable fields in a PDF form",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		jsonOut, _ := cmd.Flags().GetBool("json")

		if file == "" {
			return fmt.Errorf("--file is required")
		}

		opts := baseOpts(cmd)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("create browser: %w", err)
		}

		defer func() { _ = b.Close() }()

		absPath, err := resolvePDFPath(file)
		if err != nil {
			return err
		}

		page, err := b.NewPage("file://" + absPath)
		if err != nil {
			return fmt.Errorf("scout: pdf fields: open page: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: pdf fields: wait load: %w", err)
		}

		fields, err := page.PDFFormFields()
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(fields)
		}

		if len(fields) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No fillable fields found.")

			return nil
		}

		for _, f := range fields {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-20s  type=%-10s  value=%q  page=%d\n",
				f.Name, f.Type, f.Value, f.Page)
		}

		return nil
	},
}

var pdfFormFillCmd = &cobra.Command{
	Use:   "fill",
	Short: "Fill interactive PDF form fields",
	Long: `Fill interactive PDF form fields and export the result.

Example:
  scout pdf-form fill --file=form.pdf --field=name:John --field=email:john@example.com --output=filled.pdf`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		file, _ := cmd.Flags().GetString("file")
		fieldPairs, _ := cmd.Flags().GetStringSlice("field")
		output, _ := cmd.Flags().GetString("output")

		if file == "" {
			return fmt.Errorf("--file is required")
		}

		if len(fieldPairs) == 0 {
			return fmt.Errorf("at least one --field is required")
		}

		fields := make(map[string]string)

		for _, pair := range fieldPairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid field format %q (expected name:value)", pair)
			}

			fields[parts[0]] = parts[1]
		}

		opts := baseOpts(cmd)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("create browser: %w", err)
		}

		defer func() { _ = b.Close() }()

		absPath, err := resolvePDFPath(file)
		if err != nil {
			return err
		}

		page, err := b.NewPage("file://" + absPath)
		if err != nil {
			return fmt.Errorf("scout: pdf fill: open page: %w", err)
		}

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: pdf fill: wait load: %w", err)
		}

		if err := page.FillPDFForm(fields); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Filled %d field(s)\n", len(fields))

		if output != "" {
			pdfData, err := page.PDF()
			if err != nil {
				return fmt.Errorf("scout: pdf fill: export: %w", err)
			}

			if err := os.WriteFile(output, pdfData, 0o644); err != nil {
				return fmt.Errorf("scout: pdf fill: write output: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Saved filled PDF: %s\n", output)
		}

		return nil
	},
}

func resolvePDFPath(file string) (string, error) {
	absPath, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("scout: resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("scout: file not found: %s", absPath)
	}

	// Convert backslashes to forward slashes for file:// URLs.
	return filepath.ToSlash(absPath), nil
}
