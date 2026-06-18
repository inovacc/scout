package okf

import (
	"fmt"
	"strings"
)

// Validate checks the bundle for consistency:
//   - Every Concept must have a non-empty Type field.
//   - Every bundle-relative link in concept bodies must resolve to an existing concept ID.
//
// Returns the first error found, or nil when the bundle is valid.
func (b *Bundle) Validate() error {
	// Build a set of all concept IDs for fast lookup.
	ids := make(map[string]struct{}, len(b.Concepts))
	for _, c := range b.Concepts {
		ids[c.ID] = struct{}{}
	}

	for _, c := range b.Concepts {
		if strings.TrimSpace(c.Type) == "" {
			return fmt.Errorf("okf: validate: concept %q has empty type", c.ID)
		}

		for _, link := range c.Links() {
			if _, ok := ids[link]; !ok {
				return fmt.Errorf("okf: validate: concept %q links to unknown concept %q", c.ID, link)
			}
		}
	}

	return nil
}
