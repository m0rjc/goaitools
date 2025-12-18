# Changelog

## 0.3.0 - 2025-12-18

### Added

- Explore calling different models. [#9](https://github.com/m0rjc/goaitools/issues/9)
  - Support running the examples with different models.
- OpenAI Client allows arbitrary model parameters to be set.
- [Conversation State Documentation](docs/conversation-state.md)
- [Timeout Documentation](docs/timeout_configuration.md)

## 0.3.0-beta.1 - 2025-12-17 (feature/stateful-chats branch)

### Added

- **Stateful Multi-Turn Conversations**:  [#4](https://github.com/m0rjc/goaitools/issues/4)
  - New `ChatWithState()` API enables conversation history persistence across multiple turns
  - Opaque `ConversationState` type (`[]byte`) for easy storage in databases
  - `AppendToState()` method to add contextual messages without making API calls

- **Conversation History Compaction**:

### Notes

- This is a feature branch release for testing in production use (wide-game-bot)
- Core functionality complete; advanced features (LLM summarization) deferred until proven needed.
  See [#11](https://github.com/m0rjc/goaitools/issues/11)
- Backward compatible: existing `Chat()` method unchanged

## 0.2.0  - 2025-12-14

### Fixed

- Double encoding of tool arguments ([#2](https://github.com/m0rjc/goaitools/issues/2)) (Richard Corfield)

### Changed

- **Breaking:** The tool argument is passed as a string containing JSON.
- **Breaking:** `openai.NewClient()` and `openai.NewClientWithOptions()` now return `(*Client, error)` instead of `*Client`. An empty API key returns `openai.ErrMissingAPIKey` instead of nil.

## 0.1.0  - 2025-12-13

_First Release_