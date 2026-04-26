# goaitools Examples

This directory contains complete working examples demonstrating the `goaitools` library features.

## Prerequisites

Set your OpenAI API key as an environment variable:

```bash
export OPENAI_API_KEY="sk-..."
```

Or create a `.env` file in the example directory:

```bash
echo "OPENAI_API_KEY=sk-..." > .env
```

## Available Examples

### 1. hellowithtools - AI Tool Calling

**Location:** `hellowithtools/`

Demonstrates the core tool-calling framework with a simple game configuration system.

**Key Features:**
- Creating and registering AI tools
- Using the Chat API with functional options
- Logging tool actions for audit trails
- Tool error handling strategies
- JSON schema definition for tool parameters

**Run:**
```bash
cd hellowithtools
go run .
```

### 2. hellowithstate - Stateful Conversations

**Location:** `hellowithstate/`

Shows how to maintain conversation context across multiple turns with a travel planning assistant.

**Key Features:**
- Multi-turn conversations with `ChatWithState()`
- State persistence between turns
- Dynamic system messages (not stored in state)
- Using `AppendToState()` to add context without API calls
- State continuity and round-tripping

**Run:**
```bash
cd hellowithstate
go run .
```

### 3. statecompaction - Conversation State Management

**Location:** `statecompaction/`

Demonstrates automatic conversation state compaction to prevent unbounded growth.

**Key Features:**
- `MessageLimitCompactor` - Keep last N messages
- `TokenLimitCompactor` - Manage by token usage
- `CompositeCompactor` - Combine multiple strategies
- State size management over long conversations

**Run:**
```bash
cd statecompaction
go run .
```

## Command-Line Parameters

All examples support the following command-line flags:

### `--model`

Specify which OpenAI model to use (default: `gpt-4o-mini`)

```bash
go run . --model gpt-4
go run . --model gpt-4-turbo
go run . --model gpt-3.5-turbo
```

### `--request-params`

Provide a JSON string of request parameters to merge into API calls. Use this for model-specific configuration like temperature, token limits, and other parameters.

**Basic Parameters:**
```bash
# Set temperature and max_tokens
go run . --request-params '{"temperature":0.7,"max_tokens":2048}'

# Configure top_p
go run . --request-params '{"temperature":0.8,"top_p":0.95}'
```

**Model-Specific Parameters:**
```bash
# For models that prefer max_completion_tokens (e.g., gpt-5-nano)
go run . --model gpt-5-nano --request-params '{"max_completion_tokens":1500}'

# Omit temperature for models that don't support it
go run . --model some-model --request-params '{"max_tokens":2048}'
```

**Combined Options:**
```bash
# Combine model selection with custom parameters
go run . --model gpt-4 --request-params '{"temperature":0.8,"max_tokens":4096}'
```

## Common Request Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `temperature` | float (0.0-2.0) | Controls randomness. Higher = more random. |
| `max_tokens` | integer | Maximum tokens in completion (standard parameter). |
| `max_completion_tokens` | integer | Maximum completion tokens (some newer models). |
| `top_p` | float (0.0-1.0) | Nucleus sampling threshold. |
| `frequency_penalty` | float (-2.0-2.0) | Penalize repeated tokens. |
| `presence_penalty` | float (-2.0-2.0) | Penalize tokens that have appeared. |

**Note:** Not all models support all parameters. Consult the OpenAI documentation for your specific model.

## Building Examples

Build any example:

```bash
cd <example-directory>
go build
./<example-name>
```

Or use `go run .` to build and run in one step.

## Shared Setup Code

The `shared/` package provides common utilities used by all examples:

- `.env` file loading
- OpenAI client creation with command-line flag support
- Consistent configuration across examples

## Support

For issues, questions, or contributions, visit the [goaitools repository](https://github.com/m0rjc/goaitools).
