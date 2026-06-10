//go:build !windows

package main

// regExists is a no-op on non-Windows platforms; the native-messaging host
// manifest there is a plain file (no registry). This driver targets Windows.
func regExists() bool { return false }
