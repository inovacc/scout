package scout

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBrowserCloseConcurrent(t *testing.T) {
	b := newOwnedTestBrowser(t)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Close()
		}()
	}
	wg.Wait()
}

func TestBrowserCloseNil(t *testing.T) {
	var b *Browser
	require.NoError(t, b.Close())
}
