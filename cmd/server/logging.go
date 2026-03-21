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
	addr := ":" + cfg.Port
	log.Printf("[Server] Server listening on %s", addr)

	if cfg.SignatureSecret != "" {
		log.Printf("[Server] Signature validation: ENABLED (secret key set in SIGNATURE_SECRET env)")
		log.Printf("[Server] Signature config: algo=%s, extract_start=%d, length=%d",
			cfg.SignatureAlgorithm,
			cfg.SignatureStart,
			cfg.SignatureLength,
		)
		log.Printf("[Server] Thumbnail endpoint: http://localhost%s/thumbs/[{signature}/]{size}/[filters:{filters}/]{path}", addr)
	} else {
		log.Printf("[Server] Signature validation: DISABLED")
		log.Printf("[Server] Thumbnail endpoint: http://localhost%s/thumbs/{size}/[filters:{filters}/]{path}", addr)
	}
}
