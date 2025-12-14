// Package ai provides AI integration including OpenAI client and tool definitions.
package openai

import "encoding/json"

// ChatCompletionRequest represents a request to the OpenAI chat completion API.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"`                   // "system", "user", "assistant", or "tool"
	Content    string     `json:"content,omitempty"`      // Text content
	Name       string     `json:"name,omitempty"`         // Name (for tool messages)
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // Tool calls from assistant
	ToolCallID string     `json:"tool_call_id,omitempty"` // ID when responding to a tool call
}

// Tool represents a function that can be called by the model.
type Tool struct {
	Type     string   `json:"type"` // Always "function"
	Function Function `json:"function"`
}

// Function describes a function that can be called.
type Function struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ToolCall represents a tool call made by the assistant.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // Always "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function being called.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON arguments
}

// ChatCompletionResponse represents the API response.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents one completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"` // "stop", "tool_calls", "length", etc.
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ErrorResponse represents an error from the API.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
