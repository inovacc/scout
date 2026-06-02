package vault

import (
	"crypto/rand"
	"encoding/base64"
)

// newID returns an opaque, enumeration-resistant 22-char profile ID
// (128 bits of entropy, base64url without padding).
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("scout: vault: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
