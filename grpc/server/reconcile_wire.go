package server

import "github.com/inovacc/scout/pkg/scout"

// init points the reaper hook at the real facade reaper. scout.ReapOnce
// performs one path-bounded pass over <scouthome>/sessions, killing
// holders and removing orphan dirs; it returns a ReapStats whose Killed
// field is the holder-kill count.
func init() {
	reapHook = func() int {
		return scout.ReapOnce().Killed
	}
}
