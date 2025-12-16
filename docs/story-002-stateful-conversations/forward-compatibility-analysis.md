# Forward Compatibility Analysis: Provider Message Evolution

**Date**: 2025-12-16
**Status**: Exploratory Discussion
**Context**: Ensuring state doesn't break when OpenAI/providers add new message fields (e.g., GPT-5)

## The Problem

### Current Architecture Vulnerability

The current design has a **forward compatibility issue** when providers introduce new message fields:

```
┌─────────────────────────────────────────────────────────────┐
│ 1. OpenAI Response (GPT-5 with new fields)                  │
│    {role: "assistant", content: "...",                       │
│     reasoning_content: "...",  ← NEW FIELD                   │
│     confidence: 0.95}          ← NEW FIELD                   │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Conversion to Generic Type (client.go:156-161)           │
│    goaitools.Message{                                        │
│        Role:      Role(msg.Role),                            │
│        Content:   msg.Content,                               │
│        ToolCalls: convertToolCalls(...),                     │
│    }                                                         │
│    ❌ reasoning_content LOST                                 │
│    ❌ confidence LOST                                         │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. State Saved (chat.go:146)                                 │
│    conversationStateInternal{                                │
│        Messages: []Message (truncated)                       │
│    }                                                         │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. State Loaded & Sent Back (next turn)                     │
│    ❌ New fields gone forever                                │
│    ❌ Context lost between turns                             │
└─────────────────────────────────────────────────────────────┘
```

### Failure Scenarios

1. **Reasoning Context Loss** (GPT-5 chain-of-thought)
   - Provider returns `reasoning_content` field
   - Lost in conversion → state can't maintain reasoning thread

2. **Metadata Loss** (confidence scores, citations)
   - Provider adds `confidence: 0.95`, `sources: [...]`
   - Lost in conversion → UI can't display metadata

3. **Multimodal Content** (structured content arrays)
   - Provider evolves from `content: string` to `content: [{type: "text", text: "..."}, {type: "image_url", url: "..."}]`
   - Current `Content string` field can't represent this

### Why Current Protections Don't Help

- **Version field** (chat.go:24): Only detects format changes WE control, not provider evolution
- **Provider field** (chat.go:325): Prevents cross-provider state use, but not version skew within same provider

## Solution: Messages as Provider-Specific Opaques

### Core Insight

**Don't convert messages to a generic type** - keep them provider-specific but access through an interface.

### Design Principles

1. **Backend owns its message types** - each provider has native representation
2. **Chat accesses via interface** - read-only view of what it needs
3. **Perfect round-tripping** - serialize/deserialize preserves ALL fields
4. **No split-brain** - single source of truth (provider's native type)

## Proposed Architecture

### 1. Message Interface (backend.go)

```go
// Message is an opaque provider-specific message type.
// Chat accesses it through this interface, but never inspects internals.
type Message interface {
    // Read-only accessors for what Chat needs
    Role() Role
    Content() string
    ToolCalls() []ToolCall
    ToolCallID() string

    // Serialization - provider controls format
    MarshalJSON() ([]byte, error)
}
```

### 2. Backend Interface Updates (backend.go)

```go
type Backend interface {
    ChatCompletion(ctx context.Context, messages []Message, tools aitooling.ToolSet) (*ChatResponse, error)
    ProviderName() string

    // Message factories - backend creates its own types
    NewSystemMessage(content string) Message
    NewUserMessage(content string) Message
    NewToolMessage(toolCallID, content string) Message

    // Deserialization - backend reconstructs its types
    UnmarshalMessage(data []byte) (Message, error)
}

type ChatResponse struct {
    Message      Message      // Provider-specific, accessed via interface
    FinishReason FinishReason
}
```

### 3. State Format Changes (chat.go)

```go
type conversationStateInternal struct {
    Version  int               `json:"version"`
    Provider string            `json:"provider"`
    Messages []json.RawMessage `json:"messages"` // Changed from []Message
}
```

### 4. State Encoding/Decoding (chat.go)

```go
// Encoding - delegate to provider's MarshalJSON
func (c *Chat) encodeState(messages []Message) (ConversationState, error) {
    rawMessages := make([]json.RawMessage, len(messages))
    for i, msg := range messages {
        data, err := msg.MarshalJSON()  // Provider serializes
        if err != nil {
            return nil, fmt.Errorf("marshal message %d: %w", i, err)
        }
        rawMessages[i] = data
    }

    internal := conversationStateInternal{
        Version:  1,
        Provider: c.Backend.ProviderName(),
        Messages: rawMessages,
    }
    return json.Marshal(internal)
}

// Decoding - delegate to provider's UnmarshalMessage
func (c *Chat) decodeState(ctx context.Context, state ConversationState) []Message {
    if state == nil || len(state) == 0 {
        return nil
    }

    var internal conversationStateInternal
    if err := json.Unmarshal(state, &internal); err != nil {
        c.logError(ctx, "invalid_conversation_state", err)
        return nil
    }

    // Validate version and provider (same as before)
    if internal.Version != 1 {
        c.logError(ctx, "unsupported_state_version", nil, "version", internal.Version)
        return nil
    }

    if c.Backend != nil && internal.Provider != c.Backend.ProviderName() {
        c.logError(ctx, "provider_mismatch", nil,
            "state_provider", internal.Provider,
            "current_provider", c.Backend.ProviderName())
        return nil
    }

    // Backend reconstructs its own message types
    messages := make([]Message, len(internal.Messages))
    for i, raw := range internal.Messages {
        msg, err := c.Backend.UnmarshalMessage(raw)
        if err != nil {
            c.logError(ctx, "message_unmarshal_failed", err, "index", i)
            return nil
        }
        messages[i] = msg
    }

    return messages
}
```

### 5. OpenAI Implementation

#### New file: `openai/message.go`

```go
package openai

import (
    "encoding/json"
    "github.com/m0rjc/goaitools"
)

// message wraps the OpenAI-specific Message type.
// This preserves ALL OpenAI fields (including future ones) for round-tripping.
type message struct {
    raw Message  // From types.go - complete OpenAI message
}

// Interface implementation - read-only views
func (m *message) Role() goaitools.Role {
    return goaitools.Role(m.raw.Role)
}

func (m *message) Content() string {
    return m.raw.Content
}

func (m *message) ToolCalls() []goaitools.ToolCall {
    return convertToolCallsFromOpenAI(m.raw.ToolCalls)
}

func (m *message) ToolCallID() string {
    return m.raw.ToolCallID
}

// MarshalJSON preserves ALL fields (including unknown future fields)
func (m *message) MarshalJSON() ([]byte, error) {
    return json.Marshal(m.raw)
}

// Compile-time interface check
var _ goaitools.Message = (*message)(nil)
```

#### Updates to `openai/client.go`

```go
// Factory methods
func (c *Client) NewSystemMessage(content string) goaitools.Message {
    return &message{raw: Message{Role: "system", Content: content}}
}

func (c *Client) NewUserMessage(content string) goaitools.Message {
    return &message{raw: Message{Role: "user", Content: content}}
}

func (c *Client) NewToolMessage(toolCallID, content string) goaitools.Message {
    return &message{raw: Message{
        Role:       "tool",
        Content:    content,
        ToolCallID: toolCallID,
    }}
}

// Deserialization
func (c *Client) UnmarshalMessage(data []byte) (goaitools.Message, error) {
    var raw Message
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("unmarshal OpenAI message: %w", err)
    }
    return &message{raw: raw}, nil
}

// ChatCompletion updates
func (c *Client) ChatCompletion(
    ctx context.Context,
    messages []goaitools.Message,
    tools aitooling.ToolSet,
) (*goaitools.ChatResponse, error) {
    // Extract OpenAI messages from interface
    openaiMessages := make([]Message, len(messages))
    for i, msg := range messages {
        // If it's our own message type, use raw directly
        if m, ok := msg.(*message); ok {
            openaiMessages[i] = m.raw
        } else {
            // Fallback: reconstruct from interface (shouldn't happen in normal flow)
            openaiMessages[i] = Message{
                Role:       string(msg.Role()),
                Content:    msg.Content(),
                ToolCalls:  convertToolCallsToOpenAI(msg.ToolCalls()),
                ToolCallID: msg.ToolCallID(),
            }
        }
    }

    // ... send request ...

    // Wrap response in our message type
    responseMessage := &message{raw: choice.Message}

    return &goaitools.ChatResponse{
        Message:      responseMessage,
        FinishReason: goaitools.FinishReason(choice.FinishReason),
    }, nil
}
```

### 6. Chat Options Updates (chat.go)

```go
// WithSystemMessage now uses backend factory
func WithSystemMessage(text string) ChatOption {
    return func(cfg *chatRequest) {
        // Need access to backend in chatRequest for factory
        // OR: defer message creation until ChatWithState has backend
        cfg.systemMessages = append(cfg.systemMessages, text)
    }
}

// In ChatWithState, create messages from backend
func (c *Chat) ChatWithState(
    ctx context.Context,
    state ConversationState,
    opts ...ChatOption,
) (string, ConversationState, error) {
    request := chatRequest{
        systemMessages: []string{},
        userMessages:   []string{},
        // ...
    }
    for _, opt := range opts {
        opt(&request)
    }

    // Convert strings to backend messages
    messages := make([]Message, 0)
    for _, text := range request.systemMessages {
        messages = append(messages, c.Backend.NewSystemMessage(text))
    }
    for _, text := range request.userMessages {
        messages = append(messages, c.Backend.NewUserMessage(text))
    }

    // ... rest of implementation
}
```

## Why This Works

### Perfect Round-Tripping

```
GPT-5 Response:
{
  role: "assistant",
  content: "Here's the answer...",
  reasoning_content: "I thought about X, Y, Z...",  ← NEW
  confidence: 0.95                                   ← NEW
}
    ↓
Wrapped in message{raw: Message{...}}
    ↓
Chat accesses via Content() - doesn't see new fields
    ↓
State saves via MarshalJSON() → ALL fields preserved
    ↓
State loads via UnmarshalMessage() → ALL fields restored
    ↓
Next turn: complete message sent back to OpenAI
    ↓
GPT-5 receives its reasoning context ✓
```

### No Split-Brain

- **Single source of truth**: Provider's native type (`message.raw`)
- **Interface methods**: Read-only views, don't duplicate data
- **No ambiguity**: When sending to provider, use the native type directly

### Type Safety

```go
// Compile-time enforcement
var _ goaitools.Message = (*message)(nil)

// Runtime type assertion for optimization
if m, ok := msg.(*message); ok {
    // Fast path: use native type directly
    openaiMessages[i] = m.raw
}
```

## Breaking Changes

### What Breaks

1. **`Message` type**: Struct → Interface
   - `Message{Role: "user", Content: "..."}` no longer compiles
   - Must use `backend.NewUserMessage("...")`

2. **Direct message construction**: No longer possible
   - Old: `msg := goaitools.Message{...}`
   - New: `msg := backend.NewUserMessage(...)`

3. **Message field access**: No longer direct
   - Old: `msg.Content`, `msg.Role`
   - New: `msg.Content()`, `msg.Role()`

### What Doesn't Break

1. **Backend interface**: `ChatCompletion` signature unchanged (still `[]Message`)
2. **State format**: Still JSON, still has version/provider fields
3. **Tool execution**: Still uses provider-agnostic `aitooling` types

### Migration Path

1. Update backend implementations first (OpenAI)
2. Update message construction to use factories
3. Update field access to use methods
4. Tests guide the migration (compilation errors)

## Implementation Checklist

- [ ] Update `backend.go`
  - [ ] Change `Message` from struct to interface
  - [ ] Add factory methods to `Backend` interface
  - [ ] Update `ChatResponse.Message` to interface type

- [ ] Update `chat.go`
  - [ ] Change `conversationStateInternal.Messages` to `[]json.RawMessage`
  - [ ] Update `encodeState` to use `MarshalJSON()`
  - [ ] Update `decodeState` to use `UnmarshalMessage()`
  - [ ] Update `buildMessages` to work with interface
  - [ ] Update options to defer message creation

- [ ] Create `openai/message.go`
  - [ ] Implement `message` type wrapping `Message`
  - [ ] Implement interface methods
  - [ ] Add compile-time check

- [ ] Update `openai/client.go`
  - [ ] Add factory methods
  - [ ] Add `UnmarshalMessage` method
  - [ ] Update `ChatCompletion` to extract native types
  - [ ] Update `ChatCompletion` to wrap response

- [ ] Update tests
  - [ ] Update message construction to use factories
  - [ ] Update assertions to use interface methods
  - [ ] Add round-trip tests for unknown fields

- [ ] Update examples/docs
  - [ ] Show factory usage
  - [ ] Document migration from struct to interface

## Testing Strategy

### Round-Trip Test

```go
func TestMessageRoundTrip(t *testing.T) {
    client := openai.NewClient("test-key")

    // Simulate GPT-5 response with unknown fields
    rawJSON := `{
        "role": "assistant",
        "content": "answer",
        "reasoning_content": "thought process",
        "confidence": 0.95
    }`

    // Unmarshal (simulates receiving from API)
    msg, err := client.UnmarshalMessage([]byte(rawJSON))
    require.NoError(t, err)

    // Access known fields via interface
    assert.Equal(t, goaitools.RoleAssistant, msg.Role())
    assert.Equal(t, "answer", msg.Content())

    // Serialize (simulates saving to state)
    data, err := msg.MarshalJSON()
    require.NoError(t, err)

    // Verify unknown fields preserved
    var parsed map[string]interface{}
    json.Unmarshal(data, &parsed)
    assert.Equal(t, "thought process", parsed["reasoning_content"])
    assert.Equal(t, 0.95, parsed["confidence"])
}
```

### State Compatibility Test

```go
func TestStatePreservesUnknownFields(t *testing.T) {
    backend := openai.NewClient("test-key")
    chat := &goaitools.Chat{Backend: backend}

    // Create state with message containing unknown fields
    msg, _ := backend.UnmarshalMessage([]byte(`{
        "role": "assistant",
        "content": "test",
        "future_field": "preserved"
    }`))

    state, err := chat.encodeState([]goaitools.Message{msg})
    require.NoError(t, err)

    // Decode and verify
    messages := chat.decodeState(context.Background(), state)
    require.Len(t, messages, 1)

    // Verify unknown field survived round-trip
    data, _ := messages[0].MarshalJSON()
    var parsed map[string]interface{}
    json.Unmarshal(data, &parsed)
    assert.Equal(t, "preserved", parsed["future_field"])
}
```

## Open Questions

1. **Should we version the message format?**
   - Pro: Allows detecting OpenAI API version changes
   - Con: JSON naturally handles unknown fields, may be unnecessary

2. **What if a backend doesn't support all interface methods?**
   - Example: Anthropic might not have `ToolCallID` concept
   - Solution: Return empty string, document expectations

3. **Should `UnmarshalMessage` validate provider compatibility?**
   - Could check for OpenAI-specific field signatures
   - Graceful degradation vs fail-fast tradeoff

4. **Performance impact of interface indirection?**
   - Minimal: only affects message access, not tool execution
   - Could benchmark if concerned

## Alternative Considered: ProviderMetadata Field

**Rejected because**: Creates split-brain problem

```go
type Message struct {
    Role             Role            `json:"role"`
    Content          string          `json:"content"`
    ProviderMetadata json.RawMessage `json:"provider_metadata"` // Duplicate data
}
```

**Problem**: When sending back to provider, which is source of truth?
- Use structured fields → lose new fields again
- Use ProviderMetadata → why have structured fields?

## Next Steps

1. **Validate design** with fresh eyes tomorrow
2. **Consider edge cases**: multimodal content, streaming responses
3. **Implementation** if design holds up
4. **Migration guide** for breaking changes

## References

- Current implementation: `chat.go:318-365` (state encode/decode)
- Message conversion: `openai/client.go:118-126`, `156-161`
- State format: `chat.go:21-27` (conversationStateInternal)
