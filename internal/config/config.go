package config

import (
	"os"
)

type Config struct {
	StorageConfigPath string
	Port              string
}

func Load() *Config {
	return &Config{
		StorageConfigPath: getEnv("STORAGE_CONFIG_PATH", "./storage.json"),
		Port:              getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
