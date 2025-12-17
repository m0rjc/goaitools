# Changelog

## 0.2.0  - 2025-12-14

### Fixed

- Double encoding of tool arguments ([#2](https://github.com/m0rjc/goaitools/issues/2)) (Richard Corfield)

### Changed

- **Breaking:** The tool argument is passed as a string containing JSON.
- **Breaking:** `openai.NewClient()` and `openai.NewClientWithOptions()` now return `(*Client, error)` instead of `*Client`. An empty API key returns `openai.ErrMissingAPIKey` instead of nil.

## 0.1.0  - 2025-12-13

_First Release_