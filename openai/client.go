package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/m0rjc/goaitools"
	"github.com/m0rjc/goaitools/aitooling"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "gpt-4o-mini"
	defaultTimeout = 30 * time.Second
)

// ErrMissingAPIKey is returned when attempting to create a client with an empty API key.
var ErrMissingAPIKey = errors.New("API key is required")

// Client is an OpenAI API client.
type Client struct {
	apiKey         string
	baseURL        string
	model          string
	httpClient     *http.Client
	systemLogger   goaitools.SystemLogger    // For system/debug logging
	requestDefaults map[string]interface{}    // Default request parameters (temperature, max_tokens, etc.)
}

// NewClient creates a new OpenAI client with the given API key.
// Returns ErrMissingAPIKey if apiKey is empty.
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   defaultModel,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		requestDefaults: make(map[string]interface{}),
	}, nil
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithSystemLogger sets a custom system logger for the client.
func WithSystemLogger(logger goaitools.SystemLogger) ClientOption {
	return func(c *Client) {
		c.systemLogger = logger
	}
}

// WithBaseURL sets a custom base URL for the OpenAI API.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithModel sets a custom model for completions.
func WithModel(model string) ClientOption {
	return func(c *Client) {
		c.model = model
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTemperature sets the default temperature for requests.
func WithTemperature(temperature float64) ClientOption {
	return func(c *Client) {
		c.requestDefaults["temperature"] = temperature
	}
}

// WithMaxTokens sets the default max_tokens for requests.
func WithMaxTokens(maxTokens int) ClientOption {
	return func(c *Client) {
		c.requestDefaults["max_tokens"] = maxTokens
	}
}

// WithRequestParam sets an arbitrary request parameter.
// Use this for model-specific parameters like max_completion_tokens.
func WithRequestParam(key string, value interface{}) ClientOption {
	return func(c *Client) {
		c.requestDefaults[key] = value
	}
}

// WithRequestParams sets multiple request parameters at once.
func WithRequestParams(params map[string]interface{}) ClientOption {
	return func(c *Client) {
		for k, v := range params {
			c.requestDefaults[k] = v
		}
	}
}

// NewClientWithOptions creates a client with functional options.
// Returns ErrMissingAPIKey if apiKey is empty.
func NewClientWithOptions(apiKey string, opts ...ClientOption) (*Client, error) {
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	client := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   defaultModel,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		requestDefaults: make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// ProviderName returns the provider name for this backend.
func (c *Client) ProviderName() string {
	return "openai"
}

// Message factory methods - create provider-specific messages

// NewSystemMessage creates a system message with the given content.
func (c *Client) NewSystemMessage(content string) goaitools.Message {
	msg, _ := newMessage(Message{Role: "system", Content: content})
	return msg
}

// NewUserMessage creates a user message with the given content.
func (c *Client) NewUserMessage(content string) goaitools.Message {
	msg, _ := newMessage(Message{Role: "user", Content: content})
	return msg
}

// NewToolMessage creates a tool result message.
func (c *Client) NewToolMessage(toolCallID, content string) goaitools.Message {
	msg, _ := newMessage(Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	})
	return msg
}

// UnmarshalMessage reconstructs a message from its serialized form.
// Used when loading conversation state.
func (c *Client) UnmarshalMessage(data []byte) (goaitools.Message, error) {
	return unmarshalMessage(data)
}

// ChatCompletion makes a single API call and returns the response.
// The response may contain tool_calls (requiring further iteration)
// or a final text response (conversation complete).
// This is the preferred method - the Chat layer handles the tool-calling loop.
func (c *Client) ChatCompletion(
	ctx context.Context,
	messages []goaitools.Message,
	tools aitooling.ToolSet,
) (*goaitools.ChatResponse, error) {
	c.logSystemDebug(ctx, "openai_request_start", "model", c.model, "message_count", len(messages))

	// Extract OpenAI messages from interface
	openaiMessages := make([]Message, len(messages))
	for i, msg := range messages {
		// If it's our own message type, use parsed directly for efficiency
		if m, ok := msg.(*message); ok {
			openaiMessages[i] = m.parsed
		} else {
			// Fallback: reconstruct from interface (shouldn't happen in normal flow)
			openaiMessages[i] = Message{
				Role:       string(msg.Role()),
				Content:    msg.Content(),
				ToolCalls:  convertToolCallsToOpenAI(msg.ToolCalls()),
				ToolCallID: msg.ToolCallID(),
			}
		}
	}

	// Build request
	req := ChatCompletionRequest{
		Model:    c.model,
		Messages: openaiMessages,
		Tools:    mapToolset(tools),
	}

	// Make ONE API call (no loop!)
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		c.logSystemError(ctx, "openai_request_failed", err)
		return nil, err
	}

	if len(resp.Choices) == 0 {
		err := fmt.Errorf("no choices returned from API")
		c.logSystemError(ctx, "openai_no_choices", err)
		return nil, err
	}

	choice := resp.Choices[0]
	c.logSystemDebug(ctx, "openai_response",
		"finish_reason", choice.FinishReason,
		"tool_calls_count", len(choice.Message.ToolCalls),
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens,
		"total_tokens", resp.Usage.TotalTokens,
	)

	// Wrap the OpenAI message in our message type
	// We need to preserve the raw JSON from the response
	rawJSON, err := json.Marshal(choice.Message)
	if err != nil {
		return nil, fmt.Errorf("marshal response message: %w", err)
	}

	responseMessage := &message{
		rawJSON: rawJSON,
		parsed:  choice.Message,
	}

	return &goaitools.ChatResponse{
		Message:      responseMessage,
		FinishReason: goaitools.FinishReason(choice.FinishReason),
		Usage: &goaitools.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// sendRequest sends a single API request and returns the response.
func (c *Client) sendRequest(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Marshal base request to JSON, then merge with defaults
	body, err := c.mergeRequestDefaults(req)
	if err != nil {
		return nil, fmt.Errorf("prepare request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// mergeRequestDefaults marshals the base request and merges in requestDefaults.
// This allows arbitrary model-specific parameters to be added to requests.
func (c *Client) mergeRequestDefaults(req ChatCompletionRequest) ([]byte, error) {
	// Marshal base request to map
	baseJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal base request: %w", err)
	}

	var requestMap map[string]interface{}
	if err := json.Unmarshal(baseJSON, &requestMap); err != nil {
		return nil, fmt.Errorf("unmarshal to map: %w", err)
	}

	// Merge defaults (only if not already set in base request)
	for key, value := range c.requestDefaults {
		if _, exists := requestMap[key]; !exists {
			requestMap[key] = value
		}
	}

	// Marshal merged request
	return json.Marshal(requestMap)
}

// mapToolset converts aitooling.ToolSet to OpenAI API tool format.
func mapToolset(tools aitooling.ToolSet) []Tool {
	result := make([]Tool, len(tools))
	for i, tool := range tools {
		result[i] = Tool{
			Type: "function",
			Function: Function{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		}
	}
	return result
}

// logSystemDebug logs a debug message using the system logger (if configured).
func (c *Client) logSystemDebug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if c.systemLogger != nil {
		c.systemLogger.Debug(ctx, msg, keysAndValues...)
	}
}

// logSystemInfo logs an info message using the system logger (if configured).
func (c *Client) logSystemInfo(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if c.systemLogger != nil {
		c.systemLogger.Info(ctx, msg, keysAndValues...)
	}
}

// logSystemError logs an error message using the system logger (if configured).
func (c *Client) logSystemError(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	if c.systemLogger != nil {
		c.systemLogger.Error(ctx, msg, err, keysAndValues...)
	}
}

// convertToolCallsToOpenAI converts goaitools.ToolCall to openai.ToolCall.
func convertToolCallsToOpenAI(toolCalls []goaitools.ToolCall) []ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	result := make([]ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = ToolCall{
			ID:   tc.ID,
			Type: "function", // OpenAI-specific: always "function"
			Function: FunctionCall{
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		}
	}
	return result
}

// convertToolCallsFromOpenAI converts openai.ToolCall to goaitools.ToolCall.
func convertToolCallsFromOpenAI(toolCalls []ToolCall) []goaitools.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	result := make([]goaitools.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = goaitools.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
			// Type field is OpenAI-specific, not included in generic ToolCall
		}
	}
	return result
}
