package goaitools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/m0rjc/goaitools/aitooling"
)

// mockTool for testing tool execution
type mockTool struct {
	name        string
	description string
	executeFunc func(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error)
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string         { return m.description }
func (m *mockTool) Parameters() json.RawMessage { return aitooling.EmptyJsonSchema() }
func (m *mockTool) Execute(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return req.NewResult("success"), nil
}

// Test: Chat with simple stop response
func TestChat_SimpleStopResponse(t *testing.T) {
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			return &ChatResponse{
				Message:      Message{Role: RoleAssistant, Content: "Hello!"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	response, err := chat.Chat(
		context.Background(),
		WithUserMessage("Hi"),
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response != "Hello!" {
		t.Errorf("Expected 'Hello!', got '%s'", response)
	}
}

// Test: WithSystemMessage option
func TestChat_WithSystemMessage(t *testing.T) {
	var receivedMessages []Message

	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			receivedMessages = messages
			return &ChatResponse{
				Message:      Message{Role: RoleAssistant, Content: "ok"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	chat.Chat(
		context.Background(),
		WithSystemMessage("You are a helpful assistant"),
		WithUserMessage("Hello"),
	)

	if len(receivedMessages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(receivedMessages))
	}

	if receivedMessages[0].Role != RoleSystem {
		t.Errorf("First message should be system, got %s", receivedMessages[0].Role)
	}

	if receivedMessages[0].Content != "You are a helpful assistant" {
		t.Error("System message content not preserved")
	}

	if receivedMessages[1].Role != RoleUser {
		t.Errorf("Second message should be user, got %s", receivedMessages[1].Role)
	}
}

// Test: Tool-calling loop executes tools and continues
func TestChat_ToolCallingLoop(t *testing.T) {
	callCount := 0

	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			callCount++

			// First call: return tool_calls
			if callCount == 1 {
				return &ChatResponse{
					Message: Message{
						Role: RoleAssistant,
						ToolCalls: []ToolCall{
							{
								ID:        "call_123",
								Name:      "test_tool",
								Arguments: json.RawMessage(`{}`),
							},
						},
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			}

			// Second call: return final response
			return &ChatResponse{
				Message:      Message{Role: RoleAssistant, Content: "Done!"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	toolExecuted := false
	tools := aitooling.ToolSet{
		&mockTool{
			name: "test_tool",
			executeFunc: func(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error) {
				toolExecuted = true
				return req.NewResult("tool executed"), nil
			},
		},
	}

	chat := &Chat{Backend: backend}

	response, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
		WithTools(tools),
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !toolExecuted {
		t.Error("Tool should have been executed")
	}

	if callCount != 2 {
		t.Errorf("Expected 2 backend calls, got %d", callCount)
	}

	if response != "Done!" {
		t.Errorf("Expected final response 'Done!', got '%s'", response)
	}
}

// Test: Max iterations prevents infinite loops
func TestChat_MaxIterationsPreventsInfiniteLoop(t *testing.T) {
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			// Always request tool calls (infinite loop scenario)
			return &ChatResponse{
				Message: Message{
					Role: RoleAssistant,
					ToolCalls: []ToolCall{
						{ID: "call_1", Name: "test_tool", Arguments: json.RawMessage(`{}`)},
					},
				},
				FinishReason: FinishReasonToolCalls,
			}, nil
		},
	}

	tools := aitooling.ToolSet{
		&mockTool{name: "test_tool"},
	}

	chat := &Chat{
		Backend:           backend,
		MaxToolIterations: 3, // Small limit for testing
	}

	_, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
		WithTools(tools),
	)

	if err == nil {
		t.Fatal("Expected error for exceeding max iterations")
	}

	if err.Error() != "exceeded max tool iterations (3)" {
		t.Errorf("Unexpected error: %v", err)
	}
}

// Test: WithMaxToolIterations option overrides Chat setting
func TestChat_WithMaxToolIterations_OverridesDefault(t *testing.T) {
	callCount := 0

	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			callCount++
			// Always request tool calls
			return &ChatResponse{
				Message: Message{
					Role: RoleAssistant,
					ToolCalls: []ToolCall{
						{ID: "call_1", Name: "test_tool", Arguments: json.RawMessage(`{}`)},
					},
				},
				FinishReason: FinishReasonToolCalls,
			}, nil
		},
	}

	tools := aitooling.ToolSet{
		&mockTool{name: "test_tool"},
	}

	chat := &Chat{
		Backend:           backend,
		MaxToolIterations: 100, // High default
	}

	_, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
		WithTools(tools),
		WithMaxToolIterations(2), // Override with low value
	)

	if err == nil {
		t.Fatal("Expected error for exceeding max iterations")
	}

	// Should have stopped at 2 iterations (not 100)
	if callCount > 2 {
		t.Errorf("Expected max 2 calls due to override, got %d", callCount)
	}
}

// Test: Backend error is propagated
func TestChat_BackendError_IsPropagated(t *testing.T) {
	backendErr := errors.New("API connection failed")

	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			return nil, backendErr
		},
	}

	chat := &Chat{Backend: backend}

	_, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
	)

	if err != backendErr {
		t.Errorf("Expected backend error to be propagated, got %v", err)
	}
}

// Test: FinishReasonLength returns error
func TestChat_FinishReasonLength_ReturnsError(t *testing.T) {
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			return &ChatResponse{
				Message:      Message{Role: RoleAssistant, Content: "Partial..."},
				FinishReason: FinishReasonLength,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	_, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
	)

	if err == nil {
		t.Fatal("Expected error for length finish reason")
	}

	if err.Error() != "conversation exceeded max tokens" {
		t.Errorf("Unexpected error: %v", err)
	}
}

// mockAction for logging tests
type mockAction struct{ desc string }

func (m mockAction) Description() string { return m.desc }

// Test: ToolActionLogger receives tool actions
func TestChat_ToolActionLogger_ReceivesActions(t *testing.T) {
	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			// Return tool call on first iteration
			if len(messages) == 1 {
				return &ChatResponse{
					Message: Message{
						Role: RoleAssistant,
						ToolCalls: []ToolCall{
							{ID: "call_1", Name: "logging_tool", Arguments: json.RawMessage(`{}`)},
						},
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			}

			// Final response
			return &ChatResponse{
				Message:      Message{Role: RoleAssistant, Content: "Done"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	loggedActions := []aitooling.ToolAction{}

	tools := aitooling.ToolSet{
		&mockTool{
			name: "logging_tool",
			executeFunc: func(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error) {
				// Tool logs an action
				ctx.Logger.Log(mockAction{desc: "tool executed"})
				return req.NewResult("ok"), nil
			},
		},
	}

	logger := &mockToolLogger{
		logFunc: func(action aitooling.ToolAction) {
			loggedActions = append(loggedActions, action)
		},
	}

	chat := &Chat{Backend: backend}

	chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
		WithTools(tools),
		WithToolActionLogger(logger),
	)

	if len(loggedActions) != 1 {
		t.Errorf("Expected 1 logged action, got %d", len(loggedActions))
	}

	if loggedActions[0].Description() != "tool executed" {
		t.Error("Logged action not preserved")
	}
}

// Test: Default max iterations is 10
func TestChat_DefaultMaxIterations(t *testing.T) {
	callCount := 0

	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			callCount++
			// Always request tool calls
			return &ChatResponse{
				Message: Message{
					Role: RoleAssistant,
					ToolCalls: []ToolCall{
						{ID: "call_1", Name: "test_tool", Arguments: json.RawMessage(`{}`)},
					},
				},
				FinishReason: FinishReasonToolCalls,
			}, nil
		},
	}

	tools := aitooling.ToolSet{
		&mockTool{name: "test_tool"},
	}

	chat := &Chat{Backend: backend} // No MaxToolIterations set

	_, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
		WithTools(tools),
	)

	if err == nil {
		t.Fatal("Expected error for exceeding max iterations")
	}

	// Should have stopped at default 10 iterations
	if callCount != 10 {
		t.Errorf("Expected 10 calls (default limit), got %d", callCount)
	}
}

// mockToolLogger for testing
type mockToolLogger struct {
	logFunc func(action aitooling.ToolAction)
}

func (m *mockToolLogger) Log(action aitooling.ToolAction) {
	if m.logFunc != nil {
		m.logFunc(action)
	}
}

func (m *mockToolLogger) LogAll(actions []aitooling.ToolAction) {
	if m.logFunc != nil {
		for _, action := range actions {
			m.logFunc(action)
		}
	}
}
