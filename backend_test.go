package goaitools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/m0rjc/goaitools/aitooling"
)

// mockMessage implements Message interface for testing
type mockMessage struct {
	role       Role
	content    string
	toolCalls  []ToolCall
	toolCallID string
}

func (m *mockMessage) Role() Role              { return m.role }
func (m *mockMessage) Content() string         { return m.content }
func (m *mockMessage) ToolCalls() []ToolCall   { return m.toolCalls }
func (m *mockMessage) ToolCallID() string      { return m.toolCallID }
func (m *mockMessage) MarshalJSON() ([]byte, error) {
	// Simple JSON serialization for testing
	return json.Marshal(map[string]interface{}{
		"role":         m.role,
		"content":      m.content,
		"tool_calls":   m.toolCalls,
		"tool_call_id": m.toolCallID,
	})
}

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
		Message: &mockMessage{
			role:    RoleAssistant,
			content: "mock response",
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

func (m *mockBackend) NewSystemMessage(content string) Message {
	return &mockMessage{role: RoleSystem, content: content}
}

func (m *mockBackend) NewUserMessage(content string) Message {
	return &mockMessage{role: RoleUser, content: content}
}

func (m *mockBackend) NewToolMessage(toolCallID, content string) Message {
	return &mockMessage{role: RoleTool, content: content, toolCallID: toolCallID}
}

func (m *mockBackend) UnmarshalMessage(data []byte) (Message, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	msg := &mockMessage{
		role:    Role(raw["role"].(string)),
		content: raw["content"].(string),
	}
	if tcID, ok := raw["tool_call_id"].(string); ok {
		msg.toolCallID = tcID
	}
	return msg, nil
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

// Test: Message factory methods create messages with correct properties
func TestBackend_FactoryMethods(t *testing.T) {
	backend := &mockBackend{}

	// Test system message
	sysMsg := backend.NewSystemMessage("System instructions")
	if sysMsg.Role() != RoleSystem {
		t.Errorf("Expected role=%s, got %s", RoleSystem, sysMsg.Role())
	}
	if sysMsg.Content() != "System instructions" {
		t.Error("System message content not preserved")
	}

	// Test user message
	userMsg := backend.NewUserMessage("User question")
	if userMsg.Role() != RoleUser {
		t.Errorf("Expected role=%s, got %s", RoleUser, userMsg.Role())
	}
	if userMsg.Content() != "User question" {
		t.Error("User message content not preserved")
	}

	// Test tool message
	toolMsg := backend.NewToolMessage("call_123", "Tool result")
	if toolMsg.Role() != RoleTool {
		t.Errorf("Expected role=%s, got %s", RoleTool, toolMsg.Role())
	}
	if toolMsg.Content() != "Tool result" {
		t.Error("Tool message content not preserved")
	}
	if toolMsg.ToolCallID() != "call_123" {
		t.Error("Tool message call ID not preserved")
	}
}

// Test: ChatResponse with stop reason
func TestChatResponse_StopReason(t *testing.T) {
	resp := ChatResponse{
		Message: &mockMessage{
			role:    RoleAssistant,
			content: "Here is the answer",
		},
		FinishReason: FinishReasonStop,
	}

	if resp.FinishReason != FinishReasonStop {
		t.Errorf("Expected FinishReasonStop, got %s", resp.FinishReason)
	}

	if resp.Message.Content() != "Here is the answer" {
		t.Error("Response content should be preserved")
	}
}

// Test: ChatResponse with tool_calls reason
func TestChatResponse_ToolCallsReason(t *testing.T) {
	resp := ChatResponse{
		Message: &mockMessage{
			role: RoleAssistant,
			toolCalls: []ToolCall{
				{ID: "call_1", Name: "tool_a", Arguments: `{}`},
			},
		},
		FinishReason: FinishReasonToolCalls,
	}

	if resp.FinishReason != FinishReasonToolCalls {
		t.Errorf("Expected FinishReasonToolCalls, got %s", resp.FinishReason)
	}

	if len(resp.Message.ToolCalls()) != 1 {
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
		backend.NewUserMessage("test"),
	}

	resp, err := backend.ChatCompletion(ctx, messages, aitooling.ToolSet{})

	if err != nil {
		t.Fatalf("Backend implementation should not error: %v", err)
	}

	if resp == nil {
		t.Fatal("Backend should return non-nil response")
	}

	if resp.Message.Content() != "mock response" {
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

// Test: Messages can be JSON serialized and round-tripped (for state management)
func TestMessage_JSONRoundTrip(t *testing.T) {
	backend := &mockBackend{}

	// Create a message with tool call
	original := &mockMessage{
		role:    RoleAssistant,
		content: "test content",
		toolCalls: []ToolCall{
			{
				ID:        "call_123",
				Name:      "test_tool",
				Arguments: `{"param":"value"}`,
			},
		},
	}

	// Serialize
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Deserialize
	restored, err := backend.UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	// Verify
	if restored.Role() != original.Role() {
		t.Error("Role not preserved after JSON round-trip")
	}
	if restored.Content() != original.Content() {
		t.Error("Content not preserved after JSON round-trip")
	}
	// Note: tool calls preservation depends on implementation
}
