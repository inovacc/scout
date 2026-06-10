package capture

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

const extIDLen = 32 // Chrome maps the first 16 bytes of SHA-256 to 32 chars in a-p.

// OriginToExtID extracts and validates the extension ID from a
// chrome-extension:// origin (the value Chrome passes as argv[1] when it
// launches a native-messaging host). ok is false for anything else.
func OriginToExtID(origin string) (string, bool) {
	const p = "chrome-extension://"
	if !strings.HasPrefix(origin, p) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(origin, p), "/")
	if len(id) != extIDLen {
		return "", false
	}
	for _, c := range id {
		if c < 'a' || c > 'p' {
			return "", false
		}
	}
	return id, true
}

// IsNativeMessagingLaunch reports whether os.Args indicates Chrome launched us
// as a native-messaging host (any arg is a chrome-extension:// origin). It
// returns the origin so the caller can cross-check the ID.
func IsNativeMessagingLaunch(args []string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, "chrome-extension://") {
			return a, true
		}
	}
	return "", false
}

// ExtensionID derives the stable Chrome extension ID from a DER-encoded SPKI
// public key, matching Chromium's crx_file::id_util::GenerateId: the first 16
// bytes of SHA-256(der), each nibble mapped 0..15 -> 'a'..'p'.
func ExtensionID(derSPKI []byte) string {
	sum := sha256.Sum256(derSPKI)
	var b strings.Builder
	b.Grow(extIDLen)
	for _, c := range sum[:extIDLen/2] {
		b.WriteByte('a' + (c >> 4))
		b.WriteByte('a' + (c & 0x0f))
	}
	return b.String()
}

// ManifestKey returns the base64 value to place in manifest.json "key" so the
// loaded extension gets the stable ExtensionID(derSPKI).
func ManifestKey(derSPKI []byte) string {
	return base64.StdEncoding.EncodeToString(derSPKI)
}
