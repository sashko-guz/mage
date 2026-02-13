package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnv loads environment variables from .env file
// It searches for .env in the current directory and parent directories
// Variables already set in the environment take precedence
func LoadEnv() error {
	// Try to find .env file in current directory or parents
	envPath := findEnvFile()
	if envPath == "" {
		// .env file not found, that's OK - use system env vars
		return nil
	}

	return loadEnvFile(envPath)
}

// findEnvFile searches for .env file starting from current directory
func findEnvFile() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Search up to 5 levels up
	for range 5 {
		envPath := filepath.Join(cwd, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
		// Go to parent directory
		newCwd := filepath.Dir(cwd)
		if newCwd == cwd {
			// Reached root
			break
		}
		cwd = newCwd
	}
	return ""
}

// loadEnvFile reads and parses .env file
func loadEnvFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"'")

		// Only set if not already in environment
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}
