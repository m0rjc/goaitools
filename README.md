# goaitools

A library for building AI-powered applications with tool-calling capabilities using OpenAI's function calling API.

`goaitools` is split out from a larger Scout Wide-Game Over WhatsApp project where it provides the natural language
interface for game organisers and players. Its development is currently guided by the requirements of that project.

## Features

- **Generic Tool Framework** - Define reusable AI tools with JSON schema validation
- **Automatic Tool-Calling Loop** - Chat layer orchestrates multi-turn conversations with tool execution
- **Stateful Conversations** - Multi-turn conversations with automatic state management
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

See the examples in the examples directory in this project.

### Stateful Conversations

The library supports multi-turn conversations with automatic state management. This example shows a game assistant that remembers conversation history across multiple messages:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/m0rjc/goaitools"
    "github.com/m0rjc/goaitools/openai"
)

type Player struct {
    ID   string
    Name string
}

type Game struct {
    ID                string
    ConversationState goaitools.ConversationState // Opaque state blob
    LastInteraction   time.Time
}

func main() {
    client := openai.NewClient("your-api-key")
    chat := &goaitools.Chat{Backend: client}

    player := &Player{ID: "player1", Name: "Alice"}
    game := &Game{ID: "game123"}

    // First message - start new conversation
    fmt.Println("=== Turn 1 ===")
    response1, state1, _ := chat.ChatWithState(
        context.Background(),
        nil, // nil state = new conversation
        goaitools.WithSystemMessage(fmt.Sprintf("You are a game assistant. Current time: %s", time.Now().Format(time.RFC3339))),
        goaitools.WithUserMessage("What's my current score?"),
    )
    fmt.Printf("Assistant: %s\n\n", response1)
    game.ConversationState = state1
    game.LastInteraction = time.Now()

    // Event happens between turns
    time.Sleep(1 * time.Second)
    game.ConversationState = chat.UpdateStateAfterEvent(
        context.Background(),
        game.ConversationState,
        fmt.Sprintf("%s visited Harrogate Theatre and earned 50 points", player.Name),
    )

    // Second message - continues from previous state
    fmt.Println("=== Turn 2 ===")
    response2, state2, _ := chat.ChatWithState(
        context.Background(),
        game.ConversationState, // Previous state
        goaitools.WithSystemMessage(fmt.Sprintf("You are a game assistant. Current time: %s", time.Now().Format(time.RFC3339))),
        goaitools.WithUserMessage("What did I just do?"),
    )
    fmt.Printf("Assistant: %s\n\n", response2)
    game.ConversationState = state2
    game.LastInteraction = time.Now()

    // Third message - AI remembers full conversation
    fmt.Println("=== Turn 3 ===")
    response3, state3, _ := chat.ChatWithState(
        context.Background(),
        game.ConversationState,
        goaitools.WithSystemMessage(fmt.Sprintf("You are a game assistant. Current time: %s", time.Now().Format(time.RFC3339))),
        goaitools.WithUserMessage("What have we talked about so far?"),
    )
    fmt.Printf("Assistant: %s\n", response3)
    game.ConversationState = state3
}
```

**Output:**
```
=== Turn 1 ===
Assistant: I can help you check your score. However, I need access to the game system to retrieve that information...

=== Turn 2 ===
Assistant: You just visited Harrogate Theatre and earned 50 points!

=== Turn 3 ===
Assistant: We discussed your current score, then you visited Harrogate Theatre earning 50 points...
```

**Key Features:**

- **Opaque State**: State is `[]byte` - store in database, don't inspect it
- **System Messages Not Persisted**: Pass system message on every call (allows dynamic content like timestamps)
- **Graceful Degradation**: Invalid/corrupted state is silently discarded
- **Provider-Locked**: State from one provider (e.g., OpenAI) cannot be used with another
- **Event Updates**: Add context between turns using `UpdateStateAfterEvent()` without an LLM call

This follows [OpenAI's session memory pattern](https://cookbook.openai.com/examples/agents_sdk/session_memory) where:
- **Session state** = conversation history (user/assistant/tool messages)
- **System instructions** = passed on every turn (not stored in state)

**Backward Compatibility:**

The original stateless `Chat()` method still works - it's now a wrapper around `ChatWithState(ctx, nil, opts...)`.

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

    client, err := openai.NewClient("your-api-key")
    if err != nil {
        log.Fatal(err)
    }

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
client, err := openai.NewClientWithOptions(
    apiKey,
    openai.WithModel("gpt-4"),
    openai.WithBaseURL("https://custom-endpoint.com"),
    openai.WithSystemLogger(goaitools.NewSlogSystemLogger()),
    openai.WithHTTPClient(customHTTPClient),
)
if err != nil {
    log.Fatal(err)
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


## License

MIT License - See [LICENSE](LICENSE) file for details

## Contributing

This library was extracted from the MiniMonopoly project. Contributions welcome!

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request
