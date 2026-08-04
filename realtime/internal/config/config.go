package config

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port         string `envconfig:"PORT" required:"true"`
	Env          string `envconfig:"ENV" default:"development"`
	WSHubTimeout int    `envconfig:"WSHUB_TIMEOUT" default:"5"`
	Secret       string `envconfig:"SECRET" required:"true"`
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config

	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func MustConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return cfg
}
