package config

import (
	"os"
)

type Config struct {
	HTTPPort string `env:"PORT" envDefault:"8080"`
	APIKey   string `env:"API_KEY"`
}

// todo properly load env variables
func NewConfigFromEnv() (Config, error) {
	return Config{
		HTTPPort: os.Getenv("PORT"),
		APIKey:   os.Getenv("API_KEY"),
	}, nil
}
