package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// AIContext represents the complete AI context document
type AIContext struct {
	App        string            `json:"app"`
	Desc       string            `json:"desc"`
	Categories map[string][]CMD  `json:"categories"`
	Structure  map[string]string `json:"structure,omitempty"`
}

// CMD represents a command
type CMD struct {
	Cmd   string   `json:"cmd"`
	Desc  string   `json:"desc"`
	Flags []string `json:"flags,omitempty"`
	Sub   []string `json:"sub,omitempty"`
}

// aiCategoryMap maps command names to categories
var aiCategoryMap = map[string]string{
	// Navigation
	"navigate": "nav", "back": "nav", "forward": "nav", "reload": "nav",

	// Interaction
	"click": "interact", "type": "interact", "select": "interact",
	"hover": "interact", "focus": "interact", "clear": "interact", "key": "interact",

	// Inspection
	"title": "inspect", "url": "inspect", "text": "inspect", "attr": "inspect",
	"eval": "inspect", "html": "inspect",

	// Capture
	"screenshot": "capture", "pdf": "capture", "har": "capture",

	// Scraping
	"crawl": "scrape", "search": "scrape", "table": "scrape", "meta": "scrape",
	"markdown": "scrape", "map": "scrape", "batch": "scrape", "form": "scrape",
	"recipe": "scrape",

	// Session & browser
	"session": "session", "window": "session", "cookie": "session",
	"storage": "session", "header": "session", "block": "session",
	"extension": "session",

	// Auth & identity
	"auth": "identity", "device": "identity",

	// Bridge
	"bridge": "infra",

	// Infrastructure
	"server": "infra", "client": "infra",

	// Tools
	"version": "tools", "cmdtree": "tools", "aicontext": "tools",
}

var aiCategoryNames = map[string]string{
	"nav":      "Navigation",
	"interact": "Interaction",
	"inspect":  "Inspection",
	"capture":  "Capture",
	"scrape":   "Scraping & Extraction",
	"session":  "Session & Browser",
	"identity": "Auth & Identity",
	"infra":    "Infrastructure",
	"tools":    "Tools",
}

var aicontextCmd = &cobra.Command{
	Use:   "aicontext",
	Short: "Generate AI context for coding agents",
	Long: `Generate concise context for AI coding agents.

Examples:
  scout aicontext              # Markdown output
  scout aicontext --json       # JSON output
  scout aicontext -c scrape    # Filter by category`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cat, _ := cmd.Flags().GetString("category")
		noStruct, _ := cmd.Flags().GetBool("no-structure")
		return runAIContext(cmd.OutOrStdout(), rootCmd, jsonOut, cat, noStruct)
	},
}

func init() {
	rootCmd.AddCommand(aicontextCmd)
	aicontextCmd.Flags().BoolP("json", "j", false, "JSON output")
	aicontextCmd.Flags().StringP("category", "c", "", "filter category")
	aicontextCmd.Flags().Bool("no-structure", false, "omit project structure")
}

func runAIContext(w io.Writer, root *cobra.Command, jsonOut bool, filterCat string, noStruct bool) error {
	ctx := buildContext(root, filterCat, noStruct)

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		return enc.Encode(ctx)
	}

	return writeMarkdown(w, ctx)
}

func buildContext(root *cobra.Command, filterCat string, noStruct bool) AIContext {
	categories := make(map[string][]CMD)

	for _, c := range root.Commands() {
		if c.Name() == "help" || c.Name() == "completion" || c.Hidden {
			continue
		}

		cat := aiCategoryMap[c.Name()]
		if cat == "" {
			cat = "other"
		}

		if filterCat != "" && cat != filterCat {
			continue
		}

		cmd := CMD{
			Cmd:  c.Name(),
			Desc: c.Short,
		}

		// Collect important flags only
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" {
				return
			}

			flag := "--" + f.Name
			if f.Shorthand != "" {
				flag = "-" + f.Shorthand + "/" + flag
			}

			cmd.Flags = append(cmd.Flags, flag)
		})

		// Collect subcommands
		for _, sub := range c.Commands() {
			if sub.Name() != "help" && !sub.Hidden {
				cmd.Sub = append(cmd.Sub, sub.Name())
			}
		}

		categories[cat] = append(categories[cat], cmd)
	}

	// Sort commands within categories
	for cat := range categories {
		sort.Slice(categories[cat], func(i, j int) bool {
			return categories[cat][i].Cmd < categories[cat][j].Cmd
		})
	}

	ctx := AIContext{
		App:        "scout",
		Desc:       "Headless browser automation, web scraping, search, crawling, and forensic capture",
		Categories: categories,
	}

	if !noStruct {
		ctx.Structure = map[string]string{
			"cmd/scout/":    "Unified Cobra CLI binary",
			"pkg/scout/":    "Core library (browser, page, element wrappers)",
			"pkg/stealth/":  "Anti-bot-detection stealth module",
			"pkg/identity/": "Device identity and mTLS certificates",
			"pkg/discovery/": "mDNS service discovery",
			"grpc/proto/":   "Protobuf definitions",
			"grpc/server/":  "gRPC server with mTLS and pairing",
			"scraper/":      "Auth framework and encrypted sessions",
			"examples/":     "Runnable example programs",
			"tests/":        "Integration tests and recipes",
		}
	}

	return ctx
}

func writeMarkdown(w io.Writer, ctx AIContext) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n%s\n\n", ctx.App, ctx.Desc))

	// Sort categories for consistent output
	cats := make([]string, 0, len(ctx.Categories))
	for cat := range ctx.Categories {
		cats = append(cats, cat)
	}

	sort.Strings(cats)

	for _, cat := range cats {
		cmds := ctx.Categories[cat]

		name := aiCategoryNames[cat]
		if name == "" {
			name = cat
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", name))

		for _, cmd := range cmds {
			sb.WriteString(fmt.Sprintf("### %s\n%s\n", cmd.Cmd, cmd.Desc))

			if len(cmd.Flags) > 0 {
				sb.WriteString(fmt.Sprintf("Flags: `%s`\n", strings.Join(cmd.Flags, "` `")))
			}

			if len(cmd.Sub) > 0 {
				sb.WriteString(fmt.Sprintf("Sub: `%s`\n", strings.Join(cmd.Sub, "` `")))
			}

			sb.WriteString("\n")
		}
	}

	if len(ctx.Structure) > 0 {
		sb.WriteString("## Structure\n\n")

		keys := make([]string, 0, len(ctx.Structure))
		for k := range ctx.Structure {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("- `%s` %s\n", k, ctx.Structure[k]))
		}
	}

	_, err := io.WriteString(w, sb.String())

	return err
}
