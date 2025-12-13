package openai

import (
	"bytes"
	"context"
	"encoding/json"
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

// Client is an OpenAI API client.
type Client struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	systemLogger goaitools.SystemLogger // For system/debug logging
}

// NewClient creates a new OpenAI client with the given API key.
// If apiKey is empty, returns nil (AI features should gracefully degrade).
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return nil
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   defaultModel,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
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

// NewClientWithOptions creates a client with functional options.
func NewClientWithOptions(apiKey string, opts ...ClientOption) *Client {
	if apiKey == "" {
		return nil
	}

	client := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   defaultModel,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
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

	// Convert goaitools.Message to openai.Message
	openaiMessages := make([]Message, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = Message{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCalls:  convertToolCallsToOpenAI(msg.ToolCalls),
			ToolCallID: msg.ToolCallID,
		}
	}

	// Build request
	req := ChatCompletionRequest{
		Model:       c.model,
		Messages:    openaiMessages,
		Tools:       mapToolset(tools),
		Temperature: 0.7,
		MaxTokens:   1024,
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
	c.logSystemDebug(ctx, "openai_request_complete",
		"finish_reason", choice.FinishReason,
		"tool_calls_count", len(choice.Message.ToolCalls),
	)

	// Convert OpenAI message back to goaitools.Message
	responseMessage := goaitools.Message{
		Role:      goaitools.Role(choice.Message.Role),
		Content:   choice.Message.Content,
		ToolCalls: convertToolCallsFromOpenAI(choice.Message.ToolCalls),
	}

	return &goaitools.ChatResponse{
		Message:      responseMessage,
		FinishReason: goaitools.FinishReason(choice.FinishReason),
	}, nil
}

// sendRequest sends a single API request and returns the response.
func (c *Client) sendRequest(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
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
