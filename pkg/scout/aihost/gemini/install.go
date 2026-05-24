// Gemini-specific install ritual. Gemini extensions live at
// ~/.gemini/extensions/<name>/ with a single gemini-extension.json
// manifest. The `gemini extensions install <local-path>` CLI copies
// the directory into place and registers it. We mirror that here:
// write to the canonical target with atomic tmp+rename, then shell
// out to the CLI for registration. Fall back to a manual hint when
// the gemini CLI isn't on PATH.

package gemini

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/aihost/claude"
)

// Install writes assets + manifest to target and registers via the
// gemini CLI when available. Returns file count written.
func Install(target string) (int, error) {
	if target == "" {
		return 0, fmt.Errorf("target must not be empty")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir target: %w", err)
	}
	count := 0
	writeOne := func(rel string, data []byte) error {
		dst := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
		count++
		return nil
	}
	h := Host{}
	if err := h.Walk(writeOne); err != nil {
		return count, err
	}
	gen, err := h.ManifestFiles()
	if err != nil {
		return count, err
	}
	for rel, data := range gen {
		if err := writeOne(rel, data); err != nil {
			return count, err
		}
	}
	registerWithGeminiCLI(target)
	return count, nil
}

// Uninstall removes the install target. Best-effort shell-out to
// `gemini extensions uninstall scout` to clean the CLI's registration.
func Uninstall(target string) error {
	if bin, err := exec.LookPath("gemini"); err == nil {
		out, _ := exec.Command(bin, "extensions", "uninstall", claude.Name).CombinedOutput()
		fmt.Fprintf(os.Stderr, "[uninstall] gemini extensions uninstall: %s", string(out))
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove %s: %w", target, err)
	}
	fmt.Fprintf(os.Stderr, "[uninstall] gemini: removed %s\n", target)
	return nil
}

// PrintStatus reports install-state for Gemini.
func PrintStatus(w *os.File) error {
	target, _ := Host{}.InstallTarget()
	fmt.Fprintf(w, "embedded plugin    : %s v%s\n", claude.Name, claude.Version)
	fmt.Fprintf(w, "install target     : %s\n", target)
	fmt.Fprintf(w, "target exists      : %v\n", pathExists(target))
	fmt.Fprintf(w, "manifest present   : %v\n", pathExists(filepath.Join(target, "gemini-extension.json")))
	if _, err := exec.LookPath("gemini"); err == nil {
		fmt.Fprintf(w, "gemini CLI         : on PATH (registration via CLI on install)\n")
	} else {
		fmt.Fprintf(w, "gemini CLI         : not on PATH (manual registration required)\n")
	}
	return nil
}

// registerWithGeminiCLI shells out to `gemini extensions install <target>`.
// If the CLI isn't on PATH, prints the manual command for the user to run.
func registerWithGeminiCLI(target string) {
	geminiBin, err := exec.LookPath("gemini")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[install] gemini CLI not on PATH — run manually:\n  gemini extensions install %q\n", target)
		return
	}
	out, err := exec.Command(geminiBin, "extensions", "install", target).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[install] gemini extensions install said: %s\n", string(out))
		return
	}
	fmt.Fprintln(os.Stderr, "[install] registered via gemini extensions install")
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
