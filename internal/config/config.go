package config

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	UserService Service     `yaml:"user-service"`
	LikeService Service     `yaml:"like-service"`
	PostService Service     `yaml:"post-service"`
	Websocket   Service     `yaml:"websocket-service"`
	Outbox      Service     `yaml:"outbox-service"`
	Database    Cocroach    `yaml:"database"`
	Queue       PulsarQueue `yaml:"queue"`
}

// use service instead of juggling over all of them only have port as the field
type Service struct {
	Port int `yaml:"port"`
}

type PulsarQueue struct {
	Url              string `yaml:"url"`
	TopicName        string `yaml:"topic_name"`
	SubscriptionName string `yaml:"subscription_name"`
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
		return err
	}

	if err := yaml.Unmarshal(data, c); err != nil {

		slog.Error("failed to unmarshal config file", "error", err, "path", path)
		return err
	}

	return nil
}
