package drivers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	log.Printf("[Local storage] Initializing local storage with base path: %s", basePath)

	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base path: %w", err)
	}

	if err := os.MkdirAll(absBasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		basePath: absBasePath,
	}, nil
}

func (l *LocalStorage) GetObject(ctx context.Context, key string) ([]byte, error) {
	cleanPath := filepath.Clean(key)

	if filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, "..") {
		return nil, fmt.Errorf("invalid path: absolute paths and parent references not allowed")
	}

	fullPath := filepath.Join(l.basePath, cleanPath)
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	basePathWithSep := l.basePath
	if !strings.HasSuffix(basePathWithSep, string(filepath.Separator)) {
		basePathWithSep += string(filepath.Separator)
	}
	absFullPathWithSep := absFullPath
	if !strings.HasSuffix(absFullPathWithSep, string(filepath.Separator)) {
		absFullPathWithSep += string(filepath.Separator)
	}

	if !strings.HasPrefix(absFullPathWithSep, basePathWithSep) && absFullPath != l.basePath {
		return nil, fmt.Errorf("invalid path: directory traversal detected")
	}

	// Check if file exists and is accessible
	fileInfo, err := os.Stat(absFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[LocalStorage] file not found: %s", absFullPath)
			return nil, fmt.Errorf("file not found: %s", key)
		}
		if os.IsPermission(err) {
			log.Printf("[LocalStorage] permission denied: %s", absFullPath)
			return nil, fmt.Errorf("permission denied for file: %s", key)
		}
		log.Printf("[LocalStorage] failed to access file %s: %v", absFullPath, err)
		return nil, fmt.Errorf("failed to access file: %s", key)
	}

	// Ensure it's a regular file, not a directory
	if fileInfo.IsDir() {
		log.Printf("[LocalStorage] path is a directory: %s", absFullPath)
		return nil, fmt.Errorf("path is a directory, not a file: %s", key)
	}

	data, err := os.ReadFile(absFullPath)
	if err != nil {
		if os.IsPermission(err) {
			log.Printf("[LocalStorage] permission denied reading file: %s", absFullPath)
			return nil, fmt.Errorf("permission denied reading file: %s", key)
		}
		log.Printf("[LocalStorage] failed to read file %s: %v", absFullPath, err)
		return nil, fmt.Errorf("failed to read file: %s", key)
	}

	return data, nil
}
