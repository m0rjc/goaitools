# State Compaction Example

This example demonstrates how to use conversation state compaction to manage conversation length as it grows.

## What This Demonstrates

1. **MessageLimitCompactor**: Automatically keeps only the last N messages
2. **TokenLimitCompactor**: Manages conversation based on token usage from the API
3. **CompositeCompactor**: Combines multiple compaction strategies (first to trigger wins)
4. **State Size Management**: Shows how state size stabilizes with compaction enabled

## Running the Example

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="your-key-here"

# Or create a .env file with:
# OPENAI_API_KEY=your-key-here

# Run the example
go run .
```

## Key Concepts

### MessageLimitCompactor
- Triggers when conversation exceeds a fixed message count
- Keeps only the most recent N messages
- Useful for simple conversation length management
- Fast and predictable

### TokenLimitCompactor
- Triggers when token usage exceeds MaxTokens
- Compacts down to TargetTokens (default 75% of MaxTokens)
- Requires token usage information from API responses
- More accurate for managing API costs and context limits

### CompositeCompactor
- Tries multiple compaction strategies in order
- First compactor to trigger performs the compaction
- Useful for "whichever limit is reached first" scenarios

### Compaction Boundaries
All compactors respect user message boundaries - conversations are always trimmed to start with a user message, maintaining proper conversation structure.

## Integration with ChatWithState

Compaction happens automatically after successful conversation turns:
- Configure once on the `Chat` struct
- Runs after each turn that completes with `FinishReason: "stop"`
- Transparent to application code - state management is automatic

## Cost Considerations

- MessageLimitCompactor: Zero cost (simple truncation)
- TokenLimitCompactor: Uses API-provided token counts (no extra API calls)
- Custom compactors could use AI summarization (requires API calls)
