package devices

import "slices"

// Clear is used to clear overrides.
var Clear = Device{clear: true}

func has(arr []string, str string) bool {
	return slices.Contains(arr, str)
}
