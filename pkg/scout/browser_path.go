package scout

import (
	"errors"
	"fmt"
	"os"
)

// ErrBrowserNotFound is returned when the requested browser executable cannot be located.
var ErrBrowserNotFound = errors.New("browser executable not found")

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// firstExisting returns the first path that exists on disk, or an error.
func firstExisting(paths []string, bt BrowserType) (string, error) {
	for _, p := range paths {
		if fileExists(p) {
			return p, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrBrowserNotFound, bt)
}
