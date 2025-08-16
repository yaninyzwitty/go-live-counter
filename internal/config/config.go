package config

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	UserService UserService `yaml:"user-service"`
	Database    Cocroach    `yaml:"database"`
}

type UserService struct {
	Port int `yaml:"port"`
}

type Cocroach struct {
	Username string `yaml:"username"`
	Host     string `yaml:"host"`
	Database string `yaml:"database"`
	Port     int    `yaml:"port"`
}

//  func to load config

func (c *Config) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to load file for path", "path", path, "error", err)
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		slog.Error("failed to unmarshal config file", "error", err, "path", path)
		return err
	}

	return nil
}
