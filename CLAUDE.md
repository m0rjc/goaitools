# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Project Overview

`goaitools` is a Go library for building AI-powered applications with tool-calling capabilities using OpenAI's function calling API. It provides a clean, generic framework for defining reusable AI tools with automatic tool-calling loops.

**Key Design Principles:**
- Backend abstraction: Interface-based design supports multiple AI providers
- Minimal dependencies: Only uses Go standard library
- Graceful degradation: Returns nil for invalid configuration (e.g., empty API key)

## Technology Stack

- **Language**: Go 1.25.4
- **Dependencies**: Standard library only (zero external dependencies)
- **Testing**: Go standard `testing` package

## Development Commands

```bash
# Run tests
go test ./...

# Run tests for a specific package
go test ./aitooling
go test ./openai

# Run tests with verbose output
go test -v ./...

# Run a single test
go test ./openai -run TestClientOptions

# Check module dependencies
go mod tidy

# Format code
go fmt ./...
```

## Project Structure

```
/
├── aitooling/              # Core tool framework (provider-agnostic)
│   ├── tool.go             # Tool interface and ToolSet
│   ├── executor.go         # ToolRunner execution logic
│   ├── logger.go           # Action logging (ToolAction, Logger)
│   └── schema.go           # JSON schema helpers
├── openai/                 # OpenAI-specific implementation
│   ├── client.go           # OpenAI API client
│   ├── types.go            # OpenAI API request/response types
│   ├── logger.go           # Logging abstraction
│   └── logger_test.go      # Tests for logger and client options
├── example/                # Working examples
│   └── hellowithtools/     # Complete demonstration of tool usage
├── backend.go              # Backend interface (provider abstraction)
├── chat.go                 # High-level Chat API with functional options
├── go.mod
├── LICENSE
└── README.md
```

## Working Example

See `example/hellowithtools/` for a complete working demonstration of:
- Creating and registering AI tools
- Using the Chat API with functional options
- Logging tool actions for audit and user feedback
- Transaction patterns for stateful tools
- JSON schema definition
- Error handling strategies

The example implements a simple game configuration system with read/write tools that demonstrate all key patterns in this library.

## Core Architecture

### 1. Backend Interface (Provider Abstraction)

The `Backend` interface abstracts AI providers. Currently only OpenAI is implemented, but the design supports adding other providers (Anthropic, Azure, etc.):

```go
type Backend interface {
    ChatCompletionWithTools(
        ctx context.Context,
        messages []Message,
        tools aitooling.ToolSet,
        logger aitooling.Logger,
    ) (*ChatResult, error)
}
```

**When adding new providers**: Implement this interface in a new package (e.g., `anthropic/`, `azure/`)

### 2. Tool Framework (`aitooling` package)

**Critical Design**: This package is **provider-agnostic** and should never import OpenAI-specific code.

```go
type Tool interface {
    Name() string                    // Tool identifier
    Description() string             // AI-readable description
    Parameters() json.RawMessage     // JSON Schema for parameters
    Execute(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error)
}
```

**Key Concepts:**

- **Result Creation**: Tools create results using helper methods:
  - `req.NewResult(result)` → Successful tool execution with a result string
  - `req.NewErrorResult(err)` → Tool execution encountered an error (allows AI to recover)

- **Action Logging**: Tools log actions via `ctx.Logger.Log()` for audit trails and user feedback

**Note on Transaction Awareness**: An earlier version included transaction-aware result methods (`NewReadOnlyResult()`, `NewDatabaseModifiedResult()`). This concept was removed as it wasn't found useful in practice - each tool now manages its own transactions as needed.

### 3. Automatic Tool-Calling Loop

The `openai.Client` implements a full conversation loop:

1. Send messages + tools to OpenAI
2. If response contains tool_calls → execute tools via `ToolSet.Runner()`
3. Append tool results to conversation
4. Repeat until model returns final text response (max 10 iterations)

**Implementation**: See `openai/client.go:chatCompletionWithToolsInternal()`

### 4. High-Level Chat API

The `Chat` type provides functional options pattern for easy usage. See `example/hellowithtools/main.go` for a complete example demonstrating:
- Creating tools with `aitooling.ToolSet`
- Configuring the Chat API with functional options
- Using `WithSystemMessage()`, `WithUserMessage()`, `WithTools()`, and `WithToolActionLogger()`

## Important Patterns

### Schema Definition with `MustMarshalJSON`

**Critical**: Use `aitooling.MustMarshalJSON()` for **compile-time** tool parameter schemas only. Never use on runtime/user data (will panic on error).

See `example/hellowithtools/write_game_tool.go:Parameters()` for a complete example of defining JSON Schema for tool parameters.

### Graceful Degradation Pattern

```go
// Client returns nil if API key is empty
client := openai.NewClient("")
if client == nil {
    // AI features disabled - fall back to alternative behavior
    return handleWithoutAI()
}
```

**When implementing features**: Always check for nil client and provide fallback behavior.

### Logging Abstraction

The OpenAI client uses a `Logger` interface (different from `aitooling.Logger`):

- **Default**: `slogLogger` (uses Go's standard `log/slog`)
- **Silent**: Use `openai.WithoutLogging()` option
- **Custom**: Use `openai.WithLogger(customLogger)` option

### Error Handling Strategy

- **Tool execution errors**: Return `req.NewErrorResult(err)` to pass error to AI (allows AI to recover)
- **Infrastructure errors**: Return error from `Execute()` for unexpected failures
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)` to preserve error chains

See `example/hellowithtools/write_game_tool.go:Execute()` for examples of all three error handling patterns in practice.

## Testing Guidelines

- Use Go standard `testing` package
- Test interface implementations with compile-time checks: `var _ Logger = slogLogger{}`
- Test functional options pattern (see `openai/logger_test.go`)
- Mock the `Backend` interface for testing without API calls
- Tests should check for behaviour and contracts rather than implementation details, to avoid a project that is hamstrung by over-coupled tests.

## Configuration Best Practices

**OpenAI Client Creation**:

```go
// Simple (uses defaults: gpt-4o-mini, 30s timeout)
client := openai.NewClient(apiKey)

// With custom options
client := openai.NewClientWithOptions(
    apiKey,
    openai.WithModel("gpt-4"),
    openai.WithBaseURL("https://custom-endpoint.com"),
    openai.WithLogger(customLogger),
    openai.WithHTTPClient(customHTTPClient),
)
```

**Default Configuration**:
- Model: `gpt-4o-mini` (cost-effective)
- Timeout: 30 seconds
- Base URL: `https://api.openai.com/v1`

## Known Limitations

### Stateless Conversations

Currently, each `Chat()` call is stateless - conversation history is not preserved between calls. This is acceptable for single-turn interactions but problematic for multi-turn conversations.

## Origins

This library was extracted from a WhatsApp-based Wide-Game playing project, where it powers AI-driven game interactions.

## Package Boundaries

**Critical Rules**:
- `aitooling/` MUST be provider-agnostic (no OpenAI imports)
- `openai/` contains OpenAI-specific implementation details
- Root package (`backend.go`, `chat.go`) provides high-level abstraction
