package goaitools

import (
	"context"

	"github.com/m0rjc/goaitools/aitooling"
)

// Role represents the role of a message sender in the conversation.
type Role string

const (
	RoleSystem    Role = "system"    // System message (instructions for the AI)
	RoleUser      Role = "user"      // Message from the user
	RoleAssistant Role = "assistant" // Message from the AI assistant
	RoleTool      Role = "tool"      // Tool execution result
)

// FinishReason indicates why the model stopped generating.
type FinishReason string

const (
	FinishReasonStop      FinishReason = "stop"       // Normal completion
	FinishReasonToolCalls FinishReason = "tool_calls" // Model wants to call tools
	FinishReasonLength    FinishReason = "length"     // Max tokens reached
)

// Message represents a chat message (user, assistant, system, or tool).
// This is an opaque provider-specific type accessed through this interface.
// Backends create messages via factory methods and preserve provider-specific fields
// for perfect round-tripping through conversation state.
type Message interface {
	// Role returns the role of the message sender.
	Role() Role

	// Content returns the text content of the message.
	Content() string

	// ToolCalls returns any tool calls requested by the assistant.
	// Returns nil/empty slice for non-assistant messages.
	ToolCalls() []ToolCall

	// ToolCallID returns the ID of the tool call this message is responding to.
	// Only relevant when Role() == RoleTool.
	ToolCallID() string

	// MarshalJSON serializes the message, preserving all provider-specific fields
	// (including unknown future fields) for state persistence.
	MarshalJSON() ([]byte, error)
}

// ToolCall represents a request to call a tool.
// This is a provider-agnostic representation - provider-specific fields
// (like OpenAI's "type") are handled by the backend implementation.
type ToolCall struct {
	ID        string `json:"id"`        // Unique identifier for this call
	Name      string `json:"name"`      // Name of the function to call
	Arguments string `json:"arguments"` // JSON arguments for the function
}

// TokenUsage represents token consumption information from an API call.
// Backends that don't provide token usage will leave this nil.
type TokenUsage struct {
	PromptTokens     int // Tokens used in the prompt
	CompletionTokens int // Tokens used in the completion
	TotalTokens      int // Total tokens used (prompt + completion)
}

// ChatResponse represents a single API response from a chat completion.
// The response may contain tool_calls (requiring further iteration)
// or a final text response (conversation complete).
type ChatResponse struct {
	// Message is the assistant's response (may contain ToolCalls or Content)
	Message Message

	// FinishReason indicates why the model stopped
	FinishReason FinishReason

	// Usage contains token consumption information (may be nil if backend doesn't provide it)
	Usage *TokenUsage
}

// SystemLogger provides context-aware logging for library internals.
// Implementations can extract request IDs or other metadata from context.
type SystemLogger interface {
	Debug(ctx context.Context, msg string, keysAndValues ...interface{})
	Info(ctx context.Context, msg string, keysAndValues ...interface{})
	Error(ctx context.Context, msg string, err error, keysAndValues ...interface{})
}

// Backend represents an AI provider backend that can perform chat completions.
// Implementations should handle single-turn API calls - the Chat layer manages
// the tool-calling loop.
type Backend interface {
	// ChatCompletion makes a single API call and returns the response.
	// The response may contain tool_calls (requiring further iteration)
	// or a final text response (conversation complete).
	ChatCompletion(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error)

	// ProviderName returns the name of this backend provider (e.g., "openai", "anthropic").
	// Used for conversation state validation - state from one provider cannot be used with another.
	ProviderName() string

	// Message factory methods - backends create their own provider-specific types

	// NewSystemMessage creates a system message with the given content.
	NewSystemMessage(content string) Message

	// NewUserMessage creates a user message with the given content.
	NewUserMessage(content string) Message

	// NewToolMessage creates a tool result message.
	NewToolMessage(toolCallID, content string) Message

	// UnmarshalMessage reconstructs a message from its serialized form.
	// Used when loading conversation state. The data should come from
	// a previous call to Message.MarshalJSON().
	UnmarshalMessage(data []byte) (Message, error)
}
