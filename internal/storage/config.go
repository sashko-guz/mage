package storage

import (
	"encoding/json"
	"fmt"
	"os"
)

type StorageDriver string

const (
	DriverS3    StorageDriver = "s3"
	DriverLocal StorageDriver = "local"
)

type StorageItem struct {
	Name   string        `json:"name"`
	Driver StorageDriver `json:"driver"`

	// Cache configuration overrides
	Cache *StorageCacheConfig `json:"cache,omitempty"`

	// Signature secret key for HMAC signature validation (optional, per-storage)
	SignatureSecretKey string `json:"signature_secret_key,omitempty"`

	// S3 specific fields
	Bucket    string `json:"bucket,omitempty"`
	Region    string `json:"region,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	BaseURL   string `json:"base_url,omitempty"` // Custom endpoint for S3-compatible storage

	// S3 HTTP client tuning (optional, with sensible defaults)
	S3HTTPConfig *S3HTTPConfig `json:"s3_http_config,omitempty"`

	// Local specific fields
	Root string `json:"root,omitempty"`
}

// S3HTTPConfig contains HTTP client configuration for S3 connections
type S3HTTPConfig struct {
	MaxIdleConns          int `json:"max_idle_conns,omitempty"`              // Max idle connections across all hosts (default: 100)
	MaxIdleConnsPerHost   int `json:"max_idle_conns_per_host,omitempty"`     // Max idle connections per host (default: 100)
	MaxConnsPerHost       int `json:"max_conns_per_host,omitempty"`          // Max total connections per host (default: 0 = unlimited)
	IdleConnTimeout       int `json:"idle_conn_timeout_sec,omitempty"`       // Idle connection timeout in seconds (default: 90)
	ConnectTimeout        int `json:"connect_timeout_sec,omitempty"`         // Connection timeout in seconds (default: 10)
	RequestTimeout        int `json:"request_timeout_sec,omitempty"`         // Full request timeout in seconds (default: 30)
	ResponseHeaderTimeout int `json:"response_header_timeout_sec,omitempty"` // Response header timeout in seconds (default: 10)
}

// MemoryCacheOptions defines configuration for in-memory cache
// This cache stores both source images and generated thumbnails with a unified size limit
type MemoryCacheOptions struct {
	Enabled   *bool `json:"enabled,omitempty"`
	MaxSizeMB int   `json:"max_size_mb,omitempty"` // Total memory limit for sources + thumbnails
	MaxItems  int   `json:"max_items,omitempty"`   // Maximum number of cached items (sources + thumbnails)
}

// DiskCacheOptions defines configuration for disk-based cache
// This cache stores both source images and generated thumbnails with a unified size limit
type DiskCacheOptions struct {
	Enabled        *bool  `json:"enabled,omitempty"`
	TTLSeconds     int    `json:"ttl_seconds,omitempty"`
	MaxSizeMB      int    `json:"max_size_mb,omitempty"` // Total disk limit for sources + thumbnails (0 = unlimited)
	Dir            string `json:"dir,omitempty"`
	ClearOnStartup *bool  `json:"clear_on_startup,omitempty"`
}

type StorageCacheConfig struct {
	Memory *MemoryCacheOptions `json:"memory,omitempty"`
	Disk   *DiskCacheOptions   `json:"disk,omitempty"`
}

type StorageConfig struct {
	Storages []StorageItem `json:"storages"`
}

func LoadStorageConfig(configPath string) (*StorageConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config StorageConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}

func InitializeStorages(config *StorageConfig) (map[string]Storage, error) {
	storages := make(map[string]Storage)

	for _, cfg := range config.Storages {
		var store Storage
		var err error

		switch cfg.Driver {
		case DriverS3:
			if cfg.Bucket == "" {
				return nil, fmt.Errorf("storage '%s': bucket is required for S3 driver", cfg.Name)
			}
			if cfg.Region == "" {
				cfg.Region = "us-east-1" // default region
			}
			// Require credentials when using custom base_url (S3-compatible storage)
			if cfg.BaseURL != "" && (cfg.AccessKey == "" || cfg.SecretKey == "") {
				return nil, fmt.Errorf("storage '%s': access_key and secret_key are required when using base_url for S3-compatible storage", cfg.Name)
			}
			store, err = NewS3Client(cfg.Region, cfg.AccessKey, cfg.SecretKey, cfg.Bucket, cfg.BaseURL, cfg.S3HTTPConfig)
			if err != nil {
				return nil, fmt.Errorf("storage '%s': failed to initialize S3: %w", cfg.Name, err)
			}

		case DriverLocal:
			if cfg.Root == "" {
				return nil, fmt.Errorf("storage '%s': root is required for local driver", cfg.Name)
			}
			store, err = NewLocalStorage(cfg.Root)
			if err != nil {
				return nil, fmt.Errorf("storage '%s': failed to initialize local storage: %w", cfg.Name, err)
			}

		default:
			return nil, fmt.Errorf("storage '%s': unknown driver '%s'", cfg.Name, cfg.Driver)
		}

		storages[cfg.Name] = store
	}

	if len(storages) == 0 {
		return nil, fmt.Errorf("no storages configured")
	}

	return storages, nil
}
