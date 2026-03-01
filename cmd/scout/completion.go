package main

import (
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for scout.

To load completions:

Bash:
  $ source <(scout completion bash)
  # Or add to ~/.bashrc:
  $ scout completion bash > /etc/bash_completion.d/scout

Zsh:
  $ scout completion zsh > "${fpath[1]}/_scout"
  # Then restart your shell.

Fish:
  $ scout completion fish | source
  # Or persist:
  $ scout completion fish > ~/.config/fish/completions/scout.fish

PowerShell:
  PS> scout completion powershell | Out-String | Invoke-Expression
  # Or add to $PROFILE.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}
