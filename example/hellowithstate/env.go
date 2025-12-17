package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func readDotEnv() {
	if _, err := os.Stat(".env"); err == nil {
		if err := loadEnv(".env"); err != nil {
			fmt.Printf("Warning: failed to load .env file: %v", err)
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
