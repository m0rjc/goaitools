// Package main demonstrates conversation state compaction using the goaitools library.
//
// This program shows how compaction automatically manages conversation length as it grows,
// preventing unbounded state growth. It demonstrates:
//   - MessageLimitCompactor: Keeps only the last N messages
//   - TokenLimitCompactor: Manages conversation based on token usage
//   - CompositeCompactor: Combines multiple compaction strategies
//   - State size management across multiple conversation turns
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/m0rjc/goaitools"
	"github.com/m0rjc/goaitools/example/shared"
	"github.com/m0rjc/goaitools/openai"
)

func main() {
	shared.ReadDotEnv()
	client := shared.CreateOpenAIClient()
	ctx := context.Background()

	// Run demonstrations
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("STATE COMPACTION DEMONSTRATIONS")
	fmt.Println(strings.Repeat("=", 80))

	// Demo 1: Message limit compaction
	demonstrateMessageLimitCompaction(ctx, client)

	// Demo 2: Token limit compaction
	demonstrateTokenLimitCompaction(ctx, client)

	// Demo 3: Composite compaction (multiple strategies)
	demonstrateCompositeCompaction(ctx, client)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("DEMONSTRATIONS COMPLETE")
	fmt.Println(strings.Repeat("=", 80))
}

// demonstrateMessageLimitCompaction shows how MessageLimitCompactor keeps only the last N messages.
func demonstrateMessageLimitCompaction(ctx context.Context, client *openai.Client) {
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("DEMO 1: Message Limit Compaction")
	fmt.Println("Configuration: Keep maximum 4 messages in state")
	fmt.Println(strings.Repeat("-", 80))

	chat := &goaitools.Chat{
		Backend: client,
		Compactor: &goaitools.MessageLimitCompactor{
			MaxMessages: 4, // Keep only last 4 messages
		},
	}

	var state []byte
	systemPrompt := "You are a helpful assistant discussing world geography. Keep responses to 1-2 sentences."

	// Conversation turns - each adds 2 messages (user + assistant)
	turns := []string{
		"What is the capital of France?",           // Messages: 2
		"What about Germany?",                      // Messages: 4
		"And what's the capital of Italy?",         // Messages: 6 → should compact to ~4
		"Which of these cities is the largest?",    // Messages: 6 → should compact again
		"Tell me about the smallest one.",          // Messages: 6 → should compact again
	}

	for i, userMsg := range turns {
		fmt.Printf("\n--- Turn %d ---\n", i+1)
		fmt.Printf("USER: %s\n", userMsg)

		response, newState, err := chat.ChatWithState(ctx, state,
			goaitools.WithSystemMessage(systemPrompt),
			goaitools.WithUserMessage(userMsg),
		)
		if err != nil {
			log.Fatalf("Turn %d error: %v", i+1, err)
		}

		state = newState
		fmt.Printf("ASSISTANT: %s\n", response)

		// Decode state to count messages (for demonstration purposes)
		messageCount := estimateMessageCount(state)
		fmt.Printf("[State: %d bytes, ~%d messages]\n", len(state), messageCount)

		// Small delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n✓ Observation: Message count stabilized around 4 messages despite 5 turns")
	fmt.Println("  Early messages (France, Germany) were removed to maintain limit")
}

// demonstrateTokenLimitCompaction shows how TokenLimitCompactor manages state based on token usage.
func demonstrateTokenLimitCompaction(ctx context.Context, client *openai.Client) {
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("DEMO 2: Token Limit Compaction")
	fmt.Println("Configuration: Max 1000 tokens, target 750 tokens after compaction")
	fmt.Println(strings.Repeat("-", 80))

	chat := &goaitools.Chat{
		Backend: client,
		Compactor: &goaitools.TokenLimitCompactor{
			MaxTokens:    1000, // Trigger compaction at 1000 tokens
			TargetTokens: 750,  // Compact down to 750 tokens
		},
	}

	var state []byte
	systemPrompt := "You are a helpful assistant discussing ancient civilizations. Provide detailed responses (3-4 sentences) to build up token usage."

	turns := []string{
		"Tell me about Ancient Egypt.",
		"What about Ancient Rome?",
		"How did Ancient Greece contribute to modern society?",
		"What were the major achievements of the Mesopotamian civilization?",
		"Compare the military strategies of these civilizations.",
	}

	for i, userMsg := range turns {
		fmt.Printf("\n--- Turn %d ---\n", i+1)
		fmt.Printf("USER: %s\n", userMsg)

		response, newState, err := chat.ChatWithState(ctx, state,
			goaitools.WithSystemMessage(systemPrompt),
			goaitools.WithUserMessage(userMsg),
		)
		if err != nil {
			log.Fatalf("Turn %d error: %v", i+1, err)
		}

		state = newState
		fmt.Printf("ASSISTANT: %s\n", response)

		messageCount := estimateMessageCount(state)
		fmt.Printf("[State: %d bytes, ~%d messages]\n", len(state), messageCount)

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n✓ Observation: Token-based compaction triggered when usage exceeded 1000 tokens")
	fmt.Println("  Older messages removed to reach target of 750 tokens")
}

// demonstrateCompositeCompaction shows how multiple compaction strategies can work together.
func demonstrateCompositeCompaction(ctx context.Context, client *openai.Client) {
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("DEMO 3: Composite Compaction (Multiple Strategies)")
	fmt.Println("Configuration: Max 6 messages OR max 1500 tokens (whichever triggers first)")
	fmt.Println(strings.Repeat("-", 80))

	chat := &goaitools.Chat{
		Backend: client,
		Compactor: &goaitools.CompositeCompactor{
			Compactors: []goaitools.Compactor{
				&goaitools.MessageLimitCompactor{
					MaxMessages: 6,
				},
				&goaitools.TokenLimitCompactor{
					MaxTokens:    1500,
					TargetTokens: 1000,
				},
			},
		},
	}

	var state []byte
	systemPrompt := "You are a helpful assistant. Keep responses concise (1-2 sentences)."

	turns := []string{
		"What is machine learning?",
		"How does it differ from traditional programming?",
		"What are neural networks?",
		"Explain deep learning.",
		"What is reinforcement learning?",
		"How is ML used in healthcare?",
		"What about autonomous vehicles?",
	}

	for i, userMsg := range turns {
		fmt.Printf("\n--- Turn %d ---\n", i+1)
		fmt.Printf("USER: %s\n", userMsg)

		response, newState, err := chat.ChatWithState(ctx, state,
			goaitools.WithSystemMessage(systemPrompt),
			goaitools.WithUserMessage(userMsg),
		)
		if err != nil {
			log.Fatalf("Turn %d error: %v", i+1, err)
		}

		state = newState
		fmt.Printf("ASSISTANT: %s\n", response)

		messageCount := estimateMessageCount(state)
		fmt.Printf("[State: %d bytes, ~%d messages]\n", len(state), messageCount)

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n✓ Observation: First compactor to trigger (message or token limit) performs compaction")
	fmt.Println("  This provides protection against both long conversations and token-heavy exchanges")
}

// estimateMessageCount provides a rough estimate of messages in state by counting role fields.
// This is a hack for demonstration - real code shouldn't peek inside state.
func estimateMessageCount(state []byte) int {
	if len(state) == 0 {
		return 0
	}
	// Count occurrences of "role" field as proxy for message count
	// This is approximate and for demo purposes only
	count := strings.Count(string(state), `"role"`)
	return count
}
