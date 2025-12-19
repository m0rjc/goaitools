// Package shared provides common utilities for goaitools examples.
package shared

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/m0rjc/goaitools/openai"
)

var (
	// Command-line flags
	modelFlag         = flag.String("model", "", "OpenAI model to use (default: gpt-4o-mini)")
	requestParamsFlag = flag.String("request-params", "", "JSON string of request parameters (e.g., '{\"temperature\":0.7,\"max_tokens\":2048}')")
	timeoutFlag       = flag.Duration("timeout", 0, "Timeout")
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
// Supports command-line flags:
//
//	--model: Specify the OpenAI model to use (default: gpt-4o-mini)
//	--request-params: JSON string of request parameters (e.g., '{"temperature":0.7,"max_tokens":2048}')
//	--timeout: HTTP timeout in seconds
func CreateOpenAIClient() *openai.Client {
	// Parse command-line flags
	flag.Parse()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	// Build options list
	var opts []openai.ClientOption

	// Add model option if specified
	if *modelFlag != "" {
		opts = append(opts, openai.WithModel(*modelFlag))
	}

	// Parse and add request parameters if specified
	if *requestParamsFlag != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(*requestParamsFlag), &params); err != nil {
			log.Fatalf("Failed to parse request-params JSON: %v", err)
		}
		opts = append(opts, openai.WithRequestParams(params))
	}

	if *timeoutFlag > 0 {
		opts = append(opts, openai.WithHTTPClient(&http.Client{
			Timeout: *timeoutFlag,
		}))
	}

	// Create client with options (or default if no options)
	var client *openai.Client
	var err error

	if len(opts) > 0 {
		client, err = openai.NewClientWithOptions(apiKey, opts...)
	} else {
		client, err = openai.NewClient(apiKey)
	}

	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	return client
}
