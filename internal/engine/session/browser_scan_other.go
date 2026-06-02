//go:build !windows

package session

// findBrowsersWindows stub for non-Windows builds. Linux uses the cross-
// platform findBrowsersLinux; Darwin/BSD return nil from the dispatcher.
func findBrowsersWindows(dataDir string) []int { return nil }
