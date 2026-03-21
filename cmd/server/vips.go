package main

import (
	"os"
	"strconv"

	"github.com/cshum/vipsgen/vips"
	"github.com/sashko-guz/mage/internal/logger"
)

func configureVips() *vips.Config {
	vipsConcurrency := os.Getenv("VIPS_CONCURRENCY")
	if vipsConcurrency == "" {
		return nil
	}

	conc, err := strconv.Atoi(vipsConcurrency)
	if err != nil || conc <= 0 {
		logger.Warnf("[Server] Ignoring VIPS_CONCURRENCY=%q (must be positive integer)", vipsConcurrency)
		return nil
	}

	logger.Infof("[Server] libvips concurrency set to %d via VIPS_CONCURRENCY", conc)
	return &vips.Config{ConcurrencyLevel: conc}
}
