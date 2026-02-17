//go:build windows

package scout

import (
	"fmt"
	"os"
	"path/filepath"
)

func lookupBrowser(bt BrowserType) (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	switch bt {
	case BrowserBrave:
		paths := []string{
			filepath.Join(localAppData, `BraveSoftware\Brave-Browser\Application\brave.exe`),
			filepath.Join(programFiles, `BraveSoftware\Brave-Browser\Application\brave.exe`),
			filepath.Join(programFilesX86, `BraveSoftware\Brave-Browser\Application\brave.exe`),
		}
		return firstExisting(paths, bt)

	case BrowserEdge:
		paths := []string{
			filepath.Join(programFiles, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(programFilesX86, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(localAppData, `Microsoft\Edge\Application\msedge.exe`),
		}
		return firstExisting(paths, bt)

	case BrowserChrome:
		return "", nil // rod auto-detect

	default:
		return "", fmt.Errorf("%w: unknown browser type %q", ErrBrowserNotFound, bt)
	}
}
