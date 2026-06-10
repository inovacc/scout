package main

import (
	"fmt"
	"maps"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/vault"
	"github.com/spf13/cobra"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Encrypted secrets vault (Argon2id + AES-256-GCM)",
}

var vaultInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new vault",
	RunE: func(cmd *cobra.Command, _ []string) error {
		pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "New vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(pass)
		v, err := vault.Create(pass, vaultPathOpts(cmd)...)
		if err != nil {
			return err
		}
		_ = v.Close()
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "vault initialized")
		return nil
	},
}

var vaultSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE [KEY=VALUE...]",
	Short: "Create or update a secret profile; prints its opaque ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		fromProfile, _ := cmd.Flags().GetString("from-profile")
		id, _ := cmd.Flags().GetString("id")

		var in vault.SecretProfileInput
		if fromProfile != "" {
			var err error
			if in, err = vault.FromUserProfile(fromProfile); err != nil {
				return err
			}
		}
		if name != "" {
			in.Name = name
		}
		in.ID = id
		parsed, err := parseSecretArgs(args)
		if err != nil {
			return err
		}
		if in.Secrets == nil {
			in.Secrets = map[string][]byte{}
		}
		maps.Copy(in.Secrets, parsed)

		pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(pass)
		v, err := vault.Open(pass, vaultPathOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()

		newID, err := v.Set(in)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), newID)
		return nil
	},
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret profiles (metadata only — never values)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		_, _ = fmt.Fprint(cmd.OutOrStdout(), renderVaultList(v.List()))
		return nil
	},
}

var vaultGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show one profile's metadata (never secret values)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		for _, m := range v.List() {
			if m.ID == args[0] {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), renderVaultList([]vault.ProfileMeta{m}))
				return nil
			}
		}
		return fmt.Errorf("scout: vault: profile %q not found", args[0])
	},
}

var vaultUseCmd = &cobra.Command{
	Use:   "use <id> --url <url>",
	Short: "Inject a profile into a one-shot browser page via CDP (closes after load)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		if url == "" {
			return fmt.Errorf("scout: vault: --url is required (daemon --session injection is not yet supported)")
		}
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		h, err := v.Use(args[0])
		if err != nil {
			return err
		}
		defer func() { _ = h.Close() }()

		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()
		page, err := b.NewPage("about:blank")
		if err != nil {
			return err
		}
		if err := h.ApplyToPage(page); err != nil {
			return err
		}
		if err := page.Navigate(url); err != nil {
			return err
		}
		if err := page.WaitLoad(); err != nil {
			return err
		}
		if err := h.ApplyStorageToPage(page); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "injected profile %s into %s\n", args[0], url)
		return nil
	},
}

var vaultCaptureCmd = &cobra.Command{
	Use:   "capture <name> <url>",
	Short: "Capture a live local session's cookies + web storage into a vault profile",
	Long: `Launches a local browser, navigates to <url>, and stores the session's
cookies and the current origin's localStorage/sessionStorage into a vault profile
named <name>. Prints the profile's opaque ID.

For an authenticated capture, pair with --user-data-dir (an existing Chrome
profile) or a headed interactive login so the session is logged in before capture.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, url := args[0], args[1]

		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()

		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()

		page, err := b.NewPage(url)
		if err != nil {
			return err
		}
		if err := page.WaitLoad(); err != nil {
			return err
		}

		in, err := vault.CaptureFromPage(page, name)
		if err != nil {
			return err
		}
		id, err := v.Set(in)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), id)
		return nil
	},
}

var vaultRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Re-encrypt the vault under a new passphrase",
	RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		newPass, err := readPassphraseBytes(cmd.ErrOrStderr(), "New vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(newPass)
		if err := v.Rotate(newPass); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "vault rotated")
		return nil
	},
}

var vaultRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a secret profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		if err := v.Remove(args[0]); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removed "+args[0])
		return nil
	},
}

// vaultPathOpts maps the optional --vault-file flag to a vault.Option slice.
func vaultPathOpts(cmd *cobra.Command) []vault.Option {
	if p, _ := cmd.Flags().GetString("vault-file"); p != "" {
		return []vault.Option{vault.WithPath(p)}
	}
	return nil
}

// openVaultCLI reads the passphrase and opens the vault.
func openVaultCLI(cmd *cobra.Command) (*vault.Vault, error) {
	pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
	if err != nil {
		return nil, err
	}
	defer zeroBytesCLI(pass)
	return vault.Open(pass, vaultPathOpts(cmd)...)
}

// parseSecretArgs converts KEY=VALUE arguments into a secrets map. On a malformed
// argument it reports only the position — NEVER the raw value, which may be a secret.
func parseSecretArgs(args []string) (map[string][]byte, error) {
	secrets := make(map[string][]byte, len(args))
	for i, kv := range args {
		k, val, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("scout: vault: argument %d is not KEY=VALUE", i+1)
		}
		secrets[k] = []byte(val)
	}
	return secrets, nil
}

// renderVaultList formats profile metadata. It MUST NOT print any secret value.
func renderVaultList(metas []vault.ProfileMeta) string {
	var sb strings.Builder
	for _, m := range metas {
		fmt.Fprintf(&sb, "%s  %s  secrets=%s  headers=%d  cookies=%d\n",
			m.ID, m.Name, strings.Join(m.SecretKeys, ","), len(m.HeaderKeys), m.CookieN)
	}
	return sb.String()
}

// zeroBytesCLI overwrites a passphrase slice once it is no longer needed.
func zeroBytesCLI(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func init() {
	vaultSetCmd.Flags().String("name", "", "human label for the profile")
	vaultSetCmd.Flags().String("from-profile", "", "import secret fields from a .scoutprofile")
	vaultSetCmd.Flags().String("id", "", "update an existing profile by ID")
	vaultUseCmd.Flags().String("url", "", "URL to open and inject into")

	for _, c := range []*cobra.Command{vaultInitCmd, vaultSetCmd, vaultListCmd, vaultGetCmd, vaultUseCmd, vaultCaptureCmd, vaultRotateCmd, vaultRmCmd} {
		c.Flags().String("vault-file", "", "override vault file path (default <scouthome>/profiles/vault.bin)")
	}
	vaultCmd.AddCommand(vaultInitCmd, vaultSetCmd, vaultListCmd, vaultGetCmd, vaultUseCmd, vaultCaptureCmd, vaultRotateCmd, vaultRmCmd)
	rootCmd.AddCommand(vaultCmd)
}
