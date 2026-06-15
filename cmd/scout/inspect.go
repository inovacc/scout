package main

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(titleCmd, urlCmd, textCmd, attrCmd, evalCmd, htmlCmd)

	htmlCmd.Flags().String("selector", "", "CSS selector (defaults to entire page)")
}

// openPage launches a single-shot browser and navigates to url, waiting for
// the page to finish loading. The caller is responsible for closing the
// returned browser.
func openPage(cmd *cobra.Command, url string) (*scout.Browser, *scout.Page, error) {
	b, err := scout.New(baseOpts(cmd)...)
	if err != nil {
		return nil, nil, fmt.Errorf("scout: launch browser: %w", err)
	}

	page, err := b.NewPage(url)
	if err != nil {
		_ = b.Close()
		return nil, nil, fmt.Errorf("scout: navigate: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = b.Close()
		return nil, nil, fmt.Errorf("scout: wait load: %w", err)
	}

	return b, page, nil
}

var titleCmd = &cobra.Command{
	Use:   "title <url>",
	Short: "Get the page title",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, page, err := openPage(cmd, args[0])
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		title, err := page.Title()
		if err != nil {
			return fmt.Errorf("scout: title: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), title)

		return nil
	},
}

var urlCmd = &cobra.Command{
	Use:   "url <url>",
	Short: "Get the current page URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, page, err := openPage(cmd, args[0])
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		current, err := page.URL()
		if err != nil {
			return fmt.Errorf("scout: url: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), current)

		return nil
	},
}

var textCmd = &cobra.Command{
	Use:   "text <url> <selector>",
	Short: "Get text content of an element",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, page, err := openPage(cmd, args[0])
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		el, err := page.Element(args[1])
		if err != nil {
			return fmt.Errorf("scout: text: %w", err)
		}

		text, err := el.Text()
		if err != nil {
			return fmt.Errorf("scout: text: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), text)

		return nil
	},
}

var attrCmd = &cobra.Command{
	Use:   "attr <url> <selector> <attribute>",
	Short: "Get an attribute value from an element",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, page, err := openPage(cmd, args[0])
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		el, err := page.Element(args[1])
		if err != nil {
			return fmt.Errorf("scout: attr: %w", err)
		}

		value, _, err := el.Attribute(args[2])
		if err != nil {
			return fmt.Errorf("scout: attr: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)

		return nil
	},
}

var evalCmd = &cobra.Command{
	Use:   "eval <url> <javascript>",
	Short: "Evaluate JavaScript in the page context",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, page, err := openPage(cmd, args[0])
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		result, err := page.Eval(args[1])
		if err != nil {
			return fmt.Errorf("scout: eval: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.String())

		return nil
	},
}

var htmlCmd = &cobra.Command{
	Use:   "html <url>",
	Short: "Get HTML content of the page or an element",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, page, err := openPage(cmd, args[0])
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		selector, _ := cmd.Flags().GetString("selector")

		if selector == "" {
			html, err := page.HTML()
			if err != nil {
				return fmt.Errorf("scout: html: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), html)

			return nil
		}

		el, err := page.Element(selector)
		if err != nil {
			return fmt.Errorf("scout: html: %w", err)
		}

		html, err := el.HTML()
		if err != nil {
			return fmt.Errorf("scout: html: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), html)

		return nil
	},
}
