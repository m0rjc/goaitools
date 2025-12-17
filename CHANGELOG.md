# Changelog

## 0.3.0-beta.1 - 2025-12-17 (feature/stateful-chats branch)

### Added

- **Stateful Multi-Turn Conversations**: New `ChatWithState()` API enables conversation history persistence across multiple turns
  - Opaque `ConversationState` type (`[]byte`) for easy storage in databases
  - State versioning and provider-locking for safety
  - Graceful degradation when state is corrupted or incompatible
  - System message handling: leading system messages not persisted (can be dynamic), mid-conversation system messages preserved

- **State Management**:
  - `AppendToState()` method to add contextual messages without making API calls
  - Automatic state encoding/decoding with JSON
  - Provider compatibility validation

- **Conversation History Compaction**: Flexible system to manage conversation length and token costs
  - `Compactor` interface for custom compaction strategies
  - `MessageLimitCompactor`: Keep only the last N messages
  - `TokenLimitCompactor`: Remove messages when token usage exceeds limits (uses actual API token usage)
  - `CompositeCompactor`: Combine multiple compaction strategies
  - `SplitCompactor`: Separate "when to compact" from "how to compact" logic
  - `CompactionTrigger` and `CompactionStrategy` interfaces for fine-grained control
  - Automatic compaction at user message boundaries following OpenAI best practices

- **Examples & Documentation**:
  - `example/hellowithstate/`: Complete multi-turn conversation demonstration
  - `example/statecompaction/`: Compaction strategies demonstration
  - `docs/conversation-state.md`: Comprehensive implementation guide
  - Updated `CLAUDE.md` with stateful conversation patterns

### Notes

- This is a feature branch release for testing in production use (wide-game-bot)
- Core functionality complete; advanced features (LLM summarization) deferred until proven needed
- Backward compatible: existing `Chat()` method unchanged

## 0.2.0  - 2025-12-14

### Fixed

- Double encoding of tool arguments ([#2](https://github.com/m0rjc/goaitools/issues/2)) (Richard Corfield)

### Changed

- **Breaking:** The tool argument is passed as a string containing JSON.
- **Breaking:** `openai.NewClient()` and `openai.NewClientWithOptions()` now return `(*Client, error)` instead of `*Client`. An empty API key returns `openai.ErrMissingAPIKey` instead of nil.

## 0.1.0  - 2025-12-13

_First Release_