//go:build darwin

package browser

import "fmt"

func lookupBrowser(bt BrowserType) (string, error) {
	switch bt {
	case Brave:
		paths := []string{
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
		return firstExisting(paths, bt)

	case Edge:
		paths := []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
		return firstExisting(paths, bt)

	case Chrome:
		return "", nil // rod auto-detect

	default:
		return "", fmt.Errorf("%w: unknown browser type %q", ErrNotFound, bt)
	}
}
