package interaction

import (
	"os"

	"github.com/inovacc/scout/internal/engine/scouthome"
	"github.com/inovacc/scout/internal/flags"
)

const feature = "interactions"

// Enabled reports whether interaction capture is on (feature flag or env).
func Enabled() bool {
	if flags.IsFeatureEnabled(feature) {
		return true
	}

	switch os.Getenv("SCOUT_INTERACTIONS") {
	case "1", "true", "TRUE", "yes":
		return true
	}

	return false
}

// Dir returns the capture directory: the feature's stored data path if set,
// otherwise <scouthome>/captures.
func Dir() (string, error) {
	if d := flags.GetFeatureData(feature); d != "" {
		return d, nil
	}

	return scouthome.Sub("captures")
}
