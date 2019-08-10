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
	t.Run("handle file not found", func(t *testing.T) {
		assert.Error(t, Init())
	})

	t.Run("handle format error", func(t *testing.T) {
		file, _ := fs.FS.OpenFile(configFileName, os.O_CREATE|os.O_WRONLY, 0750)
		file.WriteString("[key")
		file.Close()
		assert.Error(t, Init())
	})

	t.Run("set to default value", func(t *testing.T) {
		file, _ := fs.FS.OpenFile(configFileName, os.O_CREATE|os.O_WRONLY, 0750)
		file.WriteString("[server]")
		file.Close()
		require.NoError(t, Init())
		assert.Equal(t, config.Server.Port, defaultPort)

	})

	t.Run("success", func(t *testing.T) {
		content := `
[server]
host = "localhost"
port = 3000

[[db]]
name = "default"
type = "mysql"
dsn = "root:frullah-cat-eat@/getting_started"
logging = true`
		file, _ := fs.FS.OpenFile(configFileName, os.O_CREATE|os.O_WRONLY, 0750)
		file.WriteString(content)
		file.Close()
		require.NoError(t, Init())
		assert.NotNil(t, Get())
	})
}
