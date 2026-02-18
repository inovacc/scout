package extensions

import (
	"embed"
	"io/fs"
)

//go:embed scout-bridge
var bridgeExtension embed.FS

func BridgeExtension() (fs.FS, error) {
	return fs.Sub(bridgeExtension, "scout-bridge")
}
