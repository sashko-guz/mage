package main

import (
	"log"
	"os"

	"github.com/sashko-guz/mage/internal/config"
	"github.com/sashko-guz/mage/internal/logger"
)

func setupLogging() {
	logger.SetOutput(os.Stderr)
	logger.SetFlags(log.LstdFlags | log.Lshortfile)
	logger.InitFromEnv()
}

func logServerInfo(cfg *config.Config) {
	addr := ":" + cfg.HTTP.Port
	log.Printf("[Server] Server listening on %s", addr)

	if cfg.Signature.Secret != "" {
		log.Printf("[Server] Signature validation: ENABLED (secret key set in SIGNATURE_SECRET env)")
		log.Printf("[Server] Signature config: algo=%s, extract_start=%d, length=%d",
			cfg.Signature.Algorithm,
			cfg.Signature.Start,
			cfg.Signature.Length,
		)
		log.Printf("[Server] Thumbnail endpoint: http://localhost%s/thumbs/[{signature}/]{size}/[filters:|f:{filters}/]{path}[/as/{alias.ext}]", addr)
	} else {
		log.Printf("[Server] Signature validation: DISABLED")
		log.Printf("[Server] Thumbnail endpoint: http://localhost%s/thumbs/{size}/[filters:|f:{filters}/]{path}[/as/{alias.ext}]", addr)
	}
}
