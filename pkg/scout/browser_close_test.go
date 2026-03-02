package scout

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBrowserCloseConcurrent(t *testing.T) {
	b := newOwnedTestBrowser(t)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_ = b.Close()
		})
	}

	wg.Wait()
}

func TestBrowserCloseNil(t *testing.T) {
	var b *Browser
	require.NoError(t, b.Close())
}
