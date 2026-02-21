package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	StorageConfigPath   string
	SignatureSecret     string
	Port                string
	ReadTimeout         time.Duration
	ReadHeaderTimeout   time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	MaxHeaderBytes      int
	MaxResizeWidth      int
	MaxResizeHeight     int
	MaxResizeResolution int
}

func Load() *Config {
	maxResizeWidth := getEnvInt("MAX_RESIZE_WIDTH", 5120)
	maxResizeHeight := getEnvInt("MAX_RESIZE_HEIGHT", 5120)
	maxResizeResolution := getEnvInt("MAX_RESIZE_RESOLUTION", maxResizeWidth*maxResizeHeight)

	return &Config{
		StorageConfigPath:   getEnv("STORAGE_CONFIG_PATH", "./storage.json"),
		SignatureSecret:     getEnv("SIGNATURE_SECRET", ""),
		Port:                getEnv("PORT", "8080"),
		ReadTimeout:         getEnvDurationSeconds("HTTP_READ_TIMEOUT_SECONDS", 5),
		ReadHeaderTimeout:   getEnvDurationSeconds("HTTP_READ_HEADER_TIMEOUT_SECONDS", 2),
		WriteTimeout:        getEnvDurationSeconds("HTTP_WRITE_TIMEOUT_SECONDS", 30),
		IdleTimeout:         getEnvDurationSeconds("HTTP_IDLE_TIMEOUT_SECONDS", 120),
		MaxHeaderBytes:      getEnvInt("HTTP_MAX_HEADER_BYTES", 1<<20),
		MaxResizeWidth:      maxResizeWidth,
		MaxResizeHeight:     maxResizeHeight,
		MaxResizeResolution: maxResizeResolution,
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
