package browser

import (
	"errors"
	"fmt"
	"os"
)

// ErrNotFound is returned when the requested browser executable cannot be located.
var ErrNotFound = errors.New("browser executable not found")

// FileExists returns true if path exists and is a regular file.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// firstExisting returns the first path that exists on disk, or an error.
func firstExisting(paths []string, bt BrowserType) (string, error) {
	for _, p := range paths {
		if FileExists(p) {
			return p, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrNotFound, bt)
}

// LookupBrowser is the exported version of lookupBrowser for external use.
func LookupBrowser(bt BrowserType) (string, error) {
	return lookupBrowser(bt)
}

// IsNotFound checks if the error wraps ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
