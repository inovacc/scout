package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ASCII tree characters for consistent width across all terminals
const (
	treeMiddle  = "+-- "
	treeLast    = "\\-- "
	treeIndent  = "|   "
	treeSpace   = "    "
	includeHelp = true
	showHidden  = true
	maxDescLen  = 40
	commentCol  = 45
)

// cmdtree flags
var (
	cmdtreeVerbose bool
	cmdtreeBrief   bool
	cmdtreeCommand string
	cmdtreeJSON    bool
)

// FlagDetail represents a single flag's information
type FlagDetail struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

// CommandDetail represents a command's full information
type CommandDetail struct {
	Name        string          `json:"name"`
	Use         string          `json:"use"`
	Short       string          `json:"short"`
	Long        string          `json:"long,omitempty"`
	Flags       []FlagDetail    `json:"flags,omitempty"`
	Subcommands []CommandDetail `json:"commands,omitempty"`
}

var cmdtreeCmd = &cobra.Command{
	Use:   "cmdtree",
	Short: "Display command tree visualization",
	Long:  "Display a tree visualization of all available commands with descriptions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmdtreeJSON {
			return printJSONTree(cmd, rootCmd)
		}

		if cmdtreeCommand != "" {
			return printSingleCommand(cmd, rootCmd, cmdtreeCommand)
		}

		var tree bytes.Buffer

		tree.WriteString("# Command Tree\n\n```\n")
		if cmdtreeBrief || !cmdtreeVerbose {
			tree.Write(buildTree(rootCmd))
		} else {
			tree.Write(buildVerboseTree(rootCmd))
		}
		tree.WriteString("```\n")

		cmd.Println(tree.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cmdtreeCmd)

	cmdtreeCmd.Flags().BoolVarP(&cmdtreeVerbose, "verbose", "v", true, "Show full details for all commands (default)")
	cmdtreeCmd.Flags().BoolVarP(&cmdtreeBrief, "brief", "b", false, "Show compact tree with short descriptions only")
	cmdtreeCmd.Flags().StringVarP(&cmdtreeCommand, "command", "c", "", "Show details for a specific command only")
	cmdtreeCmd.Flags().BoolVar(&cmdtreeJSON, "json", false, "Output in JSON format")
}

func buildTree(root *cobra.Command) []byte {
	var buf bytes.Buffer

	_, _ = buf.WriteString(fmt.Sprintf("%s\n", root.Use))
	printCommands(&buf, root.Commands(), "")

	return buf.Bytes()
}

func buildVerboseTree(root *cobra.Command) []byte {
	var buf bytes.Buffer

	_, _ = buf.WriteString(fmt.Sprintf("%s\n", root.Use))
	printVerboseCommands(&buf, root.Commands(), "")

	return buf.Bytes()
}

func printCommands(w io.Writer, commands []*cobra.Command, prefix string) {
	var visible []*cobra.Command

	for _, c := range commands {
		if !includeHelp && (c.Name() == "help" || c.Name() == "completion") {
			continue
		}

		if !showHidden && c.Hidden {
			continue
		}

		visible = append(visible, c)
	}

	for i, c := range visible {
		isLast := i == len(visible)-1

		connector := treeMiddle
		if isLast {
			connector = treeLast
		}

		desc := c.Short
		if desc == "" {
			desc = c.Long
		}

		if len(desc) > maxDescLen {
			desc = fmt.Sprintf("%s...", desc[:maxDescLen-3])
		}

		cmdPart := prefix + connector + c.Name()

		padding := max(commentCol-len(cmdPart), 2)

		_, _ = fmt.Fprintf(w, "%s%s# %s\n", cmdPart, strings.Repeat(" ", padding), desc)

		if len(c.Commands()) > 0 {
			newPrefix := prefix + treeIndent
			if isLast {
				newPrefix = prefix + treeSpace
			}

			printCommands(w, c.Commands(), newPrefix)
		}
	}
}

func printVerboseCommands(w io.Writer, commands []*cobra.Command, prefix string) {
	var visible []*cobra.Command

	for _, c := range commands {
		if !includeHelp && (c.Name() == "help" || c.Name() == "completion") {
			continue
		}

		if !showHidden && c.Hidden {
			continue
		}

		visible = append(visible, c)
	}

	for i, c := range visible {
		isLast := i == len(visible)-1

		connector := treeMiddle
		if isLast {
			connector = treeLast
		}

		// Print command name
		_, _ = fmt.Fprintf(w, "%s%s%s\n", prefix, connector, c.Name())

		// Determine the continuation prefix for details
		detailPrefix := prefix + treeIndent
		if isLast {
			detailPrefix = prefix + treeSpace
		}

		// Print usage
		_, _ = fmt.Fprintf(w, "%sUsage: %s\n", detailPrefix, c.UseLine())

		// Print description
		desc := c.Short
		if c.Long != "" {
			desc = c.Long
		}

		if desc != "" {
			_, _ = fmt.Fprintf(w, "%sDescription: %s\n", detailPrefix, desc)
		}

		// Print flags
		flags := collectFlags(c)
		if len(flags) > 0 {
			_, _ = fmt.Fprintf(w, "%s\n", detailPrefix)

			_, _ = fmt.Fprintf(w, "%sFlags:\n", detailPrefix)
			for _, f := range flags {
				printFlagDetail(w, detailPrefix+"  ", f)
			}
		}

		// Add blank line between commands
		_, _ = fmt.Fprintf(w, "%s\n", detailPrefix)

		// Handle subcommands
		if len(c.Commands()) > 0 {
			printVerboseCommands(w, c.Commands(), detailPrefix)
		}
	}
}

func collectFlags(cmd *cobra.Command) []FlagDetail {
	var flags []FlagDetail

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Skip help flag as it's added automatically
		if f.Name == "help" {
			return
		}

		flags = append(flags, FlagDetail{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		})
	})

	return flags
}

func printFlagDetail(w io.Writer, prefix string, f FlagDetail) {
	var flagStr string

	if f.Shorthand != "" {
		flagStr = fmt.Sprintf("-%s, --%s", f.Shorthand, f.Name)
	} else {
		flagStr = fmt.Sprintf("    --%s", f.Name)
	}

	// Add type for non-bool flags
	if f.Type != "bool" {
		flagStr += " " + f.Type
	}

	// Pad to align descriptions
	padding := max(26-len(flagStr), 2)
	_, _ = fmt.Fprintf(w, "%s%s%s%s\n", prefix, flagStr, strings.Repeat(" ", padding), f.Description)
}

func printSingleCommand(cobraCmd *cobra.Command, root *cobra.Command, cmdName string) error {
	target := findCommand(root, cmdName)
	if target == nil {
		return fmt.Errorf("command not found: %s", cmdName)
	}

	if cmdtreeJSON {
		detail := buildCommandDetail(target)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(detail); err != nil {
			return fmt.Errorf("json encode: %w", err)
		}

		return nil
	}

	var buf bytes.Buffer

	_, _ = buf.WriteString(fmt.Sprintf("# %s\n\n", target.Name()))
	_, _ = buf.WriteString(fmt.Sprintf("Usage: %s\n\n", target.UseLine()))

	desc := target.Short
	if target.Long != "" {
		desc = target.Long
	}

	if desc != "" {
		_, _ = buf.WriteString(fmt.Sprintf("Description: %s\n\n", desc))
	}

	flags := collectFlags(target)
	if len(flags) > 0 {
		_, _ = buf.WriteString("Flags:\n")

		for _, f := range flags {
			printFlagDetail(&buf, "  ", f)
		}

		_, _ = buf.WriteString("\n")
	}

	if len(target.Commands()) > 0 {
		_, _ = buf.WriteString("Subcommands:\n")

		for _, sub := range target.Commands() {
			if !showHidden && sub.Hidden {
				continue
			}

			_, _ = fmt.Fprintf(&buf, "  %s - %s\n", sub.Name(), sub.Short)
		}
	}

	cobraCmd.Print(buf.String())

	return nil
}

func findCommand(root *cobra.Command, name string) *cobra.Command {
	if root.Name() == name {
		return root
	}

	for _, c := range root.Commands() {
		if c.Name() == name {
			return c
		}
		// Search in subcommands
		if found := findCommand(c, name); found != nil {
			return found
		}
	}

	return nil
}

func printJSONTree(_ *cobra.Command, root *cobra.Command) error {
	detail := buildCommandDetail(root)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(detail); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}

	return nil
}

func buildCommandDetail(cmd *cobra.Command) CommandDetail {
	detail := CommandDetail{
		Name:  cmd.Name(),
		Use:   cmd.UseLine(),
		Short: cmd.Short,
		Long:  cmd.Long,
		Flags: collectFlags(cmd),
	}

	for _, sub := range cmd.Commands() {
		if !includeHelp && (sub.Name() == "help" || sub.Name() == "completion") {
			continue
		}

		if !showHidden && sub.Hidden {
			continue
		}

		detail.Subcommands = append(detail.Subcommands, buildCommandDetail(sub))
	}

	return detail
}
