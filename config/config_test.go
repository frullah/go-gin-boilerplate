package config

import (
	"os"
	"testing"

	"github.com/frullah/gin-boilerplate/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	fs.InitAsMemory()
}

func TestRead(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		assert.Panics(t, Init)
	})

	t.Run("parse error", func(t *testing.T) {
		file, _ := fs.FS.OpenFile(configFileName, os.O_CREATE|os.O_WRONLY, 0750)
		file.Close()
		assert.Panics(t, Init)
	})

	t.Run("success", func(t *testing.T) {
		content := `
server:
  host: localhost
  port: 8080
db:
- type: mysql
  dsn: user:password@tcp(127.0.0.1:3306)/dbname`
		file, _ := fs.FS.OpenFile(configFileName, os.O_WRONLY, 0750)
		file.Write([]byte(content))
		file.Close()
		require.NotPanics(t, Init)
		assert.NotNil(t, Get())
	})
}
