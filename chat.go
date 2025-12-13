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
	LogToolArguments  bool             // If true, log tool call arguments at DEBUG level
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

func (c *Chat) Chat(ctx context.Context, opts ...ChatOption) (string, error) {
	// Build configuration from options
	request := chatRequest{
		messages:    []Message{},
		tools:       aitooling.ToolSet{},
		logCallback: nil,
	}
	for _, opt := range opts {
		opt(&request)
	}

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

	// Make a copy of messages to build conversation
	messages := make([]Message, len(request.messages))
	copy(messages, request.messages)

	// Tool-calling loop (moved from backend to here)
	for iteration := 0; iteration < maxIter; iteration++ {
		c.logDebug(ctx, "starting_chat_iteration", "iteration", iteration)

		// Call backend for single turn
		response, err := c.Backend.ChatCompletion(ctx, messages, request.tools)
		if err != nil {
			c.logError(ctx, "chat_completion_failed", err, "iteration", iteration)
			return "", err
		}

		// Add assistant's response to conversation
		messages = append(messages, response.Message)

		// Check finish reason
		switch response.FinishReason {
		case FinishReasonStop:
			// Normal completion, return text
			c.logDebug(ctx, "chat_completed", "iteration", iteration)
			return response.Message.Content, nil

		case FinishReasonToolCalls:
			// Execute tools and continue loop
			c.logDebug(ctx, "executing_tools", "iteration", iteration, "count", len(response.Message.ToolCalls))
			toolResults, err := c.executeTools(ctx, iteration, response.Message.ToolCalls, request.tools, toolLogger)
			if err != nil {
				c.logError(ctx, "tool_execution_failed", err, "iteration", iteration)
				return "", err
			}
			messages = append(messages, toolResults...)
			continue

		case FinishReasonLength:
			c.logError(ctx, "max_tokens_exceeded", nil)
			return "", fmt.Errorf("conversation exceeded max tokens")

		default:
			c.logError(ctx, "unknown_finish_reason", nil, "reason", response.FinishReason)
			return "", fmt.Errorf("unknown finish reason: %s", response.FinishReason)
		}
	}

	c.logError(ctx, "max_iterations_exceeded", nil, "max", maxIter)
	return "", fmt.Errorf("exceeded max tool iterations (%d)", maxIter)
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

func (d dummyLogger) Log(action aitooling.ToolAction) {
	// Do Nothing
}

func (d dummyLogger) LogAll(actions []aitooling.ToolAction) {
	// Do Nothing
}
