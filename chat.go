package goaitools

import (
	"context"
	"fmt"

	"github.com/m0rjc/goaitools/aitooling"
)

type Chat struct {
	Backend           Backend
	MaxToolIterations int              // Default max iterations for tool-calling loop (0 = use default 10)
	SystemLogger      SystemLogger     // Optional logger for system/debug logging
	ToolActionLogger  aitooling.Logger // Optional default logger for tool actions
	LogToolArguments  bool             // If true, log tool call arguments and responses at DEBUG level
	Compactor         Compactor        // Optional compactor for managing conversation state size (nil = no compaction)
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
	stateMessages, _ := c.decodeState(ctx, state)

	// Build messages: system message (if any) + state history + new user messages
	messages := buildMessages(request.messages, stateMessages)

	// TODO: Consider if we want to perform a compaction run if messages were added since the last LLM call.
	// This would be cheap and effective for a max message length compactor, but expensive and possibly unnecessary
	// for a summarising compactor. A better approach may to to offer a SummarisePendingMessages method so that the
	// caller can decide.

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
			// Normal completion, compact if needed, then encode state and return
			c.logDebug(ctx, "chat_completed", "iteration", iteration)

			// Strip leading system messages from state
			stateMessages := stripLeadingSystemMessages(messages)

			// Compact if compactor is configured
			if c.Compactor != nil {
				compacted, err := c.Compactor.Compact(ctx, &CompactionRequest{
					StateMessages:         stateMessages,
					ProcessedLength:       len(stateMessages), // At this stage it is always all messages
					LeadingSystemMessages: extractLeadingSystemMessages(messages),
					LastAPIUsage:          response.Usage,
					Backend:               c.Backend,
				})
				if err != nil {
					c.logError(ctx, "compaction_failed", err)
					return "", nil, fmt.Errorf("compaction failed: %w", err)
				}
				if compacted.WasCompacted {
					c.logInfo(ctx, "conversation_compacted",
						"original_message_count", len(stateMessages),
						"compacted_message_count", len(compacted.StateMessages))
					stateMessages = compacted.StateMessages
				}
			}

			// Encode state
			newState, err := c.encodeState(stateMessages, len(stateMessages))
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

// Chat performs a stateless chat (existing behavior).
// This is a convenience wrapper around ChatWithState with nil state.
func (c *Chat) Chat(ctx context.Context, opts ...ChatOption) (string, error) {
	response, _, err := c.ChatWithState(ctx, nil, opts...)
	return response, err
}

// AppendToState adds messages to the state for storage. This is used to add contextual messages between user
// interactive AI calls without calling the LLM. For example if the user records arrival at a location in the
// game world this information can be logged so that they can ask about their location.
//
// Only message generation chat options are honoured. Tool and other options will be ignored.
// ALL specified messages are appended. Do not include the system message here.
// Claude recommends the use of User Messages to store information like "The user has arrived at The Railway Station".
func (c *Chat) AppendToState(ctx context.Context, state ConversationState, opts ...ChatOption) ConversationState {
	request := chatRequest{
		messages:    []Message{},
		tools:       aitooling.ToolSet{},
		logCallback: nil,
	}
	for _, opt := range opts {
		opt(&request, c.Backend) // Backend implements MessageFactory interface
	}

	// Decode existing state
	messages, processedLength := c.decodeState(ctx, state)
	if messages == nil {
		messages = []Message{}
	}

	// Append event as a user message using backend factory
	messages = append(messages, request.messages...)

	// Encode and return new state. Processed Length is preserved to not include the new messages
	newState, err := c.encodeState(messages, processedLength)
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

type dummyLogger struct{}

func (d dummyLogger) Log(_ aitooling.ToolAction) {
	// Do Nothing
}

func (d dummyLogger) LogAll(_ []aitooling.ToolAction) {
	// Do Nothing
}
