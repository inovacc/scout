package capture

import "golang.org/x/crypto/curve25519"

// curve25519ScalarBaseMult derives the Curve25519 public key for priv.
func curve25519ScalarBaseMult(dst, priv *[32]byte) {
	pub, _ := curve25519.X25519(priv[:], curve25519.Basepoint)
	copy(dst[:], pub)
}
