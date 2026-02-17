//go:build linux

package server

import "github.com/inovacc/scout/pkg/scout"

func platformSessionDefaults() []scout.Option {
	return []scout.Option{scout.WithNoSandbox()}
}
