# goaitools

A library for building AI-powered applications with tool-calling capabilities using OpenAI's function calling API.

`goaitools` is split out from a larger Scout Wide-Game Over WhatsApp project where it provides the natural language
interface for game organisers and players. Its development is currently guided by the requirements of that project.

## Features

- **Generic Tool Framework** - Define reusable AI tools with JSON schema validation
- **Automatic Tool-Calling Loop** - Chat layer orchestrates multi-turn conversations with tool execution
- **Backend Abstraction** - Interface-based design supports multiple AI providers
- **Action Logging** - Track tool executions for audit trails and user feedback
- **System Logging** - Optional context-aware debug logging via `log/slog`
- **Minimal Dependencies** - Only uses Go standard library

### Action Logging versus System Logging

As a user of the system I wanted to know that I could trust the AI when it had said it had made a change.
This captures changes made by the tools, so as a user interacting through WhatsApp I can see

```
I have set the game to start on Tuesday at 8pm and run for one hour.

*Actions Taken*

* Game Start set to Tuesday 16 December 20:00
* Game End set to Tuesday 16 December 21:00
```

This is distinct from system logging using `slog` for debug and other messages.

### Dynamic Tool Schema

The things that can be done with a game depend on the type of game being played. Tools provide their
own schema for the AI, determining dynamically which fields to offer. This is the one place I've found
coupling with the AI backend. OpenAI does not just accept any JSON Schema. It will be necessary to retest
all tool schema if the AI provider is changed.

## Installation

### As a Local Module (Development)

```bash
# In your go.mod, add:
replace github.com/m0rjc/goaitools => ./path/to/goaitools

# Then:
go get github.com/m0rjc/goaitools
go mod tidy
```

### As a Published Module (Future)

```bash
go get github.com/m0rjc/goaitools
```

## Quick Start

### Simple Chat Completion

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/m0rjc/goaitools"
    "github.com/m0rjc/goaitools/openai"
)

func main() {
    // Create OpenAI client
    client := openai.NewClient("your-api-key")
    if client == nil {
        log.Fatal("Failed to create OpenAI client")
    }

    // Create chat instance
    chat := &goaitools.Chat{Backend: client}

    // Send a simple message
    response, err := chat.Chat(
        context.Background(),
        goaitools.WithSystemMessage("You are a helpful assistant."),
        goaitools.WithUserMessage("What is the capital of France?"),
    )

    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response)
}
```

### Chat with Tools

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/m0rjc/goaitools"
    "github.com/m0rjc/goaitools/aitooling"
    "github.com/m0rjc/goaitools/openai"
)

// Define a simple tool
type WeatherTool struct{}

func (t *WeatherTool) Name() string {
    return "get_weather"
}

func (t *WeatherTool) Description() string {
    return "Get the current weather for a location"
}

func (t *WeatherTool) Parameters() json.RawMessage {
    schema := map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "The city name",
            },
        },
        "required": []string{"location"},
    }
    return aitooling.MustMarshalJSON(schema)
}

func (t *WeatherTool) Execute(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error) {
    // Parse arguments
    var args struct {
        Location string `json:"location"`
    }
    if err := json.Unmarshal(req.Args, &args); err != nil {
        return req.NewErrorResult(err), nil
    }

    // Simulate weather lookup
    result := fmt.Sprintf("The weather in %s is sunny and 72°F", args.Location)
    return req.NewResult(result), nil
}

func main() {
    client := openai.NewClient("your-api-key")
    chat := &goaitools.Chat{Backend: client}

    // Create tools
    tools := aitooling.ToolSet{&WeatherTool{}}

    // Chat with tools
    response, err := chat.Chat(
        context.Background(),
        goaitools.WithSystemMessage("You are a helpful weather assistant."),
        goaitools.WithUserMessage("What's the weather in Paris?"),
        goaitools.WithTools(tools),
    )

    if err != nil {
        panic(err)
    }

    fmt.Println(response)
}
```

### Configuring System Logging

The library supports optional system logging for debugging and monitoring the tool-calling loop:

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/m0rjc/goaitools"
    "github.com/m0rjc/goaitools/openai"
)

func main() {
    // Configure slog for debug level logging
    slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })))

    client := openai.NewClient("your-api-key")

    // Create chat with system logger
    chat := &goaitools.Chat{
        Backend:           client,
        SystemLogger:      goaitools.NewSlogSystemLogger(), // Logs to slog
        MaxToolIterations: 15,                              // Override default (10)
    }

    // The chat will now log events like:
    // - starting_chat_iteration
    // - executing_tools
    // - chat_completed
    // - tool_execution_error (if any)
    // - max_iterations_exceeded (if limit reached)

    response, err := chat.Chat(
        context.Background(),
        goaitools.WithUserMessage("Hello!"),
        goaitools.WithMaxToolIterations(5), // Override per-call
    )

    // Use NewSilentLogger() to disable system logging:
    // chat.SystemLogger = goaitools.NewSilentLogger()
}
```

**Logging Options:**

- `NewSlogSystemLogger()` - Logs to Go's standard `log/slog` (recommended)
- `NewSilentLogger()` - Disables all system logging
- Custom implementation of `SystemLogger` interface for advanced use cases

### Type-Safe Constants

The library provides type-safe constants for roles and finish reasons:

```go
// Message roles
goaitools.RoleSystem    // "system"
goaitools.RoleUser      // "user"
goaitools.RoleAssistant // "assistant"
goaitools.RoleTool      // "tool"

// Finish reasons
goaitools.FinishReasonStop      // "stop" - normal completion
goaitools.FinishReasonToolCalls // "tool_calls" - model wants to call tools
goaitools.FinishReasonLength    // "length" - max tokens reached
```

**Benefits:**
- IDE autocomplete for valid values
- Compile-time type safety
- Self-documenting code
- Easier refactoring

**Example:**
```go
// Create messages with type-safe roles
msg := goaitools.Message{
    Role:    goaitools.RoleSystem,
    Content: "You are a helpful assistant",
}

// Check finish reason with constants
if response.FinishReason == goaitools.FinishReasonToolCalls {
    // Handle tool calls
}
```

## Architecture

### Core Components

#### 1. Backend Interface

The `Backend` interface abstracts the AI provider for single-turn API calls:

```go
type Backend interface {
    // ChatCompletion makes a single API call and returns the response.
    // The response may contain tool_calls (requiring further iteration)
    // or a final text response (conversation complete).
    ChatCompletion(
        ctx context.Context,
        messages []Message,
        tools aitooling.ToolSet,
    ) (*ChatResponse, error)
}
```

**Current implementations:**
- `openai.Client` - OpenAI API backend

**Design philosophy:** Backends handle single-turn API calls. The `Chat` layer orchestrates the tool-calling loop.

#### 2. Tool Framework (`aitooling` package)

Define reusable AI tools:

```go
type Tool interface {
    Name() string                                      // Tool identifier
    Description() string                               // AI-readable description
    Parameters() json.RawMessage                       // JSON Schema for parameters
    Execute(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error)
}
```

**Key Features:**
- **Action Logging**: Tools can log actions for audit trails via `ctx.Logger`
- **Error Handling**: Return errors as `ToolResult` via `NewErrorResult()` for recoverable errors

#### 3. Chat Abstraction

The `Chat` type provides a high-level API with functional options:

```go
chat := &goaitools.Chat{Backend: openaiClient}

response, err := chat.Chat(
    ctx,
    goaitools.WithSystemMessage("system prompt"),
    goaitools.WithUserMessage("user question"),
    goaitools.WithTools(tools),
    goaitools.WithToolActionLogger(logger),
)
```

### Tool Execution Flow

1. **AI calls tool** → OpenAI returns tool_calls in response
2. **Tool execution** → `ToolSet.Runner()` executes the tool
3. **Result handling** → Tool returns result or error via `req.NewResult()` or `req.NewErrorResult()`
4. **Conversation continues** → Result added to conversation, AI generates final response

## Configuration

### OpenAI Client Options

```go
client := openai.NewClientWithOptions(
    apiKey,
    openai.WithModel("gpt-4"),
    openai.WithBaseURL("https://custom-endpoint.com"),
    openai.WithSystemLogger(goaitools.NewSlogSystemLogger()),
    openai.WithHTTPClient(customHTTPClient),
)
```

### Graceful Degradation

```go
// Client returns nil if API key is empty
client := openai.NewClient("")
if client == nil {
    // AI features disabled - fall back to alternative behavior
}
```

## Action Logging

Track tool executions for audit trails or user feedback:

```go
type MyAction struct {
    Description string
}

func (a MyAction) Description() string {
    return a.Description
}

// In your tool:
ctx.Logger.Log(MyAction{"Created new game 'Summer Hunt'"})
```

Collect logs during execution:

```go
var actionLogs []aitooling.ToolAction
logger := &myLogger{logs: &actionLogs}

response, err := chat.Chat(
    ctx,
    goaitools.WithToolActionLogger(logger),
    // ... other options
)

// Display actions to user
for _, action := range actionLogs {
    fmt.Println("• " + action.Description())
}
```

## Advanced Patterns

### Custom Backend

Implement the `Backend` interface to support other AI providers:

```go
type MyBackend struct {
    // ... your implementation
}

func (b *MyBackend) ChatCompletionWithTools(
    ctx context.Context,
    messages []goaitools.Message,
    tools aitooling.ToolSet,
    logger aitooling.Logger,
) (*goaitools.ChatResult, error) {
    // ... your implementation
}

// Use it
chat := &goaitools.Chat{Backend: &MyBackend{}}
```

### Application-Specific Tool Context

Pass application context through to tools:

```go
executeContext := aitooling.ToolExecuteContext{
    Context:     ctx,                // Go context
    ToolContext: myAppContext,       // Your application context
    Logger:      logger,
}

// In your tool's Execute method:
appCtx := ctx.ToolContext.(*MyAppContext)
// Use appCtx.Database, appCtx.RequestInfo, etc.
```

## Testing

The library is designed for easy testing:

```go
// Mock backend for testing
type MockBackend struct {
    Response string
}

func (m *MockBackend) ChatCompletionWithTools(...) (*goaitools.ChatResult, error) {
    return &goaitools.ChatResult{Content: m.Response}, nil
}

// Test with mock
chat := &goaitools.Chat{Backend: &MockBackend{Response: "test response"}}
```

## Error Handling

The library follows Go best practices:

- **Graceful degradation**: Returns nil for invalid configuration
- **Error wrapping**: Preserves error chains with `fmt.Errorf("%w")`
- **Context propagation**: Respects context cancellation and deadlines

## Related Projects

- [gowhatsapp](../gowhatsapp) - WhatsApp Cloud API integration
- Production usage: See MiniMonopoly game implementation in `src/webhook/routes/*/aitools/`

## License

MIT License - See [LICENSE](LICENSE) file for details

## Contributing

This library was extracted from the MiniMonopoly project. Contributions welcome!

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request
