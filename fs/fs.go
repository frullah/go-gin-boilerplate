package fs

import (
	"github.com/spf13/afero"
)

// FS ...
var FS afero.Fs

// InitAsMemory file system, used for testing
func InitAsMemory() {
	FS = afero.NewMemMapFs()
}

// InitAsOS file system
func InitAsOS() {
	FS = afero.NewOsFs()
}
