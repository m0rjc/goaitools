# OpenAI Model Configuration Guide

`goaitools` allows you to configure which OpenAI model to use and customize request parameters for optimal performance.

## Quick Start

### Default Behavior (gpt-4o-mini)

```go
import (
    "context"
    "github.com/m0rjc/goaitools"
    "github.com/m0rjc/goaitools/openai"
)

// Default: gpt-4o-mini with API default parameters
client, err := openai.NewClient(apiKey)
if err != nil {
    log.Fatal(err)
}

chat := &goaitools.Chat{Backend: client}
response, err := chat.Chat(context.Background(),
    goaitools.WithUserMessage("Hello"))
```

### Using a Different Model

```go
// Use GPT-4 for better reasoning
client, err := openai.NewClientWithOptions(
    apiKey,
    openai.WithModel("gpt-4"),
)
```

### Configuring Request Parameters

Request parameters control model behavior (temperature, token limits, etc.). These are merged into every API request.

```go
// Standard parameters
client, err := openai.NewClientWithOptions(
    apiKey,
    openai.WithModel("gpt-4o"),
    openai.WithTemperature(0.7), // Uses the temperature key. Some models may want a different key
    openai.WithMaxTokens(2048),
)
```

### Model-Specific Parameters

Different models may require different parameters. Use `WithRequestParam()` or `WithRequestParams()` for model-specific configuration.

```go
// Example: gpt-5-nano uses max_completion_tokens
client, err := openai.NewClientWithOptions(
    apiKey,
    openai.WithModel("gpt-5-nano"),
    openai.WithRequestParam("max_completion_tokens", 1500),
)

// Configure multiple parameters at once
client, err := openai.NewClientWithOptions(
    apiKey,
    openai.WithModel("gpt-4-turbo"),
    openai.WithRequestParams(map[string]interface{}{
        "temperature":           0.8,
        "max_completion_tokens": 4096,
        "top_p":                 0.95,
        "frequency_penalty":     0.0,
        "presence_penalty":      0.0,
    }),
)
```

### Sample Apps

The examples support runtime configuration via flags:

```bash
# Use a different model
./yourapp --model gpt-4

# Configure parameters (useful for new models)
./yourapp --model gpt-5-nano --request-params '{"max_completion_tokens":1500}'

# Combine model and parameters
./yourapp --model gpt-4-turbo --request-params '{"temperature":0.8,"max_tokens":2048}'
```

## Important Notes

- **No Parameters Configured**: If you don't set request parameters, the library omits them and OpenAI uses its defaults
- **Parameter Merging**: Configured parameters are merged into every request
- **No Overwriting**: Base request fields (model, messages, tools) are never overwritten by parameters

Refer to OpenAI's API documentation for the complete list and model-specific requirements.
