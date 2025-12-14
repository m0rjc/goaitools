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
type Message struct {
	Role       Role       `json:"role"`                   // "user", "assistant", "system", "tool"
	Content    string     `json:"content,omitempty"`      // Text content
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // Present when assistant wants to call tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // Present when role="tool"
}

// ToolCall represents a request to call a tool.
// This is a provider-agnostic representation - provider-specific fields
// (like OpenAI's "type") are handled by the backend implementation.
type ToolCall struct {
	ID        string `json:"id"`        // Unique identifier for this call
	Name      string `json:"name"`      // Name of the function to call
	Arguments string `json:"arguments"` // JSON arguments for the function
}

// ChatResponse represents a single API response from a chat completion.
// The response may contain tool_calls (requiring further iteration)
// or a final text response (conversation complete).
type ChatResponse struct {
	// Message is the assistant's response (may contain ToolCalls or Content)
	Message Message

	// FinishReason indicates why the model stopped
	FinishReason FinishReason
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
}
