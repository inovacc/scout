//go:build linux

package scout

import (
	"fmt"
	"os/exec"
)

func lookupBrowser(bt BrowserType) (string, error) {
	switch bt {
	case BrowserBrave:
		paths := []string{
			"/usr/bin/brave-browser",
			"/usr/bin/brave-browser-stable",
			"/opt/brave.com/brave/brave-browser",
			"/snap/bin/brave",
		}
		if p, err := firstExisting(paths, bt); err == nil {
			return p, nil
		}
		if p, err := exec.LookPath("brave-browser"); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%w: %s", ErrBrowserNotFound, bt)

	case BrowserEdge:
		paths := []string{
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"/opt/microsoft/msedge/msedge",
		}
		if p, err := firstExisting(paths, bt); err == nil {
			return p, nil
		}
		if p, err := exec.LookPath("microsoft-edge"); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%w: %s", ErrBrowserNotFound, bt)

	case BrowserChrome:
		return "", nil // rod auto-detect

	default:
		return "", fmt.Errorf("%w: unknown browser type %q", ErrBrowserNotFound, bt)
	}
}
