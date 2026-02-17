//go:build darwin

package scout

import "fmt"

func lookupBrowser(bt BrowserType) (string, error) {
	switch bt {
	case BrowserBrave:
		paths := []string{
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
		return firstExisting(paths, bt)

	case BrowserEdge:
		paths := []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
		return firstExisting(paths, bt)

	case BrowserChrome:
		return "", nil // rod auto-detect

	default:
		return "", fmt.Errorf("%w: unknown browser type %q", ErrBrowserNotFound, bt)
	}
}
