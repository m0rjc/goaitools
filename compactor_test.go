package goaitools

import (
	"context"
	"testing"
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
