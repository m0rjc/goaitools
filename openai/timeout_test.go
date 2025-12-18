package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m0rjc/goaitools"
	"github.com/m0rjc/goaitools/aitooling"
)

// TestTimeoutBehavior demonstrates how context deadlines and HTTP client timeouts work together.
// The actual timeout is whichever limit is reached first.
func TestTimeoutBehavior(t *testing.T) {
	tests := []struct {
		name              string
		contextTimeout    time.Duration // 0 means no context timeout
		httpClientTimeout time.Duration // 0 means no HTTP client timeout
		serverDelay       time.Duration
		expectTimeout     bool
		timeoutSource     string // "context" or "http_client"
	}{
		{
			name:              "context_timeout_wins",
			contextTimeout:    100 * time.Millisecond,
			httpClientTimeout: 500 * time.Millisecond,
			serverDelay:       200 * time.Millisecond,
			expectTimeout:     true,
			timeoutSource:     "context",
		},
		{
			name:              "http_client_timeout_wins",
			contextTimeout:    500 * time.Millisecond,
			httpClientTimeout: 100 * time.Millisecond,
			serverDelay:       200 * time.Millisecond,
			expectTimeout:     true,
			timeoutSource:     "http_client",
		},
		{
			name:              "no_timeout_fast_response",
			contextTimeout:    500 * time.Millisecond,
			httpClientTimeout: 500 * time.Millisecond,
			serverDelay:       50 * time.Millisecond,
			expectTimeout:     false,
			timeoutSource:     "",
		},
		{
			name:              "both_timeouts_exceeded",
			contextTimeout:    100 * time.Millisecond,
			httpClientTimeout: 150 * time.Millisecond,
			serverDelay:       300 * time.Millisecond,
			expectTimeout:     true,
			timeoutSource:     "context", // context is shorter, so it wins
		},
		{
			name:              "no_context_timeout_http_client_times_out",
			contextTimeout:    0, // no context timeout
			httpClientTimeout: 100 * time.Millisecond,
			serverDelay:       200 * time.Millisecond,
			expectTimeout:     true,
			timeoutSource:     "http_client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that delays response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simulate processing delay
				time.Sleep(tt.serverDelay)

				// Return valid ChatCompletion response
				response := ChatCompletionResponse{
					ID:      "test-id",
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   "gpt-4o-mini",
					Choices: []Choice{
						{
							Index: 0,
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

			// Create HTTP client with or without timeout
			httpClient := &http.Client{}
			if tt.httpClientTimeout > 0 {
				httpClient.Timeout = tt.httpClientTimeout
			}

			// Create OpenAI client with custom HTTP client and base URL
			client, err := NewClientWithOptions(
				"test-api-key",
				WithBaseURL(server.URL),
				WithHTTPClient(httpClient),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Create context with or without timeout
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.contextTimeout > 0 {
				ctx, cancel = context.WithTimeout(context.Background(), tt.contextTimeout)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			// Make request
			messages := []Message{
				{Role: "user", Content: "Test message"},
			}
			start := time.Now()
			_, err = client.sendRequest(ctx, ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: messages,
			})
			elapsed := time.Since(start)

			// Verify timeout behavior
			if tt.expectTimeout {
				if err == nil {
					t.Errorf("Expected timeout error, but request succeeded (elapsed: %v)", elapsed)
				}

				// Verify the error came from the expected source
				switch tt.timeoutSource {
				case "context":
					// Context deadline exceeded error
					if ctx.Err() != context.DeadlineExceeded {
						t.Errorf("Expected context.DeadlineExceeded, got context error: %v, request error: %v", ctx.Err(), err)
					}
				case "http_client":
					// HTTP client timeout error (wrapped in request error)
					if err == nil {
						t.Error("Expected HTTP client timeout error, but got nil")
					}
					// Note: We don't check the specific error type here because both
					// context and HTTP client timeouts can manifest as context errors
					// when using NewRequestWithContext
				}

				t.Logf("Timed out as expected from %s (elapsed: %v)", tt.timeoutSource, elapsed)
			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v (elapsed: %v)", err, elapsed)
				}
				t.Logf("Succeeded as expected (elapsed: %v)", elapsed)
			}
		})
	}
}

// TestDefaultClientTimeout verifies the default 30-second timeout.
func TestDefaultClientTimeout(t *testing.T) {
	client, err := NewClient("test-api-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.httpClient.Timeout != defaultTimeout {
		t.Errorf("Expected default timeout of %v, got %v", defaultTimeout, client.httpClient.Timeout)
	}

	if defaultTimeout != 30*time.Second {
		t.Errorf("Expected defaultTimeout constant to be 30s, got %v", defaultTimeout)
	}
}

// TestCustomHTTPClientTimeout verifies custom HTTP client timeout configuration.
func TestCustomHTTPClientTimeout(t *testing.T) {
	customTimeout := 60 * time.Second
	customClient := &http.Client{
		Timeout: customTimeout,
	}

	client, err := NewClientWithOptions(
		"test-api-key",
		WithHTTPClient(customClient),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.httpClient.Timeout != customTimeout {
		t.Errorf("Expected custom timeout of %v, got %v", customTimeout, client.httpClient.Timeout)
	}
}

// TestNoHTTPClientTimeout verifies behavior when HTTP client has no timeout.
// This is NOT recommended for production use but demonstrates that context timeout still works.
func TestNoHTTPClientTimeout(t *testing.T) {
	// Create mock server with 200ms delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)

		response := ChatCompletionResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []Choice{
				{
					Index: 0,
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

	// Create client with no HTTP timeout
	noTimeoutClient := &http.Client{} // No Timeout field set
	client, err := NewClientWithOptions(
		"test-api-key",
		WithBaseURL(server.URL),
		WithHTTPClient(noTimeoutClient),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test 1: Context timeout still works even without HTTP client timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	messages := []Message{{Role: "user", Content: "Test"}}
	_, err = client.sendRequest(ctx, ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Messages: messages,
	})

	if err == nil {
		t.Error("Expected context timeout error, but request succeeded")
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", ctx.Err())
	}

	// Test 2: Without context timeout, request can succeed (despite no HTTP timeout)
	ctx2 := context.Background()
	_, err = client.sendRequest(ctx2, ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Messages: messages,
	})

	if err != nil {
		t.Errorf("Expected success without timeouts, got error: %v", err)
	}
}

// TestChatCompletionTimeout demonstrates timeout behavior at the ChatCompletion level.
func TestChatCompletionTimeout(t *testing.T) {
	// Create mock server with 200ms delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)

		response := ChatCompletionResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []Choice{
				{
					Index: 0,
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
		"test-api-key",
		WithBaseURL(server.URL),
		WithHTTPClient(&http.Client{Timeout: 500 * time.Millisecond}),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	userMsg := client.NewUserMessage("Test")
	_, err = client.ChatCompletion(ctx, []goaitools.Message{userMsg}, aitooling.ToolSet{})

	if err == nil {
		t.Error("Expected timeout error from ChatCompletion")
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", ctx.Err())
	}
}
