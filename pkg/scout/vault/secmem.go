// Package vault stores secret profiles in one encrypted file and injects
// secrets into live browser pages without leaking plaintext to child processes.
package vault

import "crypto/subtle"

// LockedBuffer wraps a []byte holding secret material. On allocation it best-effort
// locks the pages out of swap; Zero overwrites the bytes and unlocks. Secrets held
// here must NEVER be converted to string (strings are immutable and GC-pinned).
type LockedBuffer struct {
	buf    []byte
	locked bool
}

// NewLockedBuffer takes ownership of b (does not copy) and attempts to lock it.
// Locking is best-effort: failure is silently tolerated (never fatal).
func NewLockedBuffer(b []byte) *LockedBuffer {
	lb := &LockedBuffer{buf: b}
	if len(b) > 0 && lockPages(b) == nil {
		lb.locked = true
	}
	return lb
}

// Bytes returns the underlying secret slice. Do not retain beyond the buffer's life.
func (lb *LockedBuffer) Bytes() []byte { return lb.buf }

// Len returns the current secret length (0 after Zero/Close).
func (lb *LockedBuffer) Len() int { return len(lb.buf) }

// Equal reports whether the buffer equals other in constant time.
func (lb *LockedBuffer) Equal(other []byte) bool {
	return subtle.ConstantTimeCompare(lb.buf, other) == 1
}

// Zero overwrites the secret with zeros and unlocks the pages.
func (lb *LockedBuffer) Zero() {
	for i := range lb.buf {
		lb.buf[i] = 0
	}
	if lb.locked {
		_ = unlockPages(lb.buf)
		lb.locked = false
	}
	lb.buf = lb.buf[:0]
}

// Close zeros and releases the buffer. Always returns nil; signature allows defer.
func (lb *LockedBuffer) Close() error {
	lb.Zero()
	return nil
}
