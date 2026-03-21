package main

import (
	"fmt"

	"github.com/sashko-guz/mage/internal/storage"
	storageDrivers "github.com/sashko-guz/mage/internal/storage/drivers"
)

func initializeStorage(configPath string) (storageDrivers.Storage, error) {
	storageConfig, err := storage.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load storage config: %w", err)
	}

	stor, err := storage.NewStorage(storageConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return stor, nil
}
