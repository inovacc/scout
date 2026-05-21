// Package id implements Scout's encoded session-ID format.
//
// A session ID is a 36-character string: 12 attribute chars followed by 24
// random uppercase letters. The attribute prefix is queryable without
// reading any file on disk — `ls <scouthome>/sessions/` tells you the
// browser kind, run mode, lifetime, and feature flags at a glance.
//
//	AAAAAAAAAAAA XXXXXXXXXXXXXXXXXXXXXXXX
//	└─ 12 attr ┘└──── 24 random A-Z ────┘
//
// Attribute positions:
//
//	0  Version          '1'
//	1  Browser          C=chrome B=brave E=edge X=electron M=chromium
//	2  Run mode         H=headless V=visible/headed
//	3  Lifetime         P=persistent (reusable) E=ephemeral
//	4  Stealth          S=stealth N=normal
//	5  Bridge           B=bridge on N=off
//	6  VPN              V=vpn on N=off
//	7  Reserved         '0'
//	8  Reserved         '0'
//	9  Reserved         '0'
//	10 Reserved         '0'
//	11 Reserved         '0'
//
// Reserved positions are filled with '0' and reclaimed by future versions.
// A non-'1' at position 0 means "unknown version → reject".
//
// 26²⁴ ≈ 9.1 × 10³³ ≈ 113 bits of collision-resistance on the random portion
// — safely larger than any plausible session population.
package id

import (
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	// Len is the total ID length in bytes (== runes; ASCII only).
	Len = 36

	attrLen      = 12
	randomLen    = 24
	versionChar  = '1'
	reservedChar = '0'

	posVersion  = 0
	posBrowser  = 1
	posMode     = 2
	posLifetime = 3
	posStealth  = 4
	posBridge   = 5
	posVPN      = 6
)

// Browser codes (position 1).
const (
	browserCodeChrome   = 'C'
	browserCodeBrave    = 'B'
	browserCodeEdge     = 'E'
	browserCodeElectron = 'X'
	browserCodeChromium = 'M'
)

// Attrs is the parsed attribute prefix of a session ID.
type Attrs struct {
	Version  byte
	Browser  string // canonical lowercase: chrome/brave/edge/electron/chromium
	Headless bool
	Reusable bool
	Stealth  bool
	Bridge   bool
	VPN      bool
}

// browserToCode maps a canonical browser name to the position-1 char.
// Unknown names default to chrome so id generation is total.
func browserToCode(name string) byte {
	switch name {
	case "brave":
		return browserCodeBrave
	case "edge":
		return browserCodeEdge
	case "electron":
		return browserCodeElectron
	case "chromium":
		return browserCodeChromium
	default:
		return browserCodeChrome
	}
}

func codeToBrowser(c byte) (string, bool) {
	switch c {
	case browserCodeChrome:
		return "chrome", true
	case browserCodeBrave:
		return "brave", true
	case browserCodeEdge:
		return "edge", true
	case browserCodeElectron:
		return "electron", true
	case browserCodeChromium:
		return "chromium", true
	default:
		return "", false
	}
}

// New builds a session ID encoding attrs in the 12-char prefix followed by
// 24 random uppercase chars. Returns an error only if the OS RNG is
// unavailable.
func New(a Attrs) (string, error) {
	out := make([]byte, Len)

	out[posVersion] = versionChar
	out[posBrowser] = browserToCode(a.Browser)
	out[posMode] = boolChar(a.Headless, 'H', 'V')
	out[posLifetime] = boolChar(a.Reusable, 'P', 'E')
	out[posStealth] = boolChar(a.Stealth, 'S', 'N')
	out[posBridge] = boolChar(a.Bridge, 'B', 'N')
	out[posVPN] = boolChar(a.VPN, 'V', 'N')
	for i := 7; i < attrLen; i++ {
		out[i] = reservedChar
	}

	if err := fillRandomAlpha(out[attrLen:]); err != nil {
		return "", err
	}

	return string(out), nil
}

// Parse decodes the attribute prefix. Returns an error for the wrong length
// or an unsupported version. Random-portion content is not validated past
// length — any 24-char [A-Z] string passes.
func Parse(id string) (Attrs, error) {
	var a Attrs

	if len(id) != Len {
		return a, fmt.Errorf("scout/id: length %d != %d", len(id), Len)
	}

	if id[posVersion] != versionChar {
		return a, fmt.Errorf("scout/id: unsupported version %q", id[posVersion])
	}

	a.Version = id[posVersion]

	name, ok := codeToBrowser(id[posBrowser])
	if !ok {
		return a, fmt.Errorf("scout/id: unknown browser code %q", id[posBrowser])
	}
	a.Browser = name

	a.Headless = id[posMode] == 'H'
	a.Reusable = id[posLifetime] == 'P'
	a.Stealth = id[posStealth] == 'S'
	a.Bridge = id[posBridge] == 'B'
	a.VPN = id[posVPN] == 'V'

	return a, nil
}

// Valid reports whether id parses without error.
func Valid(id string) bool {
	_, err := Parse(id)
	return err == nil
}

// fillRandomAlpha writes len(buf) cryptographically-random [A-Z] bytes.
// Uppercase-only keeps IDs case-insensitive across filesystems / shells
// (Windows treats paths case-insensitively). 26²⁴ ≈ 113 bits of entropy.
func fillRandomAlpha(buf []byte) error {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	raw := make([]byte, len(buf))
	if _, err := rand.Read(raw); err != nil {
		return errors.New("scout/id: rand source unavailable")
	}

	for i, b := range raw {
		buf[i] = alpha[int(b)%len(alpha)]
	}

	return nil
}

func boolChar(b bool, t, f byte) byte {
	if b {
		return t
	}
	return f
}
