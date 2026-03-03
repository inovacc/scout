package engine

import (
	"github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/internal/engine/stealth"
)

// stealthPage creates a stealth page with evasion scripts injected.
func stealthPage(b *rodBrowser) (*rodPage, error) {
	p, err := b.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}

	_, err = p.EvalOnNewDocument(stealth.JS)
	if err != nil {
		return nil, err
	}

	_, err = p.EvalOnNewDocument(stealth.ExtraJS)
	if err != nil {
		return nil, err
	}

	return p, nil
}
