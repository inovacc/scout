package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var detectCmd = &cobra.Command{
	Use:   "detect <url>",
	Short: "Detect page intelligence: frameworks, PWA, render mode, tech stack",
	Long: `Analyze a web page and report detected technologies.

By default runs all detectors. Use flags to select specific checks.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		page, err := b.NewPage(args[0])
		if err != nil {
			return err
		}

		defer func() { _ = page.Close() }()

		if err := page.WaitLoad(); err != nil {
			return err
		}

		fwOnly, _ := cmd.Flags().GetBool("framework")
		pwaOnly, _ := cmd.Flags().GetBool("pwa")
		techOnly, _ := cmd.Flags().GetBool("tech")
		renderOnly, _ := cmd.Flags().GetBool("render")
		format, _ := cmd.Flags().GetString("format")
		all := !fwOnly && !pwaOnly && !techOnly && !renderOnly

		w := cmd.OutOrStdout()

		// Collect results for JSON mode.
		type detectResult struct {
			URL        string                `json:"url"`
			Frameworks []scout.FrameworkInfo `json:"frameworks,omitempty"`
			PWA        *scout.PWAInfo        `json:"pwa,omitempty"`
			TechStack  *scout.TechStack      `json:"tech_stack,omitempty"`
			Render     *scout.RenderInfo     `json:"render,omitempty"`
		}

		result := detectResult{URL: args[0]}

		// Framework detection
		if all || fwOnly {
			frameworks, err := page.DetectFrameworks()
			if err != nil {
				return err
			}

			result.Frameworks = frameworks
			if format != "json" {
				if len(frameworks) == 0 {
					_, _ = fmt.Fprintln(w, "Frameworks:      none detected")
				} else {
					_, _ = fmt.Fprintln(w, "Frameworks:")

					for _, f := range frameworks {
						ver := f.Version
						if ver == "" {
							ver = "-"
						}

						spa := ""
						if f.SPA {
							spa = " (SPA)"
						}

						_, _ = fmt.Fprintf(w, "  %-15s  version=%-10s%s\n", f.Name, ver, spa)
					}
				}
			}
		}

		// PWA detection
		if all || pwaOnly {
			pwa, err := page.DetectPWA()
			if err != nil {
				return err
			}

			result.PWA = pwa

			if format != "json" {
				_, _ = fmt.Fprintln(w, "PWA:")
				_, _ = fmt.Fprintf(w, "  Service Worker: %v\n", pwa.HasServiceWorker)
				_, _ = fmt.Fprintf(w, "  Manifest:       %v\n", pwa.HasManifest)
				_, _ = fmt.Fprintf(w, "  Installable:    %v\n", pwa.Installable)
				_, _ = fmt.Fprintf(w, "  HTTPS:          %v\n", pwa.HTTPS)

				_, _ = fmt.Fprintf(w, "  Push Capable:   %v\n", pwa.PushCapable)
				if pwa.Manifest != nil {
					_, _ = fmt.Fprintf(w, "  App Name:       %s\n", pwa.Manifest.Name)
					_, _ = fmt.Fprintf(w, "  Display:        %s\n", pwa.Manifest.Display)
					_, _ = fmt.Fprintf(w, "  Icons:          %d\n", pwa.Manifest.Icons)
				}
			}
		}

		// Render mode detection
		if all || renderOnly {
			render, err := page.DetectRenderMode()
			if err != nil {
				return err
			}

			result.Render = render

			if format != "json" {
				_, _ = fmt.Fprintln(w, "Rendering:")
				_, _ = fmt.Fprintf(w, "  Mode:           %s\n", render.Mode)

				_, _ = fmt.Fprintf(w, "  Hydrated:       %v\n", render.Hydrated)
				if render.Details != "" {
					_, _ = fmt.Fprintf(w, "  Details:        %s\n", render.Details)
				}
			}
		}

		// Tech stack detection
		if all || techOnly {
			tech, err := page.DetectTechStack()
			if err != nil {
				return err
			}

			result.TechStack = tech

			if format != "json" {
				_, _ = fmt.Fprintln(w, "Tech Stack:")
				if tech.CSSFramework != "" {
					_, _ = fmt.Fprintf(w, "  CSS Framework:  %s\n", tech.CSSFramework)
				}

				if tech.BuildTool != "" {
					_, _ = fmt.Fprintf(w, "  Build Tool:     %s\n", tech.BuildTool)
				}

				if tech.CMS != "" {
					_, _ = fmt.Fprintf(w, "  CMS:            %s\n", tech.CMS)
				}

				if len(tech.Analytics) > 0 {
					_, _ = fmt.Fprintf(w, "  Analytics:      %v\n", tech.Analytics)
				}

				if tech.CDN != "" {
					_, _ = fmt.Fprintf(w, "  CDN:            %s\n", tech.CDN)
				}

				if tech.CSSFramework == "" && tech.BuildTool == "" && tech.CMS == "" && len(tech.Analytics) == 0 && tech.CDN == "" {
					_, _ = fmt.Fprintln(w, "  (none detected)")
				}
			}
		}

		if format == "json" {
			data, _ := json.MarshalIndent(result, "", "  ") //nolint:errchkjson
			_, _ = fmt.Fprintln(w, string(data))
		}

		return nil
	},
}

func init() {
	detectCmd.Flags().Bool("framework", false, "Framework detection only")
	detectCmd.Flags().Bool("pwa", false, "PWA capability check only")
	detectCmd.Flags().Bool("tech", false, "Technology stack analysis only")
	detectCmd.Flags().Bool("render", false, "Rendering mode detection only")
	rootCmd.AddCommand(detectCmd)
}
