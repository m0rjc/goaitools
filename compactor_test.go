package goaitools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/m0rjc/goaitools/aitooling"
)

// Test: advanceToFirstUserMessage removes non-user messages from start
func TestAdvanceToFirstUserMessage(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     int // Expected number of messages after advancement
	}{
		{
			name: "starts_with_user_message",
			messages: []Message{
				&mockMessage{role: RoleUser, content: "user1"},
				&mockMessage{role: RoleAssistant, content: "assistant1"},
			},
			want: 2,
		},
		{
			name: "assistant_then_user",
			messages: []Message{
				&mockMessage{role: RoleAssistant, content: "assistant1"},
				&mockMessage{role: RoleUser, content: "user1"},
				&mockMessage{role: RoleAssistant, content: "assistant2"},
			},
			want: 2,
		},
		{
			name: "tool_then_user",
			messages: []Message{
				&mockMessage{role: RoleTool, content: "tool1"},
				&mockMessage{role: RoleUser, content: "user1"},
			},
			want: 1,
		},
		{
			name: "no_user_messages",
			messages: []Message{
				&mockMessage{role: RoleAssistant, content: "assistant1"},
				&mockMessage{role: RoleAssistant, content: "assistant2"},
			},
			want: 0,
		},
		{
			name:     "empty_slice",
			messages: []Message{},
			want:     0,
		},
		{
			name:     "nil_slice",
			messages: nil,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AdvanceToFirstUserMessage(tt.messages)
			if result == nil && tt.want != 0 {
				t.Errorf("Expected %d messages, got nil", tt.want)
				return
			}
			if result == nil && tt.want == 0 {
				return // Correct
			}
			if len(result) != tt.want {
				t.Errorf("Expected %d messages, got %d", tt.want, len(result))
			}
			// Verify first message is a user message (if any)
			if len(result) > 0 && result[0].Role() != RoleUser {
				t.Errorf("First message should be user, got %s", result[0].Role())
			}
		})
	}
}

// Test: MessageLimitCompactor with messages under limit
func TestMessageLimitCompactor_UnderLimit(t *testing.T) {
	compactor := &MessageLimitCompactor{MaxMessages: 5}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
		&mockMessage{role: RoleUser, content: "user2"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		Backend:       &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if response.WasCompacted {
		t.Error("Should not compact when under limit")
	}
	if len(response.StateMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: MessageLimitCompactor with messages over limit
func TestMessageLimitCompactor_OverLimit(t *testing.T) {
	compactor := &MessageLimitCompactor{MaxMessages: 3}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
		&mockMessage{role: RoleUser, content: "user2"},
		&mockMessage{role: RoleAssistant, content: "assistant2"},
		&mockMessage{role: RoleUser, content: "user3"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		Backend:       &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !response.WasCompacted {
		t.Error("Should compact when over limit")
	}
	if len(response.StateMessages) >= len(messages) {
		t.Errorf("Expected fewer than %d messages, got %d", len(messages), len(response.StateMessages))
	}
	// Result should start with a user message
	if len(response.StateMessages) > 0 && response.StateMessages[0].Role() != RoleUser {
		t.Errorf("First message after compaction should be user, got %s", response.StateMessages[0].Role())
	}
}

// Test: MessageLimitCompactor with zero limit (disabled)
func TestMessageLimitCompactor_ZeroLimit(t *testing.T) {
	compactor := &MessageLimitCompactor{MaxMessages: 0}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		Backend:       &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if response.WasCompacted {
		t.Error("Should not compact when limit is 0 (disabled)")
	}
	if len(response.StateMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: MessageLimitCompactor advances to user message boundary
func TestMessageLimitCompactor_UserMessageBoundary(t *testing.T) {
	compactor := &MessageLimitCompactor{MaxMessages: 3}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
		&mockMessage{role: RoleTool, content: "tool1"}, // Should be removed
		&mockMessage{role: RoleUser, content: "user2"}, // First user message boundary
		&mockMessage{role: RoleAssistant, content: "assistant2"},
		&mockMessage{role: RoleUser, content: "user3"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		Backend:       &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !response.WasCompacted {
		t.Error("Should compact when over limit")
	}
	// Should start at user2 (first user message after truncation)
	if len(response.StateMessages) > 0 && response.StateMessages[0].Role() != RoleUser {
		t.Errorf("First message should be user, got %s", response.StateMessages[0].Role())
	}
	if len(response.StateMessages) > 0 && response.StateMessages[0].Content() != "user2" {
		t.Errorf("Should advance to user2, got %s", response.StateMessages[0].Content())
	}
}

// Test: TokenLimitCompactor without token usage (no-op)
func TestTokenLimitCompactor_NoTokenUsage(t *testing.T) {
	compactor := &TokenLimitCompactor{MaxTokens: 1000}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage:  nil, // No token usage available
		Backend:       &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if response.WasCompacted {
		t.Error("Should not compact without token usage information")
	}
	if len(response.StateMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: TokenLimitCompactor under token limit
func TestTokenLimitCompactor_UnderLimit(t *testing.T) {
	compactor := &TokenLimitCompactor{MaxTokens: 1000}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage: &TokenUsage{
			PromptTokens:     500,
			CompletionTokens: 100,
			TotalTokens:      600,
		},
		Backend: &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if response.WasCompacted {
		t.Error("Should not compact when under token limit")
	}
	if len(response.StateMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: TokenLimitCompactor over token limit
func TestTokenLimitCompactor_OverLimit(t *testing.T) {
	compactor := &TokenLimitCompactor{MaxTokens: 1000}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
		&mockMessage{role: RoleUser, content: "user2"},
		&mockMessage{role: RoleAssistant, content: "assistant2"},
		&mockMessage{role: RoleUser, content: "user3"},
		&mockMessage{role: RoleAssistant, content: "assistant3"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage: &TokenUsage{
			PromptTokens:     1500,
			CompletionTokens: 200,
			TotalTokens:      1700,
		},
		Backend: &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !response.WasCompacted {
		t.Error("Should compact when over token limit")
	}
	if len(response.StateMessages) >= len(messages) {
		t.Errorf("Expected fewer than %d messages, got %d", len(messages), len(response.StateMessages))
	}
	// Result should start with a user message
	if len(response.StateMessages) > 0 && response.StateMessages[0].Role() != RoleUser {
		t.Errorf("First message after compaction should be user, got %s", response.StateMessages[0].Role())
	}
}

// Test: TokenLimitCompactor with custom target
func TestTokenLimitCompactor_CustomTarget(t *testing.T) {
	compactor := &TokenLimitCompactor{
		MaxTokens:    1000,
		TargetTokens: 600,
	}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
		&mockMessage{role: RoleUser, content: "user2"},
		&mockMessage{role: RoleAssistant, content: "assistant2"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage: &TokenUsage{
			PromptTokens:     1200,
			CompletionTokens: 100,
			TotalTokens:      1300,
		},
		Backend: &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !response.WasCompacted {
		t.Error("Should compact when over token limit")
	}
	// Should have removed some messages
	if len(response.StateMessages) >= len(messages) {
		t.Errorf("Expected fewer than %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: CompositeCompactor with message limit triggered
func TestCompositeCompactor_MessageLimitTriggered(t *testing.T) {
	compactor := &CompositeCompactor{
		Compactors: []Compactor{
			&MessageLimitCompactor{MaxMessages: 3},
			&TokenLimitCompactor{MaxTokens: 1000},
		},
	}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
		&mockMessage{role: RoleUser, content: "user2"},
		&mockMessage{role: RoleAssistant, content: "assistant2"},
		&mockMessage{role: RoleUser, content: "user3"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage: &TokenUsage{
			PromptTokens:     500, // Under token limit
			CompletionTokens: 100,
			TotalTokens:      600,
		},
		Backend: &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !response.WasCompacted {
		t.Error("Should compact when message limit exceeded")
	}
	if len(response.StateMessages) >= len(messages) {
		t.Errorf("Expected fewer than %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: CompositeCompactor with token limit triggered
func TestCompositeCompactor_TokenLimitTriggered(t *testing.T) {
	compactor := &CompositeCompactor{
		Compactors: []Compactor{
			&MessageLimitCompactor{MaxMessages: 10},
			&TokenLimitCompactor{MaxTokens: 1000},
		},
	}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
		&mockMessage{role: RoleUser, content: "user2"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage: &TokenUsage{
			PromptTokens:     1500, // Over token limit
			CompletionTokens: 200,
			TotalTokens:      1700,
		},
		Backend: &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !response.WasCompacted {
		t.Error("Should compact when token limit exceeded")
	}
	if len(response.StateMessages) >= len(messages) {
		t.Errorf("Expected fewer than %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: CompositeCompactor with neither limit triggered
func TestCompositeCompactor_NoLimitTriggered(t *testing.T) {
	compactor := &CompositeCompactor{
		Compactors: []Compactor{
			&MessageLimitCompactor{MaxMessages: 10},
			&TokenLimitCompactor{MaxTokens: 2000},
		},
	}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage: &TokenUsage{
			PromptTokens:     500,
			CompletionTokens: 100,
			TotalTokens:      600,
		},
		Backend: &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if response.WasCompacted {
		t.Error("Should not compact when neither limit exceeded")
	}
	if len(response.StateMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: CompositeCompactor with both limits disabled (0)
func TestCompositeCompactor_BothDisabled(t *testing.T) {
	compactor := &CompositeCompactor{
		Compactors: []Compactor{
			&MessageLimitCompactor{MaxMessages: 0},
			&TokenLimitCompactor{MaxTokens: 0},
		},
	}

	messages := []Message{
		&mockMessage{role: RoleUser, content: "user1"},
		&mockMessage{role: RoleAssistant, content: "assistant1"},
	}

	req := &CompactionRequest{
		StateMessages: messages,
		LastAPIUsage: &TokenUsage{
			PromptTokens:     5000,
			CompletionTokens: 1000,
			TotalTokens:      6000,
		},
		Backend: &mockBackend{},
	}

	response, err := compactor.Compact(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if response.WasCompacted {
		t.Error("Should not compact when both limits are disabled (0)")
	}
	if len(response.StateMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(response.StateMessages))
	}
}

// Test: Compactor interface contract
func TestCompactor_InterfaceContract(t *testing.T) {
	// Verify all compactors implement the Compactor interface
	var _ Compactor = &MessageLimitCompactor{}
	var _ Compactor = &TokenLimitCompactor{}
	var _ Compactor = &CompositeCompactor{}
}

// Test: Integration - Compaction triggered during ChatWithState
func TestChat_CompactionIntegration_MessageLimit(t *testing.T) {
	turnCount := 0
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			turnCount++
			return &ChatResponse{
				Message: &mockMessage{
					role:    RoleAssistant,
					content: "Response " + string(rune('A'+turnCount-1)),
				},
				FinishReason: FinishReasonStop,
				Usage: &TokenUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			}, nil
		},
	}

	chat := &Chat{
		Backend: backend,
		Compactor: &MessageLimitCompactor{
			MaxMessages: 4, // Keep only 4 messages
		},
	}

	ctx := context.Background()
	var state ConversationState

	// Turn 1: 2 messages (user + assistant)
	_, state, err := chat.ChatWithState(ctx, state,
		WithUserMessage("Question 1"),
	)
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}

	// Turn 2: 4 messages total
	_, state, err = chat.ChatWithState(ctx, state,
		WithUserMessage("Question 2"),
	)
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}

	// Turn 3: Would be 6 messages, should compact to ~4
	_, state, err = chat.ChatWithState(ctx, state,
		WithUserMessage("Question 3"),
	)
	if err != nil {
		t.Fatalf("Turn 3 failed: %v", err)
	}

	// Decode state to verify compaction occurred
	messages, _ := chat.decodeState(ctx, state)
	if messages == nil {
		t.Fatal("State should contain messages after compaction")
	}

	// Should have compacted - expecting 4 messages or fewer
	if len(messages) > 4 {
		t.Errorf("Expected <= 4 messages after compaction, got %d", len(messages))
	}

	// Verify messages start with user message (compaction boundary)
	if len(messages) > 0 && messages[0].Role() != RoleUser {
		t.Errorf("After compaction, first message should be user, got %s", messages[0].Role())
	}

	// Turn 4: Verify conversation continues after compaction
	response, state, err := chat.ChatWithState(ctx, state,
		WithUserMessage("Question 4"),
	)
	if err != nil {
		t.Fatalf("Turn 4 failed after compaction: %v", err)
	}
	if response == "" {
		t.Error("Should receive response after compaction")
	}

	// State should still be compacted
	messages, _ = chat.decodeState(ctx, state)
	if len(messages) > 4 {
		t.Errorf("Expected <= 4 messages after turn 4, got %d", len(messages))
	}
}

// Test: Integration - Token-based compaction
func TestChat_CompactionIntegration_TokenLimit(t *testing.T) {
	promptTokens := 100
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			// Each turn adds more tokens
			promptTokens += 250
			return &ChatResponse{
				Message: &mockMessage{
					role:    RoleAssistant,
					content: "Response",
				},
				FinishReason: FinishReasonStop,
				Usage: &TokenUsage{
					PromptTokens:     promptTokens,
					CompletionTokens: 50,
					TotalTokens:      promptTokens + 50,
				},
			}, nil
		},
	}

	chat := &Chat{
		Backend: backend,
		Compactor: &TokenLimitCompactor{
			MaxTokens:    800, // Lower threshold
			TargetTokens: 400,
		},
	}

	ctx := context.Background()
	var state ConversationState

	// Run multiple turns - without compaction, message count would grow linearly
	for i := 1; i <= 6; i++ {
		var err error
		_, state, err = chat.ChatWithState(ctx, state,
			WithUserMessage(fmt.Sprintf("Question %d", i)),
		)
		if err != nil {
			t.Fatalf("Turn %d failed: %v", i, err)
		}
	}

	// After 6 turns, without compaction we'd have 12 messages
	// With token compaction triggering, we should have significantly fewer
	messages, _ := chat.decodeState(ctx, state)
	if len(messages) >= 10 {
		t.Errorf("Expected token compaction to limit messages, got %d (would be 12 without compaction)",
			len(messages))
	}

	// Verify state is valid and conversation continues
	response, _, err := chat.ChatWithState(ctx, state,
		WithUserMessage("Question 7"),
	)
	if err != nil {
		t.Fatalf("Turn 7 failed after token compaction: %v", err)
	}
	if response == "" {
		t.Error("Should receive response after token compaction")
	}
}

// Test: Integration - Compaction error handling
func TestChat_CompactionIntegration_ErrorHandling(t *testing.T) {
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			return &ChatResponse{
				Message: &mockMessage{
					role:    RoleAssistant,
					content: "Response",
				},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	// Create a compactor that always errors
	errorCompactor := &mockErrorCompactor{
		shouldError: true,
	}

	chat := &Chat{
		Backend:   backend,
		Compactor: errorCompactor,
	}

	ctx := context.Background()

	// Should propagate compaction error
	_, _, err := chat.ChatWithState(ctx, nil,
		WithUserMessage("Test"),
	)

	if err == nil {
		t.Error("Expected error from compaction to be propagated")
	}
	if !strings.Contains(err.Error(), "compaction failed") {
		t.Errorf("Expected 'compaction failed' in error, got: %v", err)
	}
}

// Test: Integration - State round-trip after compaction
func TestChat_CompactionIntegration_StateRoundTrip(t *testing.T) {
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			return &ChatResponse{
				Message: &mockMessage{
					role:    RoleAssistant,
					content: "Response",
				},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{
		Backend: backend,
		Compactor: &MessageLimitCompactor{
			MaxMessages: 2,
		},
	}

	ctx := context.Background()
	var state ConversationState

	// Build up conversation through compaction
	for i := 0; i < 5; i++ {
		var err error
		_, state, err = chat.ChatWithState(ctx, state,
			WithUserMessage("Question"),
		)
		if err != nil {
			t.Fatalf("Turn %d failed: %v", i+1, err)
		}
	}

	// Verify compacted state can be decoded
	messages, _ := chat.decodeState(ctx, state)
	if messages == nil {
		t.Fatal("Should be able to decode compacted state")
	}
	if len(messages) == 0 {
		t.Error("Compacted state should contain messages")
	}

	// Verify conversation continues with compacted state
	response, _, err := chat.ChatWithState(ctx, state,
		WithUserMessage("Final question"),
	)
	if err != nil {
		t.Fatalf("Failed to continue with compacted state: %v", err)
	}
	if response == "" {
		t.Error("Should receive response with compacted state")
	}
}

// Test: Integration - No compaction when compactor is nil
func TestChat_CompactionIntegration_NilCompactor(t *testing.T) {
	turnCount := 0
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			turnCount++
			return &ChatResponse{
				Message: &mockMessage{
					role:    RoleAssistant,
					content: "Response",
				},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{
		Backend:   backend,
		Compactor: nil, // No compaction
	}

	ctx := context.Background()
	var state ConversationState

	// Run multiple turns
	for i := 0; i < 5; i++ {
		var err error
		_, state, err = chat.ChatWithState(ctx, state,
			WithUserMessage("Question"),
		)
		if err != nil {
			t.Fatalf("Turn %d failed: %v", i+1, err)
		}
	}

	// Verify all messages are preserved (no compaction)
	messages, _ := chat.decodeState(ctx, state)
	expectedMessages := turnCount * 2 // user + assistant per turn
	if len(messages) != expectedMessages {
		t.Errorf("Expected %d messages without compaction, got %d",
			expectedMessages, len(messages))
	}
}

// mockErrorCompactor is a test compactor that can be configured to return errors
type mockErrorCompactor struct {
	shouldError bool
}

func (m *mockErrorCompactor) Compact(ctx context.Context, req *CompactionRequest) (*CompactionResponse, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock compaction error")
	}
	return NewNotCompactedMessagesResponse(req), nil
}
