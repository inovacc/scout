package aria

import (
	"github.com/inovacc/scout/internal/engine"
)

// ResolveElement converts a CDP backend node ID into a live *engine.Element
// suitable for action methods (.Click, .Input, .Hover, ...). Returns
// AmbiguousRefError if the node has been detached from the DOM since the
// snapshot was captured.
func ResolveElement(page *engine.Page, backendNodeID int64) (*engine.Element, error) {
	el, err := page.ElementFromBackendNodeID(int(backendNodeID))
	if err != nil {
		return nil, &AmbiguousRefError{BackendNodeID: backendNodeID}
	}
	return el, nil
}
