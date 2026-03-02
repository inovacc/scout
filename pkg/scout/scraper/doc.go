// Package scraper provides base types and utilities for building web scrapers
// with encrypted session persistence. It defines common interfaces (Credentials,
// Progress) and error types (AuthError, RateLimitError) shared across scraper
// modes, along with AES-256-GCM encryption backed by Argon2id key derivation.
package scraper
