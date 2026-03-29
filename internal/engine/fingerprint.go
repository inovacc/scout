package engine

import (
	"github.com/inovacc/scout/internal/engine/fingerprint"
)

// Fingerprint re-exports fingerprint.Fingerprint from sub-package.
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

// Internal aliases for browser_rod.go compatibility.
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

