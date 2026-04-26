// Package main demonstrates the CompletionObserver hook for tracking token usage
// and conversation size across multiple chat turns.
//
// This example shows how to:
//   - Wire a CompletionObserver to collect token usage after each API round-trip
//   - Track cumulative prompt and completion tokens across turns
//   - Monitor conversation message count (useful for verifying compactor behaviour)
//   - Structure an observer suitable for feeding a Prometheus counter/gauge
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/m0rjc/goaitools"
	"github.com/m0rjc/goaitools/example/shared"
)

// metrics accumulates token usage across the lifetime of the chat session.
// In a real application these fields would be replaced by Prometheus counters/gauges:
//
//	promptTokensCounter.Add(float64(usage.PromptTokens))
//	completionTokensCounter.Add(float64(usage.CompletionTokens))
//	conversationSizeGauge.Set(float64(messageCount))
type metrics struct {
	totalPromptTokens     atomic.Int64
	totalCompletionTokens atomic.Int64
	apiCalls              atomic.Int64
	lastMessageCount      atomic.Int64
}

func (m *metrics) observe(ctx context.Context, usage *goaitools.TokenUsage, messageCount int) {
	m.apiCalls.Add(1)
	m.lastMessageCount.Store(int64(messageCount))

	if usage != nil {
		m.totalPromptTokens.Add(int64(usage.PromptTokens))
		m.totalCompletionTokens.Add(int64(usage.CompletionTokens))

		fmt.Printf("  [observer] round-trip %d: prompt=%d completion=%d total=%d | conversation messages=%d\n",
			m.apiCalls.Load(),
			usage.PromptTokens,
			usage.CompletionTokens,
			usage.TotalTokens,
			messageCount,
		)
	} else {
		fmt.Printf("  [observer] round-trip %d: (no token data) | conversation messages=%d\n",
			m.apiCalls.Load(),
			messageCount,
		)
	}
}

func (m *metrics) summary() {
	fmt.Println()
	fmt.Println("  Session totals:")
	fmt.Printf("    API round-trips:         %d\n", m.apiCalls.Load())
	fmt.Printf("    Total prompt tokens:     %d\n", m.totalPromptTokens.Load())
	fmt.Printf("    Total completion tokens: %d\n", m.totalCompletionTokens.Load())
	fmt.Printf("    Grand total tokens:      %d\n", m.totalPromptTokens.Load()+m.totalCompletionTokens.Load())
	fmt.Printf("    Final message count:     %d\n", m.lastMessageCount.Load())
}

func main() {
	shared.ReadDotEnv()
	client := shared.CreateOpenAIClient()
	ctx := context.Background()

	m := &metrics{}

	chat := &goaitools.Chat{
		Backend:            client,
		CompletionObserver: m.observe,
		Compactor: &goaitools.MessageLimitCompactor{
			MaxMessages: 6,
		},
	}

	systemPrompt := "You are a helpful assistant. Keep responses to one or two sentences."

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("COMPLETION OBSERVER DEMO")
	fmt.Println("Each API round-trip fires the observer; totals accumulate across turns.")
	fmt.Println(strings.Repeat("=", 80))

	turns := []string{
		"What is the tallest mountain in the world?",
		"How tall is it in feet?",
		"Who first summited it, and in what year?",
		"What nationality were they?",
		"Is it still the record-holders' native country today?",
	}

	var state []byte
	for i, msg := range turns {
		fmt.Printf("\n--- Turn %d ---\n", i+1)
		fmt.Printf("USER: %s\n", msg)

		response, newState, err := chat.ChatWithState(ctx, state,
			goaitools.WithSystemMessage(systemPrompt),
			goaitools.WithUserMessage(msg),
		)
		if err != nil {
			log.Fatalf("turn %d: %v", i+1, err)
		}

		state = newState
		fmt.Printf("ASSISTANT: %s\n", response)

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("OBSERVER SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	m.summary()

	fmt.Println()
	fmt.Println("Key observations:")
	fmt.Println("- The observer fires once per backend round-trip (before compaction).")
	fmt.Println("- Token counts let you track AI spend per assistant / per session.")
	fmt.Println("- Message count lets you verify the compactor is keeping state bounded.")
	fmt.Println("- A nil usage guard is needed: some backends omit token data.")
}
