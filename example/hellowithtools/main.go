// Package main demonstrates the goaitools library with a simple game configuration example.
//
// This program executes a series of predefined operations that use AI tools to read and
// modify game properties. It demonstrates:
//   - Creating and registering AI tools
//   - Using the Chat API with functional options
//   - Logging tool actions for audit and user feedback
//   - Handling tool results and errors
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/m0rjc/goaitools"
	"github.com/m0rjc/goaitools/aitooling"
	"github.com/m0rjc/goaitools/openai"
)

func main() {
	readDotEnv()

	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	// Create game instance
	game := NewGame()

	// Create tools
	tools := aitooling.ToolSet{
		NewReadGameTool(game),
		NewWriteGameTool(game),
	}

	// Create OpenAI client and chat
	client, err := openai.NewClient(apiKey)
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	chat := &goaitools.Chat{
		Backend:          client,
		LogToolArguments: true,
	}

	// Create tool action logger
	logger := &SimpleToolActionLogger{}

	ctx := context.Background()
	systemPrompt := "You are a helpful game administrator assistant. You can read and update game properties to help users set up their game. When asked to change properties, use the write_game tool to update only the specified properties."

	// Test sequence
	testOperations := []struct {
		name    string
		message string
	}{
		{"Ask about the game", "What are the current game settings?"},
		{"Change game title", "Change the game title to 'Epic Adventure Quest'"},
		{"Change start date and duration", "Set the start date to 2024-12-25T10:00:00Z and duration to 90 minutes"},
		{"Change grid dimensions", "Set the grid to 15 rows and 20 columns"},
	}

	for i, op := range testOperations {
		fmt.Printf("\n%s\n", strings.Repeat("=", 80))
		fmt.Printf("Operation %d: %s\n", i+1, op.name)
		fmt.Printf("%s\n", strings.Repeat("=", 80))
		fmt.Printf("USER: %s\n\n", op.message)

		response, err := chat.Chat(ctx,
			goaitools.WithSystemMessage(systemPrompt),
			goaitools.WithUserMessage(op.message),
			goaitools.WithTools(tools),
			goaitools.WithToolActionLogger(logger),
		)

		if err != nil {
			log.Fatalf("Chat error: %v", err)
		}

		logger.PrintAndClear()
		fmt.Printf("\nASSISTANT: %s\n", response)
	}

	// Print final game state
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("FINAL GAME STATE\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("Title:            %s\n", game.Title)
	fmt.Printf("Start Date:       %s\n", game.StartDate.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Duration:         %d minutes\n", game.DurationMinutes)
	fmt.Printf("Grid Dimensions:  %d x %d (M x N)\n", game.GridM, game.GridN)
	fmt.Printf("%s\n", strings.Repeat("=", 80))
}
