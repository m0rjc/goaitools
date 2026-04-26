package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m0rjc/goaitools"
	"github.com/m0rjc/goaitools/aitooling"
)

// Test: NewClient with empty API key returns ErrMissingAPIKey
func TestNewClient_EmptyAPIKey_ReturnsError(t *testing.T) {
	client, err := NewClient("")

	if err != ErrMissingAPIKey {
		t.Errorf("Expected ErrMissingAPIKey, got %v", err)
	}

	if client != nil {
		t.Error("NewClient with empty API key should return nil client")
	}
}

// Test: NewClient with valid API key returns configured client
func TestNewClient_ValidAPIKey_ReturnsClient(t *testing.T) {
	client, err := NewClient("sk-test123")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client == nil {
		t.Fatal("NewClient with valid API key should return client")
	}

	if client.apiKey != "sk-test123" {
		t.Errorf("Expected apiKey='sk-test123', got '%s'", client.apiKey)
	}

	if client.model != defaultModel {
		t.Errorf("Expected default model=%s, got %s", defaultModel, client.model)
	}

	if client.baseURL != defaultBaseURL {
		t.Errorf("Expected default baseURL=%s, got %s", defaultBaseURL, client.baseURL)
	}

	if client.httpClient.Timeout != defaultTimeout {
		t.Errorf("Expected default timeout=%s, got %s", defaultTimeout, client.httpClient.Timeout)
	}
}

// Test: NewClientWithOptions with empty API key returns ErrMissingAPIKey
func TestNewClientWithOptions_EmptyAPIKey_ReturnsError(t *testing.T) {
	client, err := NewClientWithOptions("", WithModel("gpt-4"))

	if err != ErrMissingAPIKey {
		t.Errorf("Expected ErrMissingAPIKey, got %v", err)
	}

	if client != nil {
		t.Error("NewClientWithOptions with empty API key should return nil client")
	}
}

// Test: WithModel option sets custom model
func TestClientOptions_WithModel(t *testing.T) {
	client, err := NewClientWithOptions("sk-test", WithModel("gpt-4-turbo"))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client.model != "gpt-4-turbo" {
		t.Errorf("Expected model='gpt-4-turbo', got '%s'", client.model)
	}
}

// Test: WithBaseURL option sets custom base URL
func TestClientOptions_WithBaseURL(t *testing.T) {
	customURL := "https://custom-api.example.com/v1"
	client, err := NewClientWithOptions("sk-test", WithBaseURL(customURL))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client.baseURL != customURL {
		t.Errorf("Expected baseURL='%s', got '%s'", customURL, client.baseURL)
	}
}

// Test: WithHTTPClient option sets custom HTTP client
func TestClientOptions_WithHTTPClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	client, err := NewClientWithOptions("sk-test", WithHTTPClient(customClient))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client.httpClient != customClient {
		t.Error("Expected custom HTTP client to be set")
	}

	if client.httpClient.Timeout != 60*time.Second {
		t.Errorf("Expected timeout=60s, got %s", client.httpClient.Timeout)
	}
}

// Test: WithSystemLogger option sets custom logger
func TestClientOptions_WithSystemLogger(t *testing.T) {
	logger := goaitools.NewSilentLogger()

	client, err := NewClientWithOptions("sk-test", WithSystemLogger(logger))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client.systemLogger != logger {
		t.Error("Expected custom logger to be set")
	}
}

// Test: Multiple options can be combined
func TestClientOptions_MultipleOptions(t *testing.T) {
	customHTTP := &http.Client{Timeout: 45 * time.Second}
	logger := goaitools.NewSilentLogger()

	client, err := NewClientWithOptions(
		"sk-test",
		WithModel("gpt-4"),
		WithBaseURL("https://custom.example.com"),
		WithHTTPClient(customHTTP),
		WithSystemLogger(logger),
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client.model != "gpt-4" {
		t.Error("Model option not applied")
	}
	if client.baseURL != "https://custom.example.com" {
		t.Error("BaseURL option not applied")
	}
	if client.httpClient != customHTTP {
		t.Error("HTTPClient option not applied")
	}
	if client.systemLogger != logger {
		t.Error("SystemLogger option not applied")
	}
}

// Test: Client implements Backend interface
func TestClient_ImplementsBackendInterface(t *testing.T) {
	var _ goaitools.Backend = &Client{}
}

// Test: convertToolCallsToOpenAI preserves structure
func TestConvertToolCallsToOpenAI(t *testing.T) {
	input := []goaitools.ToolCall{
		{
			ID:        "call_abc123",
			Name:      "get_weather",
			Arguments: `{"location":"London"}`,
		},
	}

	result := convertToolCallsToOpenAI(input)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result))
	}

	call := result[0]

	if call.ID != "call_abc123" {
		t.Errorf("Expected ID='call_abc123', got '%s'", call.ID)
	}

	if call.Type != "function" {
		t.Errorf("Expected Type='function', got '%s'", call.Type)
	}

	if call.Function.Name != "get_weather" {
		t.Errorf("Expected Name='get_weather', got '%s'", call.Function.Name)
	}

	var args map[string]string
	json.Unmarshal([]byte(call.Function.Arguments), &args)
	if args["location"] != "London" {
		t.Error("Arguments not preserved")
	}
}

// Test: convertToolCallsFromOpenAI preserves structure
func TestConvertToolCallsFromOpenAI(t *testing.T) {
	input := []ToolCall{
		{
			ID:   "call_xyz789",
			Type: "function",
			Function: FunctionCall{
				Name:      "check_status",
				Arguments: `{"id":42}`,
			},
		},
	}

	result := convertToolCallsFromOpenAI(input)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result))
	}

	call := result[0]

	if call.ID != "call_xyz789" {
		t.Errorf("Expected ID='call_xyz789', got '%s'", call.ID)
	}

	if call.Name != "check_status" {
		t.Errorf("Expected Name='check_status', got '%s'", call.Name)
	}

	var args map[string]int
	json.Unmarshal([]byte(call.Arguments), &args)
	if args["id"] != 42 {
		t.Error("Arguments not preserved")
	}
}

// Test: convertToolCallsToOpenAI with nil/empty input
func TestConvertToolCallsToOpenAI_EmptyInput(t *testing.T) {
	result := convertToolCallsToOpenAI(nil)
	if result != nil {
		t.Error("Expected nil for nil input")
	}

	result = convertToolCallsToOpenAI([]goaitools.ToolCall{})
	if result != nil {
		t.Error("Expected nil for empty slice")
	}
}

// Test: convertToolCallsFromOpenAI with nil/empty input
func TestConvertToolCallsFromOpenAI_EmptyInput(t *testing.T) {
	result := convertToolCallsFromOpenAI(nil)
	if result != nil {
		t.Error("Expected nil for nil input")
	}

	result = convertToolCallsFromOpenAI([]ToolCall{})
	if result != nil {
		t.Error("Expected nil for empty slice")
	}
}

// Test: mapToolset converts aitooling.ToolSet to OpenAI format
func TestMapToolset(t *testing.T) {
	schema := aitooling.MustMarshalJSON(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{"type": "string"},
		},
	})

	tools := aitooling.ToolSet{
		&mockTool{
			name:        "get_weather",
			description: "Get the current weather",
			parameters:  schema,
		},
	}

	result := mapToolset(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	tool := result[0]

	if tool.Type != "function" {
		t.Errorf("Expected Type='function', got '%s'", tool.Type)
	}

	if tool.Function.Name != "get_weather" {
		t.Errorf("Expected Name='get_weather', got '%s'", tool.Function.Name)
	}

	if tool.Function.Description != "Get the current weather" {
		t.Errorf("Expected description to be preserved")
	}

	// Verify parameters are preserved
	var params map[string]interface{}
	json.Unmarshal(tool.Function.Parameters, &params)
	if params["type"] != "object" {
		t.Error("Parameters schema not preserved")
	}
}

// Test: Client with mock HTTP server
func TestClient_ChatCompletion_Integration(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Error("Authorization header not set correctly")
		}

		// Return mock response
		response := ChatCompletionResponse{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Hello from mock server",
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClientWithOptions(
		"sk-test",
		WithBaseURL(server.URL),
	)

	if err != nil {
		t.Fatalf("Expected no error creating client, got %v", err)
	}

	result, err := client.ChatCompletion(
		context.Background(),
		[]goaitools.Message{
			client.NewUserMessage("Test"),
		},
		aitooling.ToolSet{},
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Message.Content() != "Hello from mock server" {
		t.Errorf("Expected mock response, got '%s'", result.Message.Content())
	}

	if result.FinishReason != goaitools.FinishReasonStop {
		t.Errorf("Expected stop reason, got %s", result.FinishReason)
	}
}

// Test: WithTemperature option sets temperature in request defaults
func TestClientOptions_WithTemperature(t *testing.T) {
	client, err := NewClientWithOptions("sk-test", WithTemperature(0.5))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if temp, ok := client.requestDefaults["temperature"].(float64); !ok || temp != 0.5 {
		t.Errorf("Expected temperature=0.5, got %v", client.requestDefaults["temperature"])
	}
}

// Test: WithMaxTokens option sets max_tokens in request defaults
func TestClientOptions_WithMaxTokens(t *testing.T) {
	client, err := NewClientWithOptions("sk-test", WithMaxTokens(2048))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if maxTokens, ok := client.requestDefaults["max_tokens"].(int); !ok || maxTokens != 2048 {
		t.Errorf("Expected max_tokens=2048, got %v", client.requestDefaults["max_tokens"])
	}
}

// Test: WithRequestParam sets arbitrary parameter
func TestClientOptions_WithRequestParam(t *testing.T) {
	client, err := NewClientWithOptions("sk-test", WithRequestParam("max_completion_tokens", 1500))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if val, ok := client.requestDefaults["max_completion_tokens"].(int); !ok || val != 1500 {
		t.Errorf("Expected max_completion_tokens=1500, got %v", client.requestDefaults["max_completion_tokens"])
	}
}

// Test: WithRequestParams sets multiple parameters
func TestClientOptions_WithRequestParams(t *testing.T) {
	params := map[string]interface{}{
		"temperature":            0.8,
		"max_completion_tokens":  2000,
		"top_p":                  0.9,
	}

	client, err := NewClientWithOptions("sk-test", WithRequestParams(params))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if temp, ok := client.requestDefaults["temperature"].(float64); !ok || temp != 0.8 {
		t.Errorf("Expected temperature=0.8, got %v", client.requestDefaults["temperature"])
	}

	if maxComp, ok := client.requestDefaults["max_completion_tokens"].(int); !ok || maxComp != 2000 {
		t.Errorf("Expected max_completion_tokens=2000, got %v", client.requestDefaults["max_completion_tokens"])
	}

	if topP, ok := client.requestDefaults["top_p"].(float64); !ok || topP != 0.9 {
		t.Errorf("Expected top_p=0.9, got %v", client.requestDefaults["top_p"])
	}
}

// Test: Request parameters are merged into actual requests
func TestClient_RequestParametersMerged(t *testing.T) {
	var receivedRequest map[string]interface{}

	// Create mock server that captures the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request body
		json.NewDecoder(r.Body).Decode(&receivedRequest)

		// Return mock response
		response := ChatCompletionResponse{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Test response",
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClientWithOptions(
		"sk-test",
		WithBaseURL(server.URL),
		WithTemperature(0.3),
		WithMaxTokens(512),
		WithRequestParam("max_completion_tokens", 1024),
	)

	if err != nil {
		t.Fatalf("Expected no error creating client, got %v", err)
	}

	_, err = client.ChatCompletion(
		context.Background(),
		[]goaitools.Message{
			client.NewUserMessage("Test"),
		},
		aitooling.ToolSet{},
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify parameters were included in request
	if temp, ok := receivedRequest["temperature"].(float64); !ok || temp != 0.3 {
		t.Errorf("Expected temperature=0.3 in request, got %v", receivedRequest["temperature"])
	}

	if maxTokens, ok := receivedRequest["max_tokens"].(float64); !ok || maxTokens != 512 {
		t.Errorf("Expected max_tokens=512 in request, got %v", receivedRequest["max_tokens"])
	}

	if maxComp, ok := receivedRequest["max_completion_tokens"].(float64); !ok || maxComp != 1024 {
		t.Errorf("Expected max_completion_tokens=1024 in request, got %v", receivedRequest["max_completion_tokens"])
	}
}

// Test: NewClient initializes empty requestDefaults
func TestNewClient_InitializesRequestDefaults(t *testing.T) {
	client, err := NewClient("sk-test")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client.requestDefaults == nil {
		t.Error("Expected requestDefaults to be initialized")
	}

	if len(client.requestDefaults) != 0 {
		t.Errorf("Expected empty requestDefaults, got %d entries", len(client.requestDefaults))
	}
}

// Test: WithPayloadLogging option enables payload logging
func TestClientOptions_WithPayloadLogging(t *testing.T) {
	client, err := NewClientWithOptions("sk-test", WithPayloadLogging())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !client.payloadLogging {
		t.Error("Expected payloadLogging to be true")
	}
}

// Test: Payload logging logs request and response bodies
func TestClient_PayloadLogging_LogsRequestAndResponse(t *testing.T) {
	// Create a mock logger to capture debug logs
	mockLogger := &mockSystemLogger{
		debugLogs: make([]debugLogEntry, 0),
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock response
		response := ChatCompletionResponse{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Test response",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClientWithOptions(
		"sk-test",
		WithBaseURL(server.URL),
		WithSystemLogger(mockLogger),
		WithPayloadLogging(),
	)

	if err != nil {
		t.Fatalf("Expected no error creating client, got %v", err)
	}

	_, err = client.ChatCompletion(
		context.Background(),
		[]goaitools.Message{
			client.NewUserMessage("Test message"),
		},
		aitooling.ToolSet{},
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify that request body was logged
	foundRequestLog := false
	foundResponseLog := false

	for _, entry := range mockLogger.debugLogs {
		if entry.msg == "openai_request_body" {
			foundRequestLog = true
			// Verify that the body contains expected content
			if body, ok := entry.keysAndValues[1].(string); ok {
				if !strings.Contains(body, "Test message") {
					t.Error("Expected request body to contain user message")
				}
			} else {
				t.Error("Expected body to be a string")
			}
		}

		if entry.msg == "openai_response_body" {
			foundResponseLog = true
			// Verify that the body contains expected content
			if body, ok := entry.keysAndValues[3].(string); ok {
				if !strings.Contains(body, "Test response") {
					t.Error("Expected response body to contain assistant response")
				}
			} else {
				t.Error("Expected body to be a string")
			}
		}
	}

	if !foundRequestLog {
		t.Error("Expected request body to be logged")
	}

	if !foundResponseLog {
		t.Error("Expected response body to be logged")
	}
}

// Test: Without payload logging, request/response bodies are not logged
func TestClient_WithoutPayloadLogging_DoesNotLogBodies(t *testing.T) {
	// Create a mock logger to capture debug logs
	mockLogger := &mockSystemLogger{
		debugLogs: make([]debugLogEntry, 0),
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock response
		response := ChatCompletionResponse{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Test response",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClientWithOptions(
		"sk-test",
		WithBaseURL(server.URL),
		WithSystemLogger(mockLogger),
		// Note: NOT using WithPayloadLogging()
	)

	if err != nil {
		t.Fatalf("Expected no error creating client, got %v", err)
	}

	_, err = client.ChatCompletion(
		context.Background(),
		[]goaitools.Message{
			client.NewUserMessage("Test message"),
		},
		aitooling.ToolSet{},
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify that request/response bodies were NOT logged
	for _, entry := range mockLogger.debugLogs {
		if entry.msg == "openai_request_body" {
			t.Error("Expected request body NOT to be logged without payload logging enabled")
		}

		if entry.msg == "openai_response_body" {
			t.Error("Expected response body NOT to be logged without payload logging enabled")
		}
	}
}

// mockTool for testing
type mockTool struct {
	name        string
	description string
	parameters  json.RawMessage
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string         { return m.description }
func (m *mockTool) Parameters() json.RawMessage { return m.parameters }
func (m *mockTool) Execute(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error) {
	return req.NewResult("ok"), nil
}

// mockSystemLogger for testing
type mockSystemLogger struct {
	debugLogs []debugLogEntry
	infoLogs  []debugLogEntry
	errorLogs []errorLogEntry
}

type debugLogEntry struct {
	msg           string
	keysAndValues []interface{}
}

type errorLogEntry struct {
	msg           string
	err           error
	keysAndValues []interface{}
}

func (m *mockSystemLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	m.debugLogs = append(m.debugLogs, debugLogEntry{msg: msg, keysAndValues: keysAndValues})
}

func (m *mockSystemLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	m.infoLogs = append(m.infoLogs, debugLogEntry{msg: msg, keysAndValues: keysAndValues})
}

func (m *mockSystemLogger) Error(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	m.errorLogs = append(m.errorLogs, errorLogEntry{msg: msg, err: err, keysAndValues: keysAndValues})
}
