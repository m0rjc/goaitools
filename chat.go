package goaitools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/m0rjc/goaitools/aitooling"
)

// ConversationState is an opaque blob representing conversation history.
// Clients should treat this as a black box - store it, retrieve it, but don't inspect it.
type ConversationState []byte

type Chat struct {
	Backend           Backend
	MaxToolIterations int              // Default max iterations for tool-calling loop (0 = use default 10)
	SystemLogger      SystemLogger     // Optional logger for system/debug logging
	ToolActionLogger  aitooling.Logger // Optional default logger for tool actions
}

// conversationStateInternal is the internal representation of conversation state.
// This is not exposed to clients - they only see the opaque []byte.
type conversationStateInternal struct {
	Version  int       `json:"version"`  // State format version (current: 1)
	Provider string    `json:"provider"` // Backend provider name (e.g., "openai")
	Messages []Message `json:"messages"` // Conversation history
}

type chatRequest struct {
	messages          []Message
	tools             aitooling.ToolSet
	logCallback       aitooling.Logger
	maxToolIterations *int // Pointer to distinguish between "not set" and "set to 0"
}

// ChatOption is a function that configures a chatRequestConfig.
type ChatOption func(*chatRequest)

func WithToolActionLogger(callback aitooling.Logger) ChatOption {
	return func(cfg *chatRequest) {
		cfg.logCallback = callback
	}
}

func WithTools(tools aitooling.ToolSet) ChatOption {
	return func(cfg *chatRequest) {
		cfg.tools = tools
	}
}

func WithSystemMessage(text string) ChatOption {
	return func(cfg *chatRequest) {
		cfg.messages = append(cfg.messages, Message{
			Role:    RoleSystem,
			Content: text,
		})
	}
}

func WithUserMessage(text string) ChatOption {
	return func(cfg *chatRequest) {
		cfg.messages = append(cfg.messages, Message{
			Role:    RoleUser,
			Content: text,
		})
	}
}

// WithMaxToolIterations sets the maximum number of tool-calling iterations for this chat request.
// This overrides the Chat.MaxToolIterations setting for this specific request.
func WithMaxToolIterations(max int) ChatOption {
	return func(cfg *chatRequest) {
		cfg.maxToolIterations = &max
	}
}

// ChatWithState performs a chat with conversation history.
// Parameters:
//   - ctx: Standard Go context
//   - state: Opaque conversation state from previous turn (nil for new conversation)
//   - opts: Chat options (messages, tools, logger)
//
// Returns:
//   - response: AI's text response
//   - newState: Updated conversation state for next turn
//   - error: Any errors during execution
//
// System Message Handling:
// Following OpenAI's session memory pattern, system messages (via WithSystemMessage)
// are NOT stored in state. They should be passed on every call and will be prepended
// to the conversation. This allows system prompts to include dynamic content (timestamps,
// user context) that updates each turn.
func (c *Chat) ChatWithState(
	ctx context.Context,
	state ConversationState,
	opts ...ChatOption,
) (string, ConversationState, error) {
	// Build configuration from options
	request := chatRequest{
		messages:    []Message{},
		tools:       aitooling.ToolSet{},
		logCallback: nil,
	}
	for _, opt := range opts {
		opt(&request)
	}

	// Decode existing state (conversation history only, no system messages)
	stateMessages := c.decodeState(ctx, state)

	// Build messages: system message (if any) + state history + new user messages
	messages := c.buildMessages(request.messages, stateMessages)

	// Use Chat-level default logger if no per-request logger provided
	toolLogger := request.logCallback
	if toolLogger == nil {
		if c.ToolActionLogger != nil {
			toolLogger = c.ToolActionLogger
		} else {
			toolLogger = &dummyLogger{}
		}
	}

	// Determine max iterations: per-call option > Chat field > default (10)
	maxIter := c.resolveMaxIterations(request.maxToolIterations)

	// Tool-calling loop
	for iteration := 0; iteration < maxIter; iteration++ {
		c.logDebug(ctx, "starting_chat_iteration", "iteration", iteration)

		// Call backend for single turn
		response, err := c.Backend.ChatCompletion(ctx, messages, request.tools)
		if err != nil {
			c.logError(ctx, "chat_completion_failed", err, "iteration", iteration)
			return "", nil, err
		}

		// Add assistant's response to conversation
		messages = append(messages, response.Message)

		// Check finish reason
		switch response.FinishReason {
		case FinishReasonStop:
			// Normal completion, encode state (excluding system messages) and return
			c.logDebug(ctx, "chat_completed", "iteration", iteration)
			newState, err := c.encodeState(c.filterSystemMessages(messages))
			if err != nil {
				c.logError(ctx, "state_encoding_failed", err)
				return "", nil, err
			}
			return response.Message.Content, newState, nil

		case FinishReasonToolCalls:
			// Execute tools and continue loop
			c.logDebug(ctx, "executing_tools", "count", len(response.Message.ToolCalls))
			toolResults, err := c.executeTools(ctx, response.Message.ToolCalls, request.tools, toolLogger)
			if err != nil {
				c.logError(ctx, "tool_execution_failed", err)
				return "", nil, err
			}
			messages = append(messages, toolResults...)
			continue

		case FinishReasonLength:
			c.logError(ctx, "max_tokens_exceeded", nil)
			return "", nil, fmt.Errorf("conversation exceeded max tokens")

		default:
			c.logError(ctx, "unknown_finish_reason", nil, "reason", response.FinishReason)
			return "", nil, fmt.Errorf("unknown finish reason: %s", response.FinishReason)
		}
	}

	c.logError(ctx, "max_iterations_exceeded", nil, "max", maxIter)
	return "", nil, fmt.Errorf("exceeded max tool iterations (%d)", maxIter)
}

// buildMessages constructs the full message list for the API call.
// Order: system messages (from opts) + state history + new user messages (from opts)
// System messages are not persisted in state.
func (c *Chat) buildMessages(optMessages []Message, stateMessages []Message) []Message {
	// Separate system messages from other messages in options
	var systemMessages []Message
	var userMessages []Message
	for _, msg := range optMessages {
		if msg.Role == RoleSystem {
			systemMessages = append(systemMessages, msg)
		} else {
			userMessages = append(userMessages, msg)
		}
	}

	// Build final order: system + state + new user messages
	result := make([]Message, 0, len(systemMessages)+len(stateMessages)+len(userMessages))
	result = append(result, systemMessages...)
	result = append(result, stateMessages...)
	result = append(result, userMessages...)
	return result
}

// filterSystemMessages removes system messages from the message list.
// Used when encoding state to ensure system messages aren't persisted.
func (c *Chat) filterSystemMessages(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != RoleSystem {
			result = append(result, msg)
		}
	}
	return result
}

// Chat performs a stateless chat (existing behavior).
// This is a convenience wrapper around ChatWithState with nil state.
func (c *Chat) Chat(ctx context.Context, opts ...ChatOption) (string, error) {
	response, _, err := c.ChatWithState(ctx, nil, opts...)
	return response, err
}

// UpdateStateAfterEvent adds contextual information to conversation state
// without making an API call. Useful for injecting events like:
// - "User has just tagged Harrogate Theatre"
// - "Team score updated to 150 points"
// - "Game ended at 3:45pm"
//
// The event is added as a user message to the conversation history.
// Returns the updated state, or nil if the input state is invalid.
func (c *Chat) UpdateStateAfterEvent(
	ctx context.Context,
	state ConversationState,
	eventDescription string,
) ConversationState {
	// Decode existing state
	messages := c.decodeState(ctx, state)
	if messages == nil {
		messages = []Message{}
	}

	// Append event as a user message
	messages = append(messages, Message{
		Role:    RoleUser,
		Content: eventDescription,
	})

	// Encode and return new state
	newState, err := c.encodeState(messages)
	if err != nil {
		c.logError(ctx, "event_state_encoding_failed", err)
		return nil
	}

	return newState
}

// resolveMaxIterations determines the max iterations to use.
// Priority: 1) per-call option, 2) Chat.MaxToolIterations, 3) default (10)
func (c *Chat) resolveMaxIterations(override *int) int {
	if override != nil {
		return *override
	}
	if c.MaxToolIterations > 0 {
		return c.MaxToolIterations
	}
	return 10 // Default
}

// executeTools executes tool calls and returns tool result messages.
func (c *Chat) executeTools(ctx context.Context, toolCalls []ToolCall, tools aitooling.ToolSet, logger aitooling.Logger) ([]Message, error) {
	runner := tools.Runner(ctx, logger)

	var toolMessages []Message
	for _, call := range toolCalls {
		toolRequest := aitooling.ToolRequest{
			Name:   call.Name,
			Args:   call.Arguments,
			CallId: call.ID,
		}

		result, err := runner(&toolRequest)

		var resultContent string
		if err != nil {
			// Unexpected error (infrastructure failure, not domain error)
			resultContent = fmt.Sprintf("Error: %v", err)
			c.logError(ctx, "tool_execution_error", err,
				"tool_name", call.Name,
				"tool_id", call.ID,
			)
		} else {
			resultContent = result.Result
		}

		toolMessages = append(toolMessages, Message{
			Role:       RoleTool,
			Content:    resultContent,
			ToolCallID: call.ID,
		})
	}

	return toolMessages, nil
}

// logDebug logs a debug message if a SystemLogger is configured.
func (c *Chat) logDebug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if c.SystemLogger != nil {
		c.SystemLogger.Debug(ctx, msg, keysAndValues...)
	}
}

// logError logs an error message if a SystemLogger is configured.
func (c *Chat) logError(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	if c.SystemLogger != nil {
		c.SystemLogger.Error(ctx, msg, err, keysAndValues...)
	}
}

// encodeState serializes conversation state to an opaque blob.
func (c *Chat) encodeState(messages []Message) (ConversationState, error) {
	if c.Backend == nil {
		return nil, fmt.Errorf("backend is nil")
	}

	internal := conversationStateInternal{
		Version:  1,
		Provider: c.Backend.ProviderName(),
		Messages: messages,
	}

	data, err := json.Marshal(internal)
	if err != nil {
		return nil, fmt.Errorf("failed to encode conversation state: %w", err)
	}

	return ConversationState(data), nil
}

// decodeState deserializes conversation state from an opaque blob.
// Returns nil messages if state is nil, corrupted, or incompatible with current backend.
func (c *Chat) decodeState(ctx context.Context, state ConversationState) []Message {
	if state == nil || len(state) == 0 {
		return nil
	}

	var internal conversationStateInternal
	if err := json.Unmarshal(state, &internal); err != nil {
		c.logError(ctx, "invalid_conversation_state", err)
		return nil // Graceful degradation: start fresh conversation
	}

	// Validate version
	if internal.Version != 1 {
		c.logError(ctx, "unsupported_state_version", nil, "version", internal.Version)
		return nil // Graceful degradation: discard incompatible state
	}

	// Validate provider compatibility
	if c.Backend != nil && internal.Provider != c.Backend.ProviderName() {
		c.logError(ctx, "provider_mismatch", nil,
			"state_provider", internal.Provider,
			"current_provider", c.Backend.ProviderName())
		return nil // Graceful degradation: discard incompatible state
	}

	return internal.Messages
}

type dummyLogger struct{}

func (d dummyLogger) Log(action aitooling.ToolAction) {
	// Do Nothing
}

func (d dummyLogger) LogAll(actions []aitooling.ToolAction) {
	// Do Nothing
}
