package config

import (
	"os"
)

type Config struct {
	StoragesConfigPath string
	Port               string
}

func Load() *Config {
	return &Config{
		StoragesConfigPath: getEnv("STORAGE_CONFIG_PATH", "./storages.json"),
		Port:               getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
