package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/sashko-guz/mage/internal/app"
	"github.com/sashko-guz/mage/internal/config"
	"github.com/sashko-guz/mage/internal/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	logger.SetOutput(os.Stderr)
	logger.SetFlags(log.LstdFlags | log.Lshortfile)
	logger.InitFromEnv()

	cfg := config.Load()
	application := app.New(cfg)

	if err := application.Run(); err != nil {
		logger.Fatalf("[Main] %v", err)
	}
}
