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
				Message:      &mockMessage{role: RoleAssistant, content: "Hello!"},
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
				Message:      &mockMessage{role: RoleAssistant, content: "ok"},
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

	if receivedMessages[0].Role() != RoleSystem {
		t.Errorf("First message should be system, got %s", receivedMessages[0].Role())
	}

	if receivedMessages[0].Content() != "You are a helpful assistant" {
		t.Error("System message content not preserved")
	}

	if receivedMessages[1].Role() != RoleUser {
		t.Errorf("Second message should be user, got %s", receivedMessages[1].Role())
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
					Message: &mockMessage{
						role: RoleAssistant,
						toolCalls: []ToolCall{
							{
								ID:        "call_123",
								Name:      "test_tool",
								Arguments: `{}`,
							},
						},
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			}

			// Second call: return final response
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "Done!"},
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
				Message: &mockMessage{
					role: RoleAssistant,
					toolCalls: []ToolCall{
						{ID: "call_1", Name: "test_tool", Arguments: `{}`},
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
				Message: &mockMessage{
					role: RoleAssistant,
					toolCalls: []ToolCall{
						{ID: "call_1", Name: "test_tool", Arguments: `{}`},
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
				Message:      &mockMessage{role: RoleAssistant, content: "Partial..."},
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
					Message: &mockMessage{
						role: RoleAssistant,
						toolCalls: []ToolCall{
							{ID: "call_1", Name: "logging_tool", Arguments: `{}`},
						},
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			}

			// Final response
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "Done"},
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
				Message: &mockMessage{
					role: RoleAssistant,
					toolCalls: []ToolCall{
						{ID: "call_1", Name: "test_tool", Arguments: `{}`},
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

// Test: LogToolArguments flag enables argument logging
func TestChat_LogToolArguments_EnablesArgumentLogging(t *testing.T) {
	logCalls := []map[string]interface{}{}

	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			// Return tool call on first iteration
			if len(messages) == 1 {
				return &ChatResponse{
					Message: &mockMessage{
						role: RoleAssistant,
						toolCalls: []ToolCall{
							{ID: "call_1", Name: "test_tool", Arguments: `{"arg":"value"}`},
						},
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			}

			// Final response
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "Done"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	tools := aitooling.ToolSet{
		&mockTool{name: "test_tool"},
	}

	// Mock system logger to capture log calls
	systemLogger := &mockSystemLogger{
		debugFunc: func(ctx context.Context, msg string, keysAndValues ...interface{}) {
			logEntry := map[string]interface{}{"msg": msg}
			for i := 0; i < len(keysAndValues); i += 2 {
				if i+1 < len(keysAndValues) {
					logEntry[keysAndValues[i].(string)] = keysAndValues[i+1]
				}
			}
			logCalls = append(logCalls, logEntry)
		},
	}

	chat := &Chat{
		Backend:          backend,
		SystemLogger:     systemLogger,
		LogToolArguments: true, // Enable argument logging
	}

	_, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
		WithTools(tools),
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Find the executing_tool_call log
	var toolCallLog map[string]interface{}
	for _, log := range logCalls {
		if log["msg"] == "executing_tool_call" {
			toolCallLog = log
			break
		}
	}

	if toolCallLog == nil {
		t.Fatal("Expected executing_tool_call log not found")
	}

	// Verify tool_args is present
	if toolCallLog["tool_args"] == nil {
		t.Error("Expected tool_args in log when LogToolArguments=true")
	}

	if toolCallLog["tool_args"] != `{"arg":"value"}` {
		t.Errorf("Expected tool_args to be '{\"arg\":\"value\"}', got %v", toolCallLog["tool_args"])
	}
}

// Test: LogToolArguments=false does not log arguments
func TestChat_LogToolArguments_Disabled_DoesNotLogArguments(t *testing.T) {
	logCalls := []map[string]interface{}{}

	backend := &mockBackend{
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			// Return tool call on first iteration
			if len(messages) == 1 {
				return &ChatResponse{
					Message: &mockMessage{
						role: RoleAssistant,
						toolCalls: []ToolCall{
							{ID: "call_1", Name: "test_tool", Arguments: `{"arg":"value"}`},
						},
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			}

			// Final response
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "Done"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	tools := aitooling.ToolSet{
		&mockTool{name: "test_tool"},
	}

	// Mock system logger to capture log calls
	systemLogger := &mockSystemLogger{
		debugFunc: func(ctx context.Context, msg string, keysAndValues ...interface{}) {
			logEntry := map[string]interface{}{"msg": msg}
			for i := 0; i < len(keysAndValues); i += 2 {
				if i+1 < len(keysAndValues) {
					logEntry[keysAndValues[i].(string)] = keysAndValues[i+1]
				}
			}
			logCalls = append(logCalls, logEntry)
		},
	}

	chat := &Chat{
		Backend:          backend,
		SystemLogger:     systemLogger,
		LogToolArguments: false, // Disable argument logging
	}

	_, err := chat.Chat(
		context.Background(),
		WithUserMessage("Test"),
		WithTools(tools),
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Find the executing_tool_call log
	var toolCallLog map[string]interface{}
	for _, log := range logCalls {
		if log["msg"] == "executing_tool_call" {
			toolCallLog = log
			break
		}
	}

	if toolCallLog == nil {
		t.Fatal("Expected executing_tool_call log not found")
	}

	// Verify tool_args is NOT present
	if toolCallLog["tool_args"] != nil {
		t.Error("Expected tool_args NOT in log when LogToolArguments=false")
	}
}

// mockSystemLogger for testing
type mockSystemLogger struct {
	debugFunc func(ctx context.Context, msg string, keysAndValues ...interface{})
	infoFunc  func(ctx context.Context, msg string, keysAndValues ...interface{})
	errorFunc func(ctx context.Context, msg string, err error, keysAndValues ...interface{})
}

func (m *mockSystemLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if m.debugFunc != nil {
		m.debugFunc(ctx, msg, keysAndValues...)
	}
}

func (m *mockSystemLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if m.infoFunc != nil {
		m.infoFunc(ctx, msg, keysAndValues...)
	}
}

func (m *mockSystemLogger) Error(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	if m.errorFunc != nil {
		m.errorFunc(ctx, msg, err, keysAndValues...)
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

// Test: State encoding/decoding round-trip
func TestChat_StateEncodingDecoding_RoundTrip(t *testing.T) {
	backend := &mockBackend{
		providerName: "test-provider",
	}

	chat := &Chat{Backend: backend}

	originalMessages := []Message{
		backend.NewUserMessage("Hello"),
		&mockMessage{role: RoleAssistant, content: "Hi there!"},
		backend.NewUserMessage("How are you?"),
	}

	// Encode
	state, err := chat.encodeState(originalMessages, len(originalMessages))
	if err != nil {
		t.Fatalf("Failed to encode state: %v", err)
	}

	if state == nil || len(state) == 0 {
		t.Fatal("Encoded state should not be empty")
	}

	// Decode
	decodedMessages, _ := chat.decodeState(context.Background(), state)

	if len(decodedMessages) != len(originalMessages) {
		t.Fatalf("Expected %d messages, got %d", len(originalMessages), len(decodedMessages))
	}

	for i, msg := range decodedMessages {
		if msg.Role() != originalMessages[i].Role() {
			t.Errorf("Message %d: expected role %s, got %s", i, originalMessages[i].Role(), msg.Role())
		}
		if msg.Content() != originalMessages[i].Content() {
			t.Errorf("Message %d: expected content %s, got %s", i, originalMessages[i].Content(), msg.Content())
		}
	}
}

// Test: RoleOther messages are correctly round-tripped through state
func TestChat_StateEncodingDecoding_RoleOtherRoundTrip(t *testing.T) {
	backend := &mockBackend{
		providerName: "test-provider",
	}

	chat := &Chat{Backend: backend}

	// Create conversation with various message types including RoleOther
	originalMessages := []Message{
		backend.NewUserMessage("Hello"),
		&mockMessage{role: RoleAssistant, content: "Hi there!"},
		&mockMessage{role: RoleOther, content: "Thinking: I should respond politely"},
		backend.NewUserMessage("How are you?"),
		&mockMessage{role: RoleAssistant, content: "I'm doing well"},
		&mockMessage{role: RoleOther, content: "Internal reasoning about the conversation"},
	}

	// Encode
	state, err := chat.encodeState(originalMessages, len(originalMessages))
	if err != nil {
		t.Fatalf("Failed to encode state with RoleOther messages: %v", err)
	}

	if state == nil || len(state) == 0 {
		t.Fatal("Encoded state should not be empty")
	}

	// Decode
	decodedMessages, _ := chat.decodeState(context.Background(), state)

	if len(decodedMessages) != len(originalMessages) {
		t.Fatalf("Expected %d messages, got %d", len(originalMessages), len(decodedMessages))
	}

	// Verify all messages including RoleOther are preserved
	for i, msg := range decodedMessages {
		if msg.Role() != originalMessages[i].Role() {
			t.Errorf("Message %d: expected role %s, got %s", i, originalMessages[i].Role(), msg.Role())
		}
		if msg.Content() != originalMessages[i].Content() {
			t.Errorf("Message %d: expected content %s, got %s", i, originalMessages[i].Content(), msg.Content())
		}
	}

	// Specifically verify RoleOther messages
	if decodedMessages[2].Role() != RoleOther {
		t.Errorf("Message 2 should be RoleOther, got %s", decodedMessages[2].Role())
	}
	if decodedMessages[5].Role() != RoleOther {
		t.Errorf("Message 5 should be RoleOther, got %s", decodedMessages[5].Role())
	}
}

// Test: ChatWithState with nil state starts new conversation
func TestChat_ChatWithState_NilStateStartsNewConversation(t *testing.T) {
	var receivedMessages []Message

	backend := &mockBackend{
		providerName: "test-provider",
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			receivedMessages = messages
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "Hello!"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	response, state, err := chat.ChatWithState(
		context.Background(),
		nil, // nil state = new conversation
		WithUserMessage("Hi"),
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response != "Hello!" {
		t.Errorf("Expected 'Hello!', got '%s'", response)
	}

	if state == nil {
		t.Fatal("Expected non-nil state")
	}

	// Should have received only the user message
	if len(receivedMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(receivedMessages))
	}
}

// Test: ChatWithState continues conversation from existing state
func TestChat_ChatWithState_ContinuesFromExistingState(t *testing.T) {
	var receivedMessages []Message
	callCount := 0

	backend := &mockBackend{
		providerName: "test-provider",
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			callCount++
			receivedMessages = messages
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "Response " + string(rune('0'+callCount))},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	// First turn
	response1, state1, err := chat.ChatWithState(
		context.Background(),
		nil,
		WithUserMessage("First message"),
	)

	if err != nil {
		t.Fatalf("First turn failed: %v", err)
	}

	// Second turn with state from first turn
	response2, state2, err := chat.ChatWithState(
		context.Background(),
		state1,
		WithUserMessage("Second message"),
	)

	if err != nil {
		t.Fatalf("Second turn failed: %v", err)
	}

	// Second turn should see history from first turn + new message
	// (not yet including the second assistant response, which happens after this call)
	if len(receivedMessages) != 3 {
		t.Fatalf("Expected 3 messages (user 1, assistant 1, user 2), got %d", len(receivedMessages))
	}

	// Verify message order
	expectedContent := []string{"First message", "Response 1", "Second message"}
	for i := 0; i < 3; i++ {
		if receivedMessages[i].Content() != expectedContent[i] {
			t.Errorf("Message %d: expected '%s', got '%s'", i, expectedContent[i], receivedMessages[i].Content())
		}
	}

	if state2 == nil {
		t.Fatal("Expected non-nil state after second turn")
	}

	_ = response1
	_ = response2
}

// Test: System messages are not persisted in state
func TestChat_ChatWithState_SystemMessagesNotPersisted(t *testing.T) {
	backend := &mockBackend{
		providerName: "test-provider",
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "ok"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	// First turn with system message
	_, state, err := chat.ChatWithState(
		context.Background(),
		nil,
		WithSystemMessage("You are a helpful assistant"),
		WithUserMessage("Hello"),
	)

	if err != nil {
		t.Fatalf("First turn failed: %v", err)
	}

	// Decode state and verify system message is NOT present
	messages, _ := chat.decodeState(context.Background(), state)

	for _, msg := range messages {
		if msg.Role() == RoleSystem {
			t.Error("System message should not be persisted in state")
		}
	}

	// Should only have user message and assistant response
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages in state (user + assistant), got %d", len(messages))
	}
}

// Test: System messages are prepended on each turn
func TestChat_ChatWithState_SystemMessagesPrependedEachTurn(t *testing.T) {
	var receivedMessages []Message

	backend := &mockBackend{
		providerName: "test-provider",
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			receivedMessages = messages
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "ok"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	// First turn
	_, state, _ := chat.ChatWithState(
		context.Background(),
		nil,
		WithSystemMessage("System prompt 1"),
		WithUserMessage("Hello"),
	)

	// Second turn with different system message
	_, _, _ = chat.ChatWithState(
		context.Background(),
		state,
		WithSystemMessage("System prompt 2"), // Different system message
		WithUserMessage("Hi again"),
	)

	// Backend should receive system message at the start
	if receivedMessages[0].Role() != RoleSystem {
		t.Error("First message should be system message")
	}

	if receivedMessages[0].Content() != "System prompt 2" {
		t.Errorf("Expected 'System prompt 2', got '%s'", receivedMessages[0].Content())
	}
}

// Test: Provider mismatch discards state
func TestChat_DecodeState_ProviderMismatch_DiscardsState(t *testing.T) {
	// Create state with one provider
	backend1 := &mockBackend{providerName: "provider-a"}
	chat1 := &Chat{Backend: backend1}
	state, _ := chat1.encodeState([]Message{
		backend1.NewUserMessage("test"),
	}, 1)

	// Try to decode with different provider
	backend2 := &mockBackend{providerName: "provider-b"}
	chat2 := &Chat{Backend: backend2}
	messages, _ := chat2.decodeState(context.Background(), state)

	// Should return nil (graceful degradation)
	if messages != nil {
		t.Error("Expected nil messages when provider mismatches")
	}
}

// Test: Invalid state is gracefully ignored
func TestChat_DecodeState_InvalidState_ReturnsNil(t *testing.T) {
	backend := &mockBackend{providerName: "test"}
	chat := &Chat{Backend: backend}

	// Corrupt state
	invalidState := ConversationState([]byte("not valid json"))

	messages, _ := chat.decodeState(context.Background(), invalidState)

	// Should return nil (graceful degradation)
	if messages != nil {
		t.Error("Expected nil messages for invalid state")
	}
}

// Test: AppendToState adds event to state
func TestChat_AppendToState_AddsEventToState(t *testing.T) {
	backend := &mockBackend{providerName: "test"}
	chat := &Chat{Backend: backend}

	// Create initial state
	initialState, _ := chat.encodeState([]Message{
		backend.NewUserMessage("Hello"),
		&mockMessage{role: RoleAssistant, content: "Hi!"},
	}, 2)

	// Add event
	newState := chat.AppendToState(
		context.Background(),
		initialState,
		WithUserMessage("User visited location X"),
	)

	if newState == nil {
		t.Fatal("Expected non-nil state after event")
	}

	// Decode and verify event was added
	messages, _ := chat.decodeState(context.Background(), newState)

	if len(messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(messages))
	}

	lastMessage := messages[len(messages)-1]
	if lastMessage.Role() != RoleUser {
		t.Errorf("Event should be added as user message, got %s", lastMessage.Role())
	}

	if lastMessage.Content() != "User visited location X" {
		t.Errorf("Event content not preserved, got '%s'", lastMessage.Content())
	}
}

// Test: AppendToState works with nil state
func TestChat_AppendToState_NilState_CreatesNewState(t *testing.T) {
	backend := &mockBackend{providerName: "test"}
	chat := &Chat{Backend: backend}

	newState := chat.AppendToState(
		context.Background(),
		nil, // nil state
		WithUserMessage("Initial event"),
	)

	if newState == nil {
		t.Fatal("Expected non-nil state")
	}

	messages, _ := chat.decodeState(context.Background(), newState)

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Content() != "Initial event" {
		t.Error("Event content not preserved")
	}
}

// Test: Chat() delegates to ChatWithState
func TestChat_DelegatesToChatWithState(t *testing.T) {
	backend := &mockBackend{
		providerName: "test",
		chatFunc: func(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error) {
			return &ChatResponse{
				Message:      &mockMessage{role: RoleAssistant, content: "response"},
				FinishReason: FinishReasonStop,
			}, nil
		},
	}

	chat := &Chat{Backend: backend}

	// Call Chat() (stateless)
	response, err := chat.Chat(
		context.Background(),
		WithUserMessage("test"),
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response != "response" {
		t.Errorf("Expected 'response', got '%s'", response)
	}
}

// Test: ProcessedLength is correctly serialized in state
func TestChat_StateEncodingDecoding_ProcessedLength(t *testing.T) {
	backend := &mockBackend{
		providerName: "test-provider",
	}

	chat := &Chat{Backend: backend}

	// Create messages and encode with a specific ProcessedLength
	messages := []Message{
		backend.NewUserMessage("Message 1"),
		&mockMessage{role: RoleAssistant, content: "Response 1"},
		backend.NewUserMessage("Message 2"),
		&mockMessage{role: RoleAssistant, content: "Response 2"},
		backend.NewUserMessage("Message 3 - appended via AppendToState"),
	}

	// Encode with ProcessedLength = 4 (first 4 messages were seen by LLM, last message was appended)
	expectedProcessedLength := 4
	state, err := chat.encodeState(messages, expectedProcessedLength)
	if err != nil {
		t.Fatalf("Failed to encode state: %v", err)
	}

	// Decode the state as JSON to verify ProcessedLength was serialized
	var internal conversationStateInternal
	err = json.Unmarshal(state, &internal)
	if err != nil {
		t.Fatalf("Failed to unmarshal state: %v", err)
	}

	// Verify ProcessedLength was correctly serialized
	if internal.ProcessedLength != expectedProcessedLength {
		t.Errorf("ProcessedLength in serialized state: expected %d, got %d", expectedProcessedLength, internal.ProcessedLength)
	}

	// Also verify via decodeState
	_, decodedProcessedLength := chat.decodeState(context.Background(), state)
	if decodedProcessedLength != expectedProcessedLength {
		t.Errorf("ProcessedLength from decodeState: expected %d, got %d", expectedProcessedLength, decodedProcessedLength)
	}
}

// Test: AppendToState preserves ProcessedLength
func TestChat_AppendToState_PreservesProcessedLength(t *testing.T) {
	backend := &mockBackend{providerName: "test"}
	chat := &Chat{Backend: backend}

	// Create initial state with 2 messages, both processed
	initialMessages := []Message{
		backend.NewUserMessage("Hello"),
		&mockMessage{role: RoleAssistant, content: "Hi!"},
	}
	initialProcessedLength := 2
	initialState, err := chat.encodeState(initialMessages, initialProcessedLength)
	if err != nil {
		t.Fatalf("Failed to encode initial state: %v", err)
	}

	// Append a new message (this should not increase ProcessedLength)
	newState := chat.AppendToState(
		context.Background(),
		initialState,
		WithUserMessage("User visited location X"),
	)

	if newState == nil {
		t.Fatal("Expected non-nil state after event")
	}

	// Decode and verify ProcessedLength was preserved
	messages, processedLength := chat.decodeState(context.Background(), newState)

	if len(messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(messages))
	}

	// ProcessedLength should still be 2 (the appended message is not processed yet)
	if processedLength != initialProcessedLength {
		t.Errorf("ProcessedLength should be preserved after AppendToState: expected %d, got %d", initialProcessedLength, processedLength)
	}
}
