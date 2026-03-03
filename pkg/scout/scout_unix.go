//go:build !windows

package scout

import "github.com/inovacc/scout/internal/engine"

func WithXvfb(args ...string) Option { return engine.WithXvfb(args...) }
