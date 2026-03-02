package scout

import (
	"net/url"
	"sync"
	"time"
)

// FingerprintRotation defines the rotation strategy.
type FingerprintRotation int

const (
	// FingerprintRotatePerSession generates one fingerprint per browser session.
	FingerprintRotatePerSession FingerprintRotation = iota
	// FingerprintRotatePerPage generates a new fingerprint for each new page.
	FingerprintRotatePerPage
	// FingerprintRotatePerDomain uses a consistent fingerprint per domain.
	FingerprintRotatePerDomain
	// FingerprintRotateInterval rotates on a time interval.
	FingerprintRotateInterval
)

// FingerprintRotationConfig configures fingerprint rotation behaviour.
type FingerprintRotationConfig struct {
	Strategy FingerprintRotation
	Interval time.Duration       // for FingerprintRotateInterval
	Options  []FingerprintOption // generation constraints
	Pool     []*Fingerprint      // optional pre-generated pool (round-robin)
}

// fingerprintRotator manages fingerprint rotation state.
type fingerprintRotator struct {
	mu         sync.Mutex
	config     FingerprintRotationConfig
	current    *Fingerprint
	domainMap  map[string]*Fingerprint
	poolIndex  int
	lastRotate time.Time
}

func newFingerprintRotator(config FingerprintRotationConfig) *fingerprintRotator {
	r := &fingerprintRotator{
		config:     config,
		domainMap:  make(map[string]*Fingerprint),
		lastRotate: time.Now(),
	}
	// Generate initial fingerprint.
	r.current = r.generate()

	return r
}

// forPage returns the fingerprint to use for a given page URL domain.
func (r *fingerprintRotator) forPage(domain string) *Fingerprint {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	switch r.config.Strategy { //nolint:exhaustive
	case FingerprintRotatePerPage:
		r.current = r.generate()
		return r.current

	case FingerprintRotatePerDomain:
		if fp, ok := r.domainMap[domain]; ok {
			return fp
		}

		fp := r.generate()
		r.domainMap[domain] = fp

		return fp

	case FingerprintRotateInterval:
		if r.config.Interval > 0 && time.Since(r.lastRotate) >= r.config.Interval {
			r.current = r.generate()
			r.lastRotate = time.Now()
		}

		return r.current

	default: // PerSession
		return r.current
	}
}

// domainFromURL extracts the hostname from a URL string.
// Returns empty string if parsing fails.
func domainFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return u.Hostname()
}

// generate returns the next fingerprint, using the pool if available.
func (r *fingerprintRotator) generate() *Fingerprint {
	if len(r.config.Pool) > 0 {
		fp := r.config.Pool[r.poolIndex%len(r.config.Pool)]
		r.poolIndex++

		return fp
	}

	return GenerateFingerprint(r.config.Options...)
}
