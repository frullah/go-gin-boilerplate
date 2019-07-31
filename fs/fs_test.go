package fs

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestFS(t *testing.T) {
	InitAsOS()
	_, ok := FS.(*afero.OsFs)
	require.True(t, ok)

	InitAsMemory()
	_, ok = FS.(*afero.MemMapFs)
	require.True(t, ok)
}
