package cmd

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
	"clone": "repo", "add": "repo", "remove": "repo", "list": "repo",
	"open": "repo", "favorite": "repo", "unfavorite": "repo", "map": "repo",

	"pull": "git", "push": "git", "commit": "git", "checkout": "git",
	"merge": "git", "stash": "git", "tag": "git", "scan": "git",
	"branches": "git", "diff": "git", "status": "git", "reauthor": "git",

	"gh": "github", "org": "github",

	"pm": "pm", "ai": "ai",

	"gmail": "services", "teams": "services", "outlook": "services", "slack": "services",

	"configure": "config", "profile": "config",

	"server": "infra", "service": "infra", "mirror": "infra",

	"workspace": "tools", "data": "tools", "update": "tools", "version": "tools",
	"cmdtree": "tools", "aicontext": "tools", "nerds": "tools", "monitor": "tools",
}

var aiCategoryNames = map[string]string{
	"repo":     "Repository",
	"git":      "Git",
	"github":   "GitHub",
	"pm":       "Project Management",
	"ai":       "AI Planning",
	"services": "Services",
	"config":   "Configuration",
	"infra":    "Infrastructure",
	"tools":    "Tools",
}

var aicontextCmd = &cobra.Command{
	Use:   "aicontext",
	Short: "Generate AI context for coding agents",
	Long: `Generate concise context for AI coding agents.

Examples:
  kody aicontext              # Markdown output
  kody aicontext --json       # JSON output
  kody aicontext -c github    # Filter by category`,
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
		App:        "kody",
		Desc:       "Your code companion - Git repository manager with AI-powered planning",
		Categories: categories,
	}

	if !noStruct {
		ctx.Structure = map[string]string{
			"cmd/":           "CLI commands",
			"internal/ai/":   "AI personas",
			"internal/core/": "Business logic",
			"internal/git/":  "Git client",
			"tests/":         "Go integration tests",
			"testing/":       "Python black-box tests",
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
