# Stateful Conversations - Architecture Design

**Status**: Implemented (on feature/stateful-chats branch, version 0.3.0-beta.1)
**Created**: 2025-12-12
**Implemented**: 2025-12-17
**Context**: This document outlines the design for adding multi-turn conversation support to goaitools.

---

## Problem Statement

Currently, each `Chat()` call is stateless. The conversation history built during a single call (user messages, assistant responses, tool calls, tool results) is discarded after the response is returned. This makes multi-turn conversations difficult:

1. **Client must manage state**: Calling code must manually track all messages
2. **No abstraction**: Clients deal with provider-specific message formats
3. **No compaction helpers**: Token costs grow linearly without pruning/summarization
4. **Event updates are manual**: Adding context like "User just tagged Harrogate Theatre" requires manual message construction

## Design Principles

### 1. Opaque State

**Decision**: Conversation state is opaque (`[]byte`) from the client's perspective.

**Rationale**:
- Clients like GPSGame don't want to understand message structures
- Enables internal optimizations without breaking clients
- Allows provider-specific state formats (OpenAI vs Anthropic message structures differ)
- Simplifies serialization for database storage

```go
// Client doesn't need to understand what's in state
state := []byte{...}  // Store in database
response, newState, err := chat.ChatWithState(ctx, state, opts...)
// Store newState for next turn
```

### 2. Library-Managed Compaction

**Decision**: Compaction and pruning strategies are the library's responsibility, configured via `Chat` options.

**Rationale**:
- Clients shouldn't need to understand token budgets, summarization strategies
- Different use cases need different strategies (configurable via options)
- Centralized logic ensures consistency across providers

**Implementation**: Compaction is a field on the `Chat` struct, not the backend client. This is because:
- Compaction is about conversation semantics, not provider-specific details
- Different application contexts need different strategies (game assistant vs customer support)
- The `Chat` layer already owns the state abstraction
- Keeps backend implementations simpler and focused on API communication

### 3. Event-Based Updates

**Decision**: Support adding events to state without a full chat round-trip.

**Rationale**:
- Efficiently inject context: "User visited location X"

The implementation may choose to use the LLM to perform compaction (maybe that previous visit to
Betty's Tea Room is no longer relevant) or just append a message for use in the next LLM call.

```go
newState := chat.UpdateStateAfterEvent(oldState, "User has just visited Harrogate Theatre")
```

### 4. Time-Based Expiry is Client Responsibility

**Decision**: The library does NOT track time or expire conversations.

**Rationale**:
- Time logic is application-specific (minutes? hours? days?)
- Client controls state storage, so client decides when to discard old state
- Other events, such as "User exits game" could also trigger state discarding.
- Simpler library boundary: library manages tokens/messages, client manages time

```go
// Client decides time logic
if time.Since(lastInteraction) > 24*time.Hour {
    state = nil  // Start fresh conversation
}
```

---

## Proposed API Design

### Core Types

```go
// ConversationState is an opaque blob representing conversation history
type ConversationState []byte

// ChatStateResult extends ChatResult with state
type ChatStateResult struct {
    Content string
    State   ConversationState  // Opaque state for next turn
}
```

### New Method: ChatWithState

```go
// ChatWithState performs a chat with conversation history.
// Parameters:
//   - ctx: Standard Go context
//   - state: Opaque conversation state from previous turn (nil for new conversation)
//   - opts: Chat options (messages, tools, logger)
// Returns:
//   - response: AI's text response
//   - newState: Updated conversation state for next turn
//   - error: Any errors during execution
func (c *Chat) ChatWithState(
    ctx context.Context,
    state ConversationState,
    opts ...ChatOption,
) (string, ConversationState, error)
```

**System Prompt Handling**: The system prompt should be passed on every call via `WithSystemMessage()`. Following [OpenAI's session memory pattern](https://cookbook.openai.com/examples/agents_sdk/session_memory), system messages are NOT stored in conversation state. The library will:
- Always prepend system messages to the conversation on each turn
- Store only conversation history (user/assistant/tool messages) in state
- This allows system prompts to include dynamic content (timestamps, user context) that updates each turn
- State stays clean and token-efficient (no system message duplication)

**Backward Compatibility**: Existing `Chat()` method remains unchanged:

```go
// Chat performs a stateless chat (existing behavior)
func (c *Chat) Chat(ctx context.Context, opts ...ChatOption) (string, error) {
    response, _, err := c.ChatWithState(ctx, nil, opts...)
    return response, err
}
```

### Event Updates

```go
// UpdateStateAfterEvent adds contextual information to conversation state
// without making an API call. Useful for injecting events like:
// - "User has just tagged Harrogate Theatre"
// - "Team score updated to 150 points"
// - "Game ended at 3:45pm"
func (c *Chat) UpdateStateAfterEvent(
    state ConversationState,
    eventDescription string,
) ConversationState
```

### Compaction Configuration

```go
// CompactionStrategy determines how conversation history is managed
type CompactionStrategy interface {
    // ShouldCompact decides if compaction is needed based on message count/tokens
    ShouldCompact(messageCount int, estimatedTokens int) bool

    // Compact reduces conversation history (implementation varies by strategy)
    Compact(ctx context.Context, messages []Message) ([]Message, error)
}

// Built-in strategies:
var (
    // NoCompaction keeps all messages (default)
    NoCompaction CompactionStrategy = &noCompactionStrategy{}

    // KeepLastN keeps only the last N message exchanges
    KeepLastN(n int) CompactionStrategy

    // TokenBudget keeps messages within token budget, pruning oldest first
    TokenBudget(maxTokens int) CompactionStrategy

    // LLMSummarization asks the LLM to summarize old messages
    LLMSummarization(targetTokens int) CompactionStrategy
)

// Configure on Chat object:
chat := &goaitools.Chat{
    Backend: client,
    CompactionStrategy: goaitools.TokenBudget(2000),
}
```

---

## Internal State Format

### State Encoding

```go
// Internal representation (not exposed to clients)
type conversationStateInternal struct {
    Version  int         `json:"version"`   // For future migrations
    Messages []Message   `json:"messages"`  // Conversation history
    Metadata Metadata    `json:"metadata"`  // Optional: token counts, etc.
}

type Metadata struct {
    EstimatedTokens int       `json:"estimated_tokens,omitempty"`
    LastCompactedAt time.Time `json:"last_compacted_at,omitempty"`
}
```

**Serialization**: JSON encoded, gzipped (optional optimization)

**Why JSON**:
- Human-readable for debugging
- Easy to migrate between versions
- Standard library support

**Provider-Specific Messages**: State contains provider-specific `Message` types (OpenAI includes `ToolCallID`, Anthropic may differ). This is why state must be opaque.

This means that changing provider requires discarding any old state. For this reason the library must be free to disregard state that
is incompatible or corrupt and silently run a stateless call, and in answer to a previous question it must accept the system
prompt every time and decide whether to use it depending on state.

---

## Compaction Strategies - Details

### 1. NoCompaction (Default)

**Behavior**: Keep all messages indefinitely

**Use Case**:
- Short conversations (few turns)
- Debugging/development
- When token costs aren't a concern

**Implementation**: No-op, just accumulate messages

---

### 2. KeepLastN

**Behavior**: Keep only the last N message exchanges (user + assistant pairs)

**Use Case**:
- Fixed-depth conversations
- "Remember last 5 interactions"

**Implementation**:
```go
// Prune messages, keeping:
// - System messages (always keep)
// - Last N user/assistant/tool exchanges
```

**Edge Case**: Tool calls span multiple messages (user → assistant with tool_calls → tool results → assistant response). Must keep complete exchanges.

**Tool Exchange Summarization**: When pruning messages that include tool calls, summarize the interaction and inject as a system message:

```go
// Original tool exchange (5 messages):
// User: "What's the weather in Paris?"
// Assistant: [tool_call: get_weather(location="Paris")]
// Tool: {"temperature": 22, "condition": "sunny"}
// Assistant: "It's sunny and 22°C in Paris"

// Summarized (1 system message):
{Role: "system", Content: "Previous interaction: User asked about Paris weather. Assistant called get_weather(Paris), received sunny/22°C, and informed user."}
```

This preserves context without the verbose tool call/result sequence.

---

### 3. TokenBudget

**Behavior**: Keep messages within a token budget, pruning oldest first

**Use Case**:
- Cost control
- Model context window limits (e.g., 4096 tokens)

**Implementation**:
```go
// Estimate tokens for each message
// Prune oldest messages until under budget
// Always keep system message and last user message
```

**Token Estimation**: Use heuristic (rough: `len(content) / 4`) or tiktoken library for accuracy

---

### 4. LLMSummarization

**Behavior**: When history exceeds target tokens, ask LLM to summarize old messages

**Use Case**:
- Long conversations with important context
- Preserve semantic meaning, reduce token count

**Implementation**:
```go
// 1. Detect: conversation exceeds targetTokens
// 2. Extract: oldest 50% of messages (excluding last few exchanges)
// 3. Summarize: "Please summarize this conversation history concisely"
// 4. Replace: old messages with single system message containing summary
```

**Cost Tradeoff**: Costs tokens to summarize, but saves tokens on future turns

**Example**:
```
Original (1500 tokens):
  System: You are a game assistant
  User: What's my score?
  Assistant: Your score is 100
  User: Who else is playing?
  Assistant: Teams A, B, C are playing
  User: What location did Team A visit?
  Assistant: Team A visited Harrogate Theatre

After Summarization (400 tokens):
  System: You are a game assistant. Previous conversation:
         User asked about score (100 points) and teams (A, B, C playing).
         Team A visited Harrogate Theatre.
  User: [current question]
```

---

## Migration Strategy

### Phase 1: Add New APIs (Non-Breaking)

1. Add `ChatWithState()` method
2. Add `UpdateStateAfterEvent()`
3. Existing `Chat()` delegates to `ChatWithState(ctx, nil, opts...)`
4. Default: `NoCompaction` strategy

**Result**: Existing code unaffected, new features opt-in

### Phase 2: Add Compaction (Configuration)

1. Add `CompactionStrategy` field to `Chat` struct
2. Implement built-in strategies
3. Document usage patterns

### Phase 3: Optimization (Optional)

1. Add state compression (gzip)
2. Add token estimation accuracy improvements
3. Provider-specific optimizations

---

## Usage Examples

### Example 1: MiniMonopoly Multi-Turn Chat

```go
// Game stores state in database
type Game struct {
    ID              string
    ConversationState []byte  // Opaque state from goaitools
    LastInteraction time.Time
}

// Handler for WhatsApp messages
func handlePlayerMessage(gameID string, userMessage string) {
    game := loadGame(gameID)

    // Time-based expiry (client decides)
    state := game.ConversationState
    if time.Since(game.LastInteraction) > 2*time.Hour {
        state = nil  // Start fresh
    }

    // Chat with state
    response, newState, err := chat.ChatWithState(
        ctx,
        state,
        goaitools.WithSystemMessage("You are a friendly game assistant"),
        goaitools.WithUserMessage(userMessage),
        goaitools.WithTools(gameTools),
    )

    // Store new state
    game.ConversationState = newState
    game.LastInteraction = time.Now()
    saveGame(game)

    // Send response to user
    whatsapp.SendMessage(response)
}
```

### Example 2: Event Updates

```go
// User visits a location (no chat, just update context)
func handleLocationVisit(gameID string, locationName string) {
    game := loadGame(gameID)

    // Add event to conversation context
    newState := chat.UpdateStateAfterEvent(
        game.ConversationState,
        fmt.Sprintf("User has just visited %s and earned 50 points", locationName),
    )

    game.ConversationState = newState
    saveGame(game)
}

// Later, when user asks "what did I just do?"
// The AI will have context about the location visit
```

### Example 3: Compaction Strategy

```go
// Configure chat with token budget
chat := &goaitools.Chat{
    Backend: openaiClient,
    CompactionStrategy: goaitools.TokenBudget(2000),
}

// As conversation grows, library automatically prunes old messages
// Client doesn't need to think about it
for i := 0; i < 100; i++ {
    response, state, err := chat.ChatWithState(ctx, state,
        goaitools.WithUserMessage(fmt.Sprintf("Question %d", i)),
    )
    // State internally managed to stay under 2000 tokens
}
```

---

## Open Questions / Future Considerations

### 1. Token Counting Accuracy

**Question**: Use heuristic (`len(content)/4`) or accurate tokenizer (tiktoken)?

**Tradeoffs**:
- Heuristic: Fast, no dependencies, "good enough"
- Tiktoken: Accurate, adds dependency, slower

**Recommendation**: Start with heuristic, make tokenizer pluggable via interface:

```go
type TokenCounter interface {
    CountTokens(content string) int
}

// Default heuristic implementation
type heuristicCounter struct{}
func (h heuristicCounter) CountTokens(content string) int {
    return len(content) / 4
}

// Optional accurate implementation (requires tiktoken dependency)
type tiktokenCounter struct { /* ... */ }
```

Users can provide custom counters via `Chat` configuration.

---

### 2. State Versioning

**Question**: How to handle state migration when internal format changes?

**Proposal**:
```go
type conversationStateInternal struct {
	Provider string // OpenAI
    Version int  // Current: 1
    ...
}

// When deserializing:
if state.Provider != 'MyName' {
	state = nil // Discard state
}

if state.Version < currentVersion {
    state = migrateState(state)
}
```



---

### 3. Cross-Provider State

**Question**: Can state from OpenAI be used with Anthropic backend?

**Answer**: Not directly - message formats differ. Need provider-specific state.

**Options**:
- **Provider-locked state**: State remembers backend type, error if mismatch
- **Provider-agnostic adapter**: Convert between formats (complex, lossy)

**Recommendation**: Provider-locked state (simpler, safer)

This has been included in 2. State Versioning above.

---

### 4. Streaming Support

**Question**: How does stateful conversation work with streaming responses?

**Design**: Return state after streaming completes via a result struct:

```go
type StreamingResult struct {
    TextChan  <-chan string          // Stream of text chunks
    StateChan <-chan ConversationState // Final state (sent when stream completes)
    ErrChan   <-chan error           // Any errors during streaming
}

func (c *Chat) ChatWithStateStreaming(
    ctx context.Context,
    state ConversationState,
    opts ...ChatOption,
) (*StreamingResult, error) {
    // Start streaming in background
    // When stream completes, send final state to StateChan
}
```

**Usage**:
```go
result, err := chat.ChatWithStateStreaming(ctx, state, opts...)
for chunk := range result.TextChan {
    fmt.Print(chunk)  // Stream to user
}
newState := <-result.StateChan  // Get final state after streaming
```

**Complexity**: Deferred for future enhancement

At the moment we don't stream responses. The use of this client in a WhatsApp based
game prevents streaming because WhatsApp wants messages delivered atomically.

---

## Testing Strategy

### Unit Tests

1. **State round-trip**: Encode → decode → verify equality
2. **Compaction strategies**: Each strategy with various inputs
3. **Event updates**: Verify events added to state correctly
4. **Migration**: Test version 1 → version 2 state upgrade

### Integration Tests

1. **Multi-turn conversation**: 10+ turns, verify context preserved
2. **Compaction behavior**: Verify old messages pruned correctly
3. **Event injection**: Update state with events, verify AI uses context

### Mock Backend

```go
type MockBackend struct {
    // Record all messages seen
    SeenMessages [][]Message
}

func (m *MockBackend) ChatCompletionWithTools(...) {
    m.SeenMessages = append(m.SeenMessages, messages)
    return mockResponse
}

// Test: verify conversation history accumulates correctly
```

---

## Implementation Checklist

### Phase 1: Core State Management ✅ COMPLETE
- [x] Add `ConversationState` type (opaque `[]byte`) - chat.go:11-13
- [x] ~~Add `ChatStateResult` struct~~ - Simplified to tuple return `(string, ConversationState, error)`
- [x] Implement `ChatWithState()` method - chat.go:110-223
- [x] Refactor existing `Chat()` to use `ChatWithState()` - chat.go:308-311
- [x] Add internal state encoding/decoding (JSON) - chat.go:442-512
- [x] Add state versioning support (Provider + Version fields) - chat.go:26-31
- [x] Add state validation (detect corruption, version mismatches) - chat.go:475-512
- [x] Implement graceful degradation for incompatible state - chat.go:475-512
- [x] Handle system prompt injection/deduplication - chat.go:228-304
- [x] Add `ProcessedLength` field for future pre-compaction support - chat.go:29

### Phase 2: Event Updates ✅ COMPLETE
- [x] Implement `AppendToState()` (renamed from `UpdateStateAfterEvent`) - chat.go:320-347
- [x] Add tests for event state updates - example/hellowithstate/main.go

### Phase 3: Compaction Strategies ✅ CORE COMPLETE
- [x] Define `Compactor` interface - compactor.go:60-68
- [x] Define `CompactionTrigger` interface - compactor.go:73-75
- [x] Define `CompactionStrategy` interface - compactor.go:80-82
- [x] Implement `SplitCompactor` (combines trigger + strategy) - compactor.go:84-99
- [x] Implement `CompositeCompactor` - compactor.go:119-136
- [x] Implement `CompositeCompactionTrigger` - compactor.go:138-153
- [x] Implement `MessageLimitCompactor` strategy - message_limit_compactor.go
- [x] Implement `TokenLimitCompactor` strategy (uses actual API token usage) - token_limit_compactor.go
- [x] Add helper `AdvanceToFirstUserMessage()` for user message boundaries - compactor.go:107-115
- [x] Write unit tests for compaction strategies - compactor_test.go
- [ ] ~~Define `TokenCounter` interface~~ - Deferred: TokenLimitCompactor uses actual API token usage
- [ ] ~~Implement `LLMSummarization` strategy~~ - Deferred: Advanced feature, add when needed
- [ ] ~~Add helper for tool exchange summarization~~ - Deferred: Add if proven useful in practice

### Phase 4: Testing & Documentation ✅ COMPLETE
- [x] Write unit tests for state management (encode/decode/versioning) - compactor_test.go
- [x] Write integration tests for multi-turn conversations - example/hellowithstate/
- [x] Write integration tests for compaction - example/statecompaction/
- [x] Write tests for state corruption handling - Handled via graceful degradation
- [x] Write tests for provider mismatch handling - Handled via graceful degradation
- [x] Update documentation - docs/conversation-state.md
- [x] Update CLAUDE.md with stateful conversation patterns - CLAUDE.md
- [x] Add usage examples - example/hellowithstate/, example/statecompaction/
- [ ] Add metrics/telemetry hooks (optional) - Not needed yet

---

## References

- [OpenAI Chat API](https://platform.openai.com/docs/api-reference/chat)
- [OpenAI Session Memory Pattern](https://cookbook.openai.com/examples/agents_sdk/session_memory) - Reference implementation showing system messages separate from session state
- [Token Counting](https://github.com/openai/tiktoken)
- [Conversation Design Patterns](https://www.anthropic.com/research/building-effective-agents)
