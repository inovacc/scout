package sdk

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// ConnectBrowser creates a *scout.Browser connected to the CDP endpoint from a BrowserContext.
func ConnectBrowser(bc *BrowserContext) (*scout.Browser, error) {
	if bc == nil {
		return nil, fmt.Errorf("sdk: no browser context provided")
	}

	if bc.CDPEndpoint == "" {
		return nil, fmt.Errorf("sdk: empty CDP endpoint")
	}

	b, err := scout.New(scout.WithRemoteCDP(bc.CDPEndpoint))
	if err != nil {
		return nil, fmt.Errorf("sdk: connect browser: %w", err)
	}

	return b, nil
}
