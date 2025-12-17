// Package shared provides common utilities for goaitools examples.
package shared

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/m0rjc/goaitools/openai"
)

// ReadDotEnv loads environment variables from a .env file if it exists.
// This is a simple implementation for demo purposes - not production quality.
// Silently skips if .env file doesn't exist.
func ReadDotEnv() {
	if _, err := os.Stat(".env"); err == nil {
		if err := loadEnv(".env"); err != nil {
			fmt.Printf("Warning: failed to load .env file: %v\n", err)
		}
	}
}

// loadEnv loads environment variables from a .env file.
// Lines starting with # are treated as comments.
// Each line should be in KEY=VALUE format.
func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

// CreateOpenAIClient creates an OpenAI client from the OPENAI_API_KEY environment variable.
// Calls log.Fatal if the API key is not set or client creation fails.
func CreateOpenAIClient() *openai.Client {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	client, err := openai.NewClient(apiKey)
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	return client
}
