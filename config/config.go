package config

import (
	"bufio"
	"os"

	"github.com/frullah/gin-boilerplate/fs"
	"gopkg.in/yaml.v3"
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

const configFileName = "config.yaml"

var (
	config *Config
)

// Init config from config.yaml
func Init() {
	file, err := fs.FS.OpenFile(configFileName, os.O_RDONLY, 0750)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	config = &Config{}
	reader := bufio.NewReader(file)
	decoder := yaml.NewDecoder(reader)
	err = decoder.Decode(config)
	if err != nil {
		panic(err)
	}
}

// Get ...
func Get() *Config {
	return config
}
