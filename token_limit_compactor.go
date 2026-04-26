package goaitools

import "context"

// TokenLimitCompactor removes older messages when token count exceeds the limit.
// This strategy uses actual token usage from the API to make informed decisions.
// Messages are removed at user message boundaries to maintain conversation structure.
type TokenLimitCompactor struct {
	// MaxTokens is the maximum number of tokens to allow in conversation state.
	// This is checked against the PromptTokens from the API response.
	MaxTokens int

	// TargetTokens is the target token count after compaction (optional).
	// If 0, defaults to 75% of MaxTokens to avoid repeated compaction.
	// This provides headroom for the next few messages.
	TargetTokens int
}

func (c *TokenLimitCompactor) Compact(ctx context.Context, req *CompactionRequest) (*CompactionResponse, error) {
	// Error cannot be nil in this class
	if compact, _ := c.ShouldCompact(ctx, req); compact {
		return c.CompactMessages(ctx, req)
	}
	return NewNotCompactedMessagesResponse(req), nil
}

func (c *TokenLimitCompactor) ShouldCompact(_ context.Context, req *CompactionRequest) (bool, error) {
	// Can't compact without token usage information
	// TODO: Either we have to guess, or we ask the backend to guess
	if req.LastAPIUsage == nil {
		return false, nil
	}

	// Check if compaction is needed
	if c.MaxTokens <= 0 || req.LastAPIUsage.PromptTokens <= c.MaxTokens {
		return false, nil
	}

	return true, nil
}

func (c *TokenLimitCompactor) CompactMessages(_ context.Context, req *CompactionRequest) (*CompactionResponse, error) {
	// Determine target token count
	target := c.TargetTokens
	if target <= 0 {
		// Default to 75% of max to provide headroom
		target = (c.MaxTokens * 3) / 4
	}

	// Simple strategy: remove messages from the beginning
	// More sophisticated strategies could:
	// - Preserve semantically important messages
	// - Keep tool call/result pairs together
	// - Summarize removed content

	// Estimate: remove approximately enough messages to reach target
	// This is approximate because we don't know per-message token counts
	tokensToRemove := req.LastAPIUsage.PromptTokens - target
	if tokensToRemove <= 0 {
		return NewNotCompactedMessagesResponse(req), nil
	}

	// Simple approach: remove first 1/3 of messages
	// More sophisticated: estimate per-message tokens and iterate
	if len(req.StateMessages) <= 2 {
		// Keep at least 2 messages for context
		return NewNotCompactedMessagesResponse(req), nil
	}

	removeCount := len(req.StateMessages) / 3
	if removeCount < 1 {
		removeCount = 1
	}

	compacted := req.StateMessages[removeCount:]

	// Advance to first user message boundary
	compacted = AdvanceToFirstUserMessage(compacted)

	// If we removed too few messages, remove more aggressively
	// (Only matters if we had token tracking per message, which we don't yet)

	return NewCompactedMessagesResponse(compacted), nil
}
