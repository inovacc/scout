// Package all is a barrel-import that triggers every host package's
// init() so they register themselves into aihost.All(). Import this
// package (blank-import) anywhere you need the full host registry.
package all

import (
	_ "github.com/inovacc/scout/pkg/scout/aihost/claude"
	_ "github.com/inovacc/scout/pkg/scout/aihost/codex"
	_ "github.com/inovacc/scout/pkg/scout/aihost/gemini"
)
