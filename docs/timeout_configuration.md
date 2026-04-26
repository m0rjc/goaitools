# Timeout Configuration Guide

`goaitools` honours two types of timeouts that work together:

1. **Context Timeout** - The Context passed into the Chat method governs the whole cycle.
2. **Client Timeout** - The OpenAI client's HTTPClient governs each request to OpenAI

Different backend implementations may implement their own timeouts.

**Important:** Whichever timeout is shorter will be used. This gives you both global defaults and per-request control.

## Quick Start

### Default Behavior (30-second timeout)

```go
import (
    "context"
    "github.com/m0rjc/goaitools"
    "github.com/m0rjc/goaitools/openai"
)

// Default: 30-second timeout for all requests
client, err := openai.NewClient(apiKey)
if err != nil {
    log.Fatal(err)
}

chat := &goaitools.Chat{Backend: client}

// This request will timeout after 30 seconds
response, err := chat.Chat(context.Background(),
    goaitools.WithUserMessage("Tell me a story"))
```

### Changing the HTTP Timeout in the OpenAI Client

The OpenAI Client's timeout is configured as part of this HTTP Client.

```go
import "net/http"

// 60-second default timeout
customClient := &http.Client{
    Timeout: 60 * time.Second,
}

client, err := openai.NewClientWithOptions(
    apiKey,
    openai.WithHTTPClient(customClient),
)

chat := &goaitools.Chat{Backend: client}

// All requests now use 60s timeout by default
response, _ := chat.Chat(context.Background(),
    goaitools.WithUserMessage("Long request"))
```

### Chat Timeouts And Tool Calling Limits for Chat

The Context deadline passed to the Chat methods controls the entire agent cycle
including its multiple tool calls. The Chat class also provides a limit for the amount of
tool calling cycles performed. This is to prevent runaway loops. Max Iterations can be set on the
Client struct or for each request. It defaults to 10.

```go
chat := &goaitools.Chat{
	Backend: client,
	MaxToolIterations: 5
}
```

```go
response, err := chat.Chat(context.Background(),
    goaitools.WithUserMessage("Long request"),
	goaitools.WithMaxToolIterations(5))
```

60 seconds per HTTP request, 120 seconds for the entire process with up to
10 tool calling iterations.

```go
// 60-second default timeout
customClient := &http.Client{
    Timeout: 60 * time.Second,
}

client, err := openai.NewClientWithOptions(
    apiKey,
    openai.WithHTTPClient(customClient),
)

ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
defer cancel()

response, err := chat.Chat(ctx,
    goaitools.WithUserMessage("Long request"),
    goaitools.WithMaxToolIterations(10))
```

