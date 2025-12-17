# Conversation State Management

This document explains how `goaitools` implements stateful multi-turn conversations.

## Overview

The `ChatWithState()` API enables multi-turn conversations with state persistence, similar to OpenAI's session memory pattern. State is stored as an opaque `[]byte` that clients can persist between calls.

## Design Inspiration

This implementation follows the pattern described in OpenAI's [Conversation Context Management](https://platform.openai.com/docs/guides/conversation-context) documentation, with the following principles:

1. **System messages are not stored in session state** - they should be provided fresh on each call
2. **State contains only conversation history** (user/assistant/tool messages)

## System Message Handling

### Leading System Messages (Not Persisted)

Leading system messages (those at the start of the message list) are treated as "preamble" and are **NOT** stored in state:

```go
// First turn
response, state, _ := chat.ChatWithState(ctx, nil,
    WithSystemMessage("You are a helpful assistant...."),
    WithUserMessage("Hello"),
)
// API receives: [SystemMsg("You are a ...."), UserMsg("Hello"), AssistantMsg("..."), UserMsg("What's the weather?")]
// State contains: [UserMsg("Hello"), AssistantMsg("...")]

// Second turn - provide fresh system message
response, state, _ := chat.ChatWithState(ctx, state,
    WithSystemMessage("Possibly updated but likely the same system message"),
    WithUserMessage("What's the weather?"),
)
// API receives: [SystemMsg("Possible updated..."), UserMsg("Hello"), AssistantMsg("..."), UserMsg("What's the weather?")]
// State contains: [UserMsg("Hello"), AssistantMsg("..."), UserMsg("What's the weather?"), AssistantMsg("Warm and sunny")]
```

### Mid-Conversation System Messages (Persisted)

Methods are provided to add context to the persisted state without requiring a user action. In the game-bot example
this could be the user checking into a location. This allows the user to later ask "Tell me about this place...".

System messages that appear **after** the first non-system message are treated as contextual events and **ARE** preserved:

Claude tells me that I should be using user messages for this, and has implemented the UpdateStateAfterEvent method this
way. The API intends to preserve developer intent, so will preserve system messages inserted into message chain,
maintaining order if they appear after any user messages.

```go
// Using UpdateStateAfterEvent (convenience method)
state = chat.UpdateStateAfterEvent(ctx, state, "User has checked in at Harrogate Theatre")
// This adds a user message to state (contextual events use user role)

// Or manually via WithSystemMessage after other messages
chat.ChatWithState(ctx, state,
    WithUserMessage("First message"),
    WithSystemMessage("User completed task X"), // This WILL be stored
    WithUserMessage("Next question"),
)
```

## State Format

State is stored as JSON with the following structure:

```json
{
  "version": 1,
  "provider": "openai",
  "messages": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ]
}
```

This is intended to be opaque to users of the API, and is subject to change.
The messages are raw messages received by the backend.

### Version Field

The `version` field allows detecting incompatible state format changes. Currently version 1.

### Provider Field

State is **provider-locked** - state created with OpenAI cannot be used with Anthropic. This prevents cross-provider compatibility issues.

## Forward Compatibility

The implementation uses an **opaque message design** to handle future provider API evolution:

### Message Interface

The Message interface allows the Chat tool calling loop and final response to work with opaque messages.

```go
type Message interface {
    Role() Role
    Content() string
    ToolCalls() []ToolCall
    ToolCallID() string
    MarshalJSON() ([]byte, error)  // Preserves ALL fields
}
```

Backends implement this interface and provide factory methods:

```go
type Backend interface {
    // ... existing methods ...

    NewSystemMessage(content string) Message
    NewUserMessage(content string) Message
    NewToolMessage(toolCallID, content string) Message
    UnmarshalMessage(data []byte) (Message, error)
}
```

## API Usage Patterns

See the examples folder for sample code. This code also acts as an integration test for the system.

## Error Handling

State decoding is **gracefully degrading**:

- **Invalid JSON**: Discarded, starts fresh conversation
- **Unsupported version**: Discarded with log error
- **Provider mismatch**: Discarded (can't use OpenAI state with Anthropic)
- **Corrupted messages**: Discarded

This ensures users never get stuck with bad state - worst case is they lose conversation history.

## References

- OpenAI Conversation Context: https://platform.openai.com/docs/guides/conversation-context
