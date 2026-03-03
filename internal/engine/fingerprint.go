package engine

import (
	"time"

	"github.com/inovacc/scout/internal/engine/fingerprint"
)

// Re-export fingerprint types and functions.
type Fingerprint = fingerprint.Fingerprint
type FingerprintOption = fingerprint.FingerprintOption
type FingerprintRotation = fingerprint.FingerprintRotation
type FingerprintRotationConfig = fingerprint.FingerprintRotationConfig
type FingerprintStore = fingerprint.FingerprintStore
type StoredFingerprint = fingerprint.StoredFingerprint

var (
	GenerateFingerprint = fingerprint.GenerateFingerprint
	NewFingerprintStore = fingerprint.NewFingerprintStore

	WithFingerprintOS     = fingerprint.WithFingerprintOS
	WithFingerprintMobile = fingerprint.WithFingerprintMobile
	WithFingerprintLocale = fingerprint.WithFingerprintLocale
)

// Internal aliases for browser.go compatibility.
type fingerprintRotator = fingerprint.Rotator

var (
	newFingerprintRotator = fingerprint.NewRotator
	domainFromURL         = fingerprint.DomainFromURL
)

// Fingerprint rotation strategy constants.
const (
	FingerprintRotatePerSession = fingerprint.FingerprintRotatePerSession
	FingerprintRotatePerPage    = fingerprint.FingerprintRotatePerPage
	FingerprintRotatePerDomain  = fingerprint.FingerprintRotatePerDomain
	FingerprintRotateInterval   = fingerprint.FingerprintRotateInterval
)

// FingerprintToProfile converts a fingerprint to a UserProfile for persistence.
func FingerprintToProfile(fp *Fingerprint) *UserProfile {
	now := time.Now()

	lang := ""
	if len(fp.Languages) > 0 {
		lang = fp.Languages[0]
	}

	return &UserProfile{
		Version:   1,
		Name:      "fingerprint-" + now.Format("20060102-150405"),
		CreatedAt: now,
		UpdatedAt: now,
		Browser: ProfileBrowser{
			WindowW: fp.ScreenWidth,
			WindowH: fp.ScreenHeight,
		},
		Identity: ProfileIdentity{
			UserAgent: fp.UserAgent,
			Language:  lang,
			Timezone:  fp.Timezone,
		},
	}
}
