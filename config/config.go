package config

import (
	"os"

	"github.com/BurntSushi/toml"

	"github.com/frullah/gin-boilerplate/fs"
)

// Config struct
type Config struct {
	Server struct {
		Host string
		Port uint16
	}
	DB []struct {
		Name    string
		Type    string
		DSN     string
		Logging bool
	}
}

const configFileName = "config.toml"
const defaultPort = uint16(3000)

var (
	config *Config
)

// Init config from config.yaml
func Init() error {
	file, err := fs.FS.OpenFile(configFileName, os.O_RDONLY, 0750)
	if err != nil {
		return err
	}
	defer file.Close()

	config = &Config{}
	_, err = toml.DecodeReader(file, &config)
	if err != nil {
		return err
	}

	if config.Server.Port == 0 {
		config.Server.Port = defaultPort
	}

	return nil
}

// Get config
func Get() *Config {
	return config
}
