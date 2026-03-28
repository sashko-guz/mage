package config

import (
	"os"
	"strconv"
	"time"
)

type SignatureConfig struct {
	Secret    string
	Algorithm string
	Start     int
	Length    int
}

type CORSConfig struct {
	AllowOrigin   string
	AllowMethods  string
	AllowHeaders  string
	ExposeHeaders string
	MaxAge        int
}

type HTTPConfig struct {
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

type ResizeConfig struct {
	MaxWidth      int
	MaxHeight     int
	MaxResolution int
	MaxInputSize  int
}

type Config struct {
	StorageConfigPath          string
	CacheControlResponseHeader string
	Signature                  SignatureConfig
	CORS                       CORSConfig
	HTTP                       HTTPConfig
	Resize                     ResizeConfig
}

func Load() *Config {
	maxWidth := getEnvInt("MAX_RESIZE_WIDTH", 5120)
	maxHeight := getEnvInt("MAX_RESIZE_HEIGHT", 5120)

	return &Config{
		StorageConfigPath:          getEnv("STORAGE_CONFIG_PATH", "./storage.json"),
		CacheControlResponseHeader: getEnv("CACHE_CONTROL_RESPONSE_HEADER", "public, max-age=31536000, immutable"),
		Signature: SignatureConfig{
			Secret:    getEnv("SIGNATURE_SECRET", ""),
			Algorithm: getEnv("SIGNATURE_ALGO", "sha256"),
			Start:     getEnvIntMin("SIGNATURE_EXTRACT_START", 0, 0),
			Length:    getEnvIntMin("SIGNATURE_LENGTH", 16, 1),
		},
		CORS: CORSConfig{
			AllowOrigin:   getEnv("CORS_ALLOW_ORIGIN", "*"),
			AllowMethods:  getEnv("CORS_ALLOW_METHODS", "GET, HEAD, OPTIONS"),
			AllowHeaders:  getEnv("CORS_ALLOW_HEADERS", "Origin, Content-Type, Accept, Authorization"),
			ExposeHeaders: getEnv("CORS_EXPOSE_HEADERS", "Content-Type, Content-Length, Cache-Control, X-Mage-Cache"),
			MaxAge:        getEnvInt("CORS_MAX_AGE", 86400),
		},
		HTTP: HTTPConfig{
			Port:              getEnv("PORT", "8080"),
			ReadTimeout:       getEnvDurationSeconds("HTTP_READ_TIMEOUT_SECONDS", 5),
			ReadHeaderTimeout: getEnvDurationSeconds("HTTP_READ_HEADER_TIMEOUT_SECONDS", 2),
			WriteTimeout:      getEnvDurationSeconds("HTTP_WRITE_TIMEOUT_SECONDS", 30),
			IdleTimeout:       getEnvDurationSeconds("HTTP_IDLE_TIMEOUT_SECONDS", 120),
			MaxHeaderBytes:    getEnvInt("HTTP_MAX_HEADER_BYTES", 1<<20),
		},
		Resize: ResizeConfig{
			MaxWidth:      maxWidth,
			MaxHeight:     maxHeight,
			MaxResolution: getEnvInt("MAX_RESIZE_RESOLUTION", maxWidth*maxHeight),
			MaxInputSize:  getEnvInt("MAX_INPUT_IMAGE_SIZE_MB", 64) * 1024 * 1024,
		},
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

func getEnvIntMin(key string, defaultValue int, minValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minValue {
		return defaultValue
	}

	return parsed
}
