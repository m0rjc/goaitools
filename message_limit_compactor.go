package goaitools

import "context"

// MessageLimitCompactor keeps only the last N messages when the limit is exceeded.
// Messages are removed at user message boundaries to maintain conversation structure.
// MessageLimitCompactor can be used as a Compactor, or its two parts independently as CompactionTrigger and CompactionStrategy
type MessageLimitCompactor struct {
	// MaxMessages is the maximum number of messages to keep in state.
	// When exceeded, older messages are removed to maintain this limit.
	MaxMessages int
}

// Compact checks if message count exceeds the limit and truncates to keep last N messages.
// Ensures the resulting conversation starts at a user message boundary.
func (c *MessageLimitCompactor) Compact(ctx context.Context, req *CompactionRequest) (*CompactionResponse, error) {
	if compact, _ := c.ShouldCompact(ctx, req); compact {
		return c.CompactMessages(ctx, req)
	}
	return NewNotCompactedMessagesResponse(req), nil
}

func (c *MessageLimitCompactor) ShouldCompact(_ context.Context, request *CompactionRequest) (bool, error) {
	return c.MaxMessages > 0 && len(request.StateMessages) > c.MaxMessages, nil
}

func (c *MessageLimitCompactor) CompactMessages(_ context.Context, req *CompactionRequest) (*CompactionResponse, error) {
	// Remove the oldest messages to reach limit
	compacted := req.StateMessages[len(req.StateMessages)-c.MaxMessages:]

	// Advance to first user message boundary
	compacted = AdvanceToFirstUserMessage(compacted)

	return NewCompactedMessagesResponse(compacted), nil
}
