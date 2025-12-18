// Package main demonstrates stateful conversations using the goaitools library.
//
// This program shows how to maintain conversation state across multiple turns without
// using tools. It demonstrates:
//   - Using ChatWithState() for multi-turn conversations
//   - Passing state between conversation turns
//   - System messages being provided on each call (not stored in state)
//   - AppendToState() to add context without API calls
//   - State persistence and continuity
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/m0rjc/goaitools"
	"github.com/m0rjc/goaitools/example/shared"
)

func main() {
	shared.ReadDotEnv()
	client := shared.CreateOpenAIClient()

	chat := &goaitools.Chat{
		Backend:          client,
		LogToolArguments: true,
	}

	ctx := context.Background()

	// System message includes current time - demonstrates why system messages
	// aren't stored in state (they can be dynamic)
	getSystemPrompt := func() string {
		return fmt.Sprintf("You are a helpful travel planning assistant. Help users plan their vacation. Keep responses concise and friendly. Current date/time: %s",
			time.Now().Format("2006-01-02 15:04:05 MST"))
	}

	// Simulate a multi-turn conversation
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("STATEFUL CONVERSATION DEMO - Travel Planning Assistant")
	fmt.Println(strings.Repeat("=", 80))

	var state []byte

	// Turn 1: Initial request
	fmt.Println("\n--- Turn 1 ---")
	fmt.Println("USER: I want to plan a vacation to Japan in spring")
	response, newState, err := chat.ChatWithState(ctx, state,
		goaitools.WithSystemMessage(getSystemPrompt()),
		goaitools.WithUserMessage("I want to plan a vacation to Japan in spring"),
	)
	if err != nil {
		log.Fatalf("Turn 1 error: %v", err)
	}
	state = newState
	fmt.Printf("ASSISTANT: %s\n", response)
	fmt.Printf("[State size: %d bytes]\n", len(state))

	// Turn 2: Follow-up question - AI should remember we're talking about Japan
	fmt.Println("\n--- Turn 2 ---")
	fmt.Println("USER: What are the best cities to visit?")
	response, newState, err = chat.ChatWithState(ctx, state,
		goaitools.WithSystemMessage(getSystemPrompt()),
		goaitools.WithUserMessage("What are the best cities to visit?"),
	)
	if err != nil {
		log.Fatalf("Turn 2 error: %v", err)
	}
	state = newState
	fmt.Printf("ASSISTANT: %s\n", response)
	fmt.Printf("[State size: %d bytes]\n", len(state))

	// Turn 3: Add context via AppendToState (no API call)
	fmt.Println("\n--- Turn 3 ---")
	fmt.Println("[EVENT: User clicked on 'Kyoto' in the UI]")
	state = chat.AppendToState(ctx, state, goaitools.WithUserMessage("User expressed interest in Kyoto by selecting it"))
	fmt.Printf("[State updated without API call, size: %d bytes]\n", len(state))

	// Turn 4: Continue conversation - AI should know we're interested in Kyoto
	fmt.Println("\n--- Turn 4 ---")
	fmt.Println("USER: Tell me more about this city")
	response, newState, err = chat.ChatWithState(ctx, state,
		goaitools.WithSystemMessage(getSystemPrompt()),
		goaitools.WithUserMessage("Tell me more about this city"),
	)
	if err != nil {
		log.Fatalf("Turn 4 error: %v", err)
	}
	state = newState
	fmt.Printf("ASSISTANT: %s\n", response)
	fmt.Printf("[State size: %d bytes]\n", len(state))

	// Turn 5: Ask about something specific
	fmt.Println("\n--- Turn 5 ---")
	fmt.Println("USER: What's the best time to see cherry blossoms there?")
	response, newState, err = chat.ChatWithState(ctx, state,
		goaitools.WithSystemMessage(getSystemPrompt()),
		goaitools.WithUserMessage("What's the best time to see cherry blossoms there?"),
	)
	if err != nil {
		log.Fatalf("Turn 5 error: %v", err)
	}
	state = newState
	fmt.Printf("ASSISTANT: %s\n", response)
	fmt.Printf("[State size: %d bytes]\n", len(state))

	// Demonstrate that system message is not in state by changing it
	fmt.Println("\n--- Turn 6 (with different system message) ---")
	fmt.Println("USER: Thanks for your help!")
	response, newState, err = chat.ChatWithState(ctx, state,
		goaitools.WithSystemMessage("You are a cheerful assistant who loves to use enthusiasm. Current time: " + time.Now().Format("15:04:05")),
		goaitools.WithUserMessage("Thanks for your help!"),
	)
	if err != nil {
		log.Fatalf("Turn 6 error: %v", err)
	}
	state = newState
	fmt.Printf("ASSISTANT: %s\n", response)
	fmt.Printf("[State size: %d bytes]\n", len(state))

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("DEMONSTRATION COMPLETE")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\nKey Observations:")
	fmt.Println("- State is preserved between turns (AI remembers Japan, Kyoto, cherry blossoms)")
	fmt.Println("- System message is passed on each call (not stored in state)")
	fmt.Println("- AppendToState() adds context without making an API call")
	fmt.Println("- Changing system message affects AI behavior (turn 6 is more enthusiastic)")
	fmt.Printf("- Final conversation state: %d bytes\n", len(state))
}
