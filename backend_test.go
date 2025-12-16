package goaitools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/m0rjc/goaitools/aitooling"
)

// mockBackend implements Backend interface for testing
type mockBackend struct {
	chatFunc     func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error)
	providerName string
}

func (m *mockBackend) ChatCompletion(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, messages, tools)
	}
	return &ChatResponse{
		Message: Message{
			Role:    RoleAssistant,
			Content: "mock response",
		},
		FinishReason: FinishReasonStop,
	}, nil
}

func (m *mockBackend) ProviderName() string {
	if m.providerName != "" {
		return m.providerName
	}
	return "mock-provider"
}

// Test: Role constants are type-safe strings
func TestRole_Constants(t *testing.T) {
	// Verify constants exist and have correct values
	if RoleSystem != "system" {
		t.Errorf("RoleSystem should be 'system', got '%s'", RoleSystem)
	}
	if RoleUser != "user" {
		t.Errorf("RoleUser should be 'user', got '%s'", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant should be 'assistant', got '%s'", RoleAssistant)
	}
	if RoleTool != "tool" {
		t.Errorf("RoleTool should be 'tool', got '%s'", RoleTool)
	}
}

// Test: FinishReason constants are type-safe strings
func TestFinishReason_Constants(t *testing.T) {
	if FinishReasonStop != "stop" {
		t.Errorf("FinishReasonStop should be 'stop', got '%s'", FinishReasonStop)
	}
	if FinishReasonToolCalls != "tool_calls" {
		t.Errorf("FinishReasonToolCalls should be 'tool_calls', got '%s'", FinishReasonToolCalls)
	}
	if FinishReasonLength != "length" {
		t.Errorf("FinishReasonLength should be 'length', got '%s'", FinishReasonLength)
	}
}

// Test: Message with user role
func TestMessage_UserRole(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "What is the weather?",
	}

	if msg.Role != RoleUser {
		t.Errorf("Expected role=%s, got %s", RoleUser, msg.Role)
	}

	if msg.Content != "What is the weather?" {
		t.Errorf("Expected content to be preserved")
	}

	// ToolCalls should be empty for user message
	if len(msg.ToolCalls) != 0 {
		t.Error("User message should not have tool calls")
	}
}

// Test: Message with tool calls
func TestMessage_WithToolCalls(t *testing.T) {
	msg := Message{
		Role: RoleAssistant,
		ToolCalls: []ToolCall{
			{
				ID:        "call_abc123",
				Name:      "get_weather",
				Arguments: json.RawMessage(`{"location":"London"}`),
			},
		},
	}

	if len(msg.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(msg.ToolCalls))
	}

	call := msg.ToolCalls[0]
	if call.Name != "get_weather" {
		t.Errorf("Expected tool name='get_weather', got '%s'", call.Name)
	}

	if call.ID != "call_abc123" {
		t.Errorf("Expected call ID='call_abc123', got '%s'", call.ID)
	}

	// Verify arguments are valid JSON
	var args map[string]string
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		t.Errorf("Arguments should be valid JSON: %v", err)
	}

	if args["location"] != "London" {
		t.Errorf("Expected location='London', got '%s'", args["location"])
	}
}

// Test: Tool result message
func TestMessage_ToolResult(t *testing.T) {
	msg := Message{
		Role:       RoleTool,
		Content:    "The weather in London is sunny",
		ToolCallID: "call_abc123",
	}

	if msg.Role != RoleTool {
		t.Errorf("Expected role=%s, got %s", RoleTool, msg.Role)
	}

	if msg.ToolCallID != "call_abc123" {
		t.Errorf("Tool result should reference the call ID")
	}
}

// Test: ChatResponse with stop reason
func TestChatResponse_StopReason(t *testing.T) {
	resp := ChatResponse{
		Message: Message{
			Role:    RoleAssistant,
			Content: "Here is the answer",
		},
		FinishReason: FinishReasonStop,
	}

	if resp.FinishReason != FinishReasonStop {
		t.Errorf("Expected FinishReasonStop, got %s", resp.FinishReason)
	}

	if resp.Message.Content != "Here is the answer" {
		t.Error("Response content should be preserved")
	}
}

// Test: ChatResponse with tool_calls reason
func TestChatResponse_ToolCallsReason(t *testing.T) {
	resp := ChatResponse{
		Message: Message{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{
				{ID: "call_1", Name: "tool_a", Arguments: json.RawMessage(`{}`)},
			},
		},
		FinishReason: FinishReasonToolCalls,
	}

	if resp.FinishReason != FinishReasonToolCalls {
		t.Errorf("Expected FinishReasonToolCalls, got %s", resp.FinishReason)
	}

	if len(resp.Message.ToolCalls) != 1 {
		t.Error("Should have tool calls when finish reason is tool_calls")
	}
}

// Test: Backend interface contract - can be implemented by any provider
func TestBackend_InterfaceContract(t *testing.T) {
	// Verify mockBackend implements Backend
	var _ Backend = &mockBackend{}

	backend := &mockBackend{}
	ctx := context.Background()

	messages := []Message{
		{Role: RoleUser, Content: "test"},
	}

	resp, err := backend.ChatCompletion(ctx, messages, aitooling.ToolSet{})

	if err != nil {
		t.Fatalf("Backend implementation should not error: %v", err)
	}

	if resp == nil {
		t.Fatal("Backend should return non-nil response")
	}

	if resp.Message.Content != "mock response" {
		t.Error("Backend should return configured response")
	}
}

// Test: SystemLogger interface contract
func TestSystemLogger_InterfaceContract(t *testing.T) {
	// Verify our public loggers implement SystemLogger
	var _ SystemLogger = SlogSystemLogger{}
	var _ SystemLogger = SilentLogger{}

	// Test SilentLogger doesn't panic
	silent := NewSilentLogger()
	ctx := context.Background()

	// Should not panic
	silent.Debug(ctx, "debug message", "key", "value")
	silent.Info(ctx, "info message", "key", "value")
	silent.Error(ctx, "error message", nil, "key", "value")
}

// Test: Messages can be JSON serialized (for state management in Story 2)
func TestMessage_JSONSerialization(t *testing.T) {
	original := Message{
		Role:    RoleAssistant,
		Content: "test content",
		ToolCalls: []ToolCall{
			{
				ID:        "call_123",
				Name:      "test_tool",
				Arguments: json.RawMessage(`{"param":"value"}`),
			},
		},
	}

	// Serialize
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Deserialize
	var restored Message
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	// Verify
	if restored.Role != original.Role {
		t.Error("Role not preserved after JSON round-trip")
	}
	if restored.Content != original.Content {
		t.Error("Content not preserved after JSON round-trip")
	}
	if len(restored.ToolCalls) != len(original.ToolCalls) {
		t.Error("ToolCalls not preserved after JSON round-trip")
	}
}
