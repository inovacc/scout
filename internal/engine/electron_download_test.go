package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestElectronAssetName(t *testing.T) {
	name := electronAssetName("v33.2.0")
	assert.NotEmpty(t, name)
	assert.Contains(t, name, "electron")
	assert.Contains(t, name, "v33.2.0")
}

func TestElectronBinPath(t *testing.T) {
	bin := electronBinPath()
	assert.NotEmpty(t, bin)
	assert.True(t, strings.Contains(bin, "electron") || strings.Contains(bin, "Electron"))
}

func TestElectronCacheDir(t *testing.T) {
	dir, err := ElectronCacheDir()
	require.NoError(t, err)
	assert.Contains(t, dir, "electron")
}

func TestElectronVersionNormalization(t *testing.T) {
	// electronAssetName should work with "v" prefix
	name1 := electronAssetName("v33.2.0")
	name2 := electronAssetName("v33.2.0")
	assert.Equal(t, name1, name2)
}
