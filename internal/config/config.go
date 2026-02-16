package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	StorageConfigPath string
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

func Load() *Config {
	return &Config{
		StorageConfigPath: getEnv("STORAGE_CONFIG_PATH", "./storage.json"),
		Port:              getEnv("PORT", "8080"),
		ReadTimeout:       getEnvDurationSeconds("HTTP_READ_TIMEOUT_SECONDS", 5),
		ReadHeaderTimeout: getEnvDurationSeconds("HTTP_READ_HEADER_TIMEOUT_SECONDS", 2),
		WriteTimeout:      getEnvDurationSeconds("HTTP_WRITE_TIMEOUT_SECONDS", 30),
		IdleTimeout:       getEnvDurationSeconds("HTTP_IDLE_TIMEOUT_SECONDS", 120),
		MaxHeaderBytes:    getEnvInt("HTTP_MAX_HEADER_BYTES", 1<<20),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}

	return parsed
}

func getEnvDurationSeconds(key string, defaultSeconds int) time.Duration {
	return time.Duration(getEnvInt(key, defaultSeconds)) * time.Second
}
