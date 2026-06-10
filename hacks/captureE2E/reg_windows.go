//go:build windows

package main

import "golang.org/x/sys/windows/registry"

// regExists reports whether the Chrome native-messaging-host registry key for
// the Scout Capture host is present under HKCU. Used to verify install/uninstall.
func regExists() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Google\Chrome\NativeMessagingHosts\`+nativeHost, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	_ = k.Close()
	return true
}
