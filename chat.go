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
	LogToolArguments  bool             // If true, log tool call arguments and responses at DEBUG level
}

// conversationStateInternal is the internal representation of conversation state.
// This is not exposed to clients - they only see the opaque []byte.
type conversationStateInternal struct {
	Version  int               `json:"version"`  // State format version (current: 1)
	Provider string            `json:"provider"` // Backend provider name (e.g., "openai")
	Messages []json.RawMessage `json:"messages"` // Conversation history (opaque provider-specific messages)
}

type chatRequest struct {
	messages          []Message
	tools             aitooling.ToolSet
	logCallback       aitooling.Logger
	maxToolIterations *int // Pointer to distinguish between "not set" and "set to 0"
}

// MessageFactory is the subset of Backend interface needed for creating messages.
// This follows the interface segregation principle.
type MessageFactory interface {
	NewSystemMessage(content string) Message
	NewUserMessage(content string) Message
	NewToolMessage(toolCallID, content string) Message
}

// ChatOption is a function that configures a chatRequest.
// It receives a MessageFactory to create provider-specific messages.
type ChatOption func(*chatRequest, MessageFactory)

func WithToolActionLogger(callback aitooling.Logger) ChatOption {
	return func(cfg *chatRequest, _ MessageFactory) {
		cfg.logCallback = callback
	}
}

func WithTools(tools aitooling.ToolSet) ChatOption {
	return func(cfg *chatRequest, _ MessageFactory) {
		cfg.tools = tools
	}
}

func WithSystemMessage(text string) ChatOption {
	return func(cfg *chatRequest, factory MessageFactory) {
		cfg.messages = append(cfg.messages, factory.NewSystemMessage(text))
	}
}

func WithUserMessage(text string) ChatOption {
	return func(cfg *chatRequest, factory MessageFactory) {
		cfg.messages = append(cfg.messages, factory.NewUserMessage(text))
	}
}

// WithMaxToolIterations sets the maximum number of tool-calling iterations for this chat request.
// This overrides the Chat.MaxToolIterations setting for this specific request.
func WithMaxToolIterations(max int) ChatOption {
	return func(cfg *chatRequest, _ MessageFactory) {
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
// Following OpenAI's session memory pattern, LEADING system messages (via WithSystemMessage)
// are NOT stored in state. They should be passed on every call and will be prepended
// to the stored conversation. This allows system prompts to include dynamic content
// (timestamps, user context) that updates each turn. When you call again with a new
// leading SystemMsg, it will be prepended to the stored state.
//
// Mid-conversation system messages (e.g., from WithSystemMessage after a WithUserMessage,
// or inline in the conversation flow) ARE preserved in state. This allows contextual
// system messages like "User has checked in at Location X" to be retained.
//
// Example: If you call with [SystemMsg, UserMsg, SystemMsg], state will contain
// [UserMsg, SystemMsg] - only the leading system message is stripped. On the next
// call with [NewSystemMsg, UserMsg2], the API receives [NewSystemMsg, UserMsg,
// SystemMsg, UserMsg2].
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
		opt(&request, c.Backend) // Backend implements MessageFactory interface
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
			// Normal completion, encode state (excluding leading system messages) and return
			c.logDebug(ctx, "chat_completed", "iteration", iteration)
			newState, err := c.encodeState(c.stripLeadingSystemMessages(messages))
			if err != nil {
				c.logError(ctx, "state_encoding_failed", err)
				return "", nil, err
			}
			return response.Message.Content(), newState, nil

		case FinishReasonToolCalls:
			// Execute tools and continue loop
			c.logDebug(ctx, "executing_tools", "iteration", iteration, "count", len(response.Message.ToolCalls()))
			toolResults, err := c.executeTools(ctx, iteration, response.Message.ToolCalls(), request.tools, toolLogger)
			if err != nil {
				c.logError(ctx, "tool_execution_failed", err, "iteration", iteration)
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
// Order: leading system messages from opts + state history + remaining non-system messages from opts
// This allows fresh system "preamble" on each call while preserving inline system messages in state.
func (c *Chat) buildMessages(optMessages []Message, stateMessages []Message) []Message {
	// Separate leading system messages from other messages in opts
	var leadingSystemMessages []Message
	var otherOptMessages []Message

	// Find all leading system messages
	foundNonSystem := false
	for _, msg := range optMessages {
		if !foundNonSystem && msg.Role() == RoleSystem {
			leadingSystemMessages = append(leadingSystemMessages, msg)
		} else {
			foundNonSystem = true
			otherOptMessages = append(otherOptMessages, msg)
		}
	}

	// Build final order: leading system + state + other opts
	result := make([]Message, 0, len(leadingSystemMessages)+len(stateMessages)+len(otherOptMessages))
	result = append(result, leadingSystemMessages...)
	result = append(result, stateMessages...)
	result = append(result, otherOptMessages...)
	return result
}

// stripLeadingSystemMessages removes only the leading system messages from the message list.
// Everything from the first non-system message onward is preserved (including any inline system messages).
// Used when encoding state - allows caller to provide fresh "preamble" system messages on each call
// while preserving mid-conversation system messages (like event notifications).
//
// Example: {1S, 2S, 3U, 4S, 5U} → {3U, 4S, 5U}
func (c *Chat) stripLeadingSystemMessages(messages []Message) []Message {
	// Find first non-system message
	firstNonSystem := -1
	for i, msg := range messages {
		if msg.Role() != RoleSystem {
			firstNonSystem = i
			break
		}
	}

	// If all messages are system messages (or empty), return empty
	if firstNonSystem == -1 {
		return nil
	}

	// Return slice from first non-system message onward
	return messages[firstNonSystem:]
}

// Chat performs a stateless chat (existing behavior).
// This is a convenience wrapper around ChatWithState with nil state.
func (c *Chat) Chat(ctx context.Context, opts ...ChatOption) (string, error) {
	response, _, err := c.ChatWithState(ctx, nil, opts...)
	return response, err
}

// UpdateStateAfterEvent adds contextual information to conversation state
// without making an API call. Useful for injecting events like:
// - "User has just visited Harrogate Theatre"
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

	// Append event as a user message using backend factory
	messages = append(messages, c.Backend.NewUserMessage(eventDescription))

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
func (c *Chat) executeTools(ctx context.Context, iteration int, toolCalls []ToolCall, tools aitooling.ToolSet, logger aitooling.Logger) ([]Message, error) {
	runner := tools.Runner(ctx, logger)

	var toolMessages []Message
	for idx, call := range toolCalls {
		// Log tool call execution at DEBUG level
		logFields := []interface{}{
			"iteration", iteration,
			"tool_call_index", idx,
			"tool_calls_count", len(toolCalls),
			"tool_name", call.Name,
			"tool_id", call.ID,
		}

		// Optionally include arguments for debugging
		if c.LogToolArguments {
			logFields = append(logFields, "tool_args", string(call.Arguments))
		}

		c.logDebug(ctx, "executing_tool_call", logFields...)

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
				"iteration", iteration,
				"tool_name", call.Name,
				"tool_id", call.ID,
			)
		} else {
			resultContent = result.Result
		}

		// Optionally log tool response for debugging
		if c.LogToolArguments {
			c.logDebug(ctx, "tool_response",
				"iteration", iteration,
				"tool_call_index", idx,
				"tool_name", call.Name,
				"tool_id", call.ID,
				"response", resultContent,
			)
		}

		toolMessages = append(toolMessages, c.Backend.NewToolMessage(call.ID, resultContent))
	}

	return toolMessages, nil
}

// logDebug logs a debug message if a SystemLogger is configured.
func (c *Chat) logDebug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if c.SystemLogger != nil {
		c.SystemLogger.Debug(ctx, msg, keysAndValues...)
	}
}

// logInfo logs an info message if a SystemLogger is configured.
func (c *Chat) logInfo(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if c.SystemLogger != nil {
		c.SystemLogger.Info(ctx, msg, keysAndValues...)
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

	// Serialize each message to json.RawMessage using provider's MarshalJSON
	rawMessages := make([]json.RawMessage, len(messages))
	for i, msg := range messages {
		data, err := msg.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("marshal message %d: %w", i, err)
		}
		rawMessages[i] = data
	}

	internal := conversationStateInternal{
		Version:  1,
		Provider: c.Backend.ProviderName(),
		Messages: rawMessages,
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

	// Deserialize each message using backend's UnmarshalMessage
	messages := make([]Message, len(internal.Messages))
	for i, raw := range internal.Messages {
		msg, err := c.Backend.UnmarshalMessage(raw)
		if err != nil {
			c.logError(ctx, "message_unmarshal_failed", err, "index", i)
			return nil // Graceful degradation: discard corrupted state
		}
		messages[i] = msg
	}

	return messages
}

type dummyLogger struct{}

func (d dummyLogger) Log(_ aitooling.ToolAction) {
	// Do Nothing
}

func (d dummyLogger) LogAll(_ []aitooling.ToolAction) {
	// Do Nothing
}
