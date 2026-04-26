package goaitools

import "context"

// CompactionRequest provides context for compaction decisions.
type CompactionRequest struct {
	// StateMessages contains the conversation history from state (what gets compacted).
	// This excludes leading system messages.
	StateMessages []Message

	// ProcessedLength is the amount of StateMessages previously seen by the LLM.
	// It excludes any messages added by Chat.AppendToState.
	// This will always be len(StateMessages) when the compactor is called at the end of an
	// LLM run.
	ProcessedLength int

	// LeadingSystemMessages contains the system preamble (not stored in state).
	// Provided for context - useful if compactor needs to summarize with full context.
	// May be empty (e.g., when compacting from UpdateStateAfterEvent or no system message in call).
	LeadingSystemMessages []Message

	// LastAPIUsage contains token usage from the most recent API call.
	// May be nil if backend doesn't provide token usage or if compacting outside of API call cycle.
	// PromptTokens represents the total tokens for all messages in the conversation
	// (including system messages).
	LastAPIUsage *TokenUsage

	// Backend is the backend being used (allows provider-specific compaction strategies)
	Backend Backend
}

// CompactionResponse contains the result of message compaction
type CompactionResponse struct {
	// StateMessages contains the new message history for state. This excludes the leading
	// system messages. This may be the input StateMessages slice, a subset of it, a wholly new
	// (summarised) set of messages, an empty slice or nil.
	StateMessages []Message

	// WasCompacted is true if the StateMessages list has changed.
	WasCompacted bool
}

// NewNotCompactedMessagesResponse returns a response indicating that messages were not compacted.
func NewNotCompactedMessagesResponse(originalRequest *CompactionRequest) *CompactionResponse {
	return &CompactionResponse{
		StateMessages: originalRequest.StateMessages,
		WasCompacted:  false,
	}
}

// NewCompactedMessagesResponse returns a response indicating that messages were compacted.
func NewCompactedMessagesResponse(stateMessages []Message) *CompactionResponse {
	return &CompactionResponse{
		StateMessages: stateMessages,
		WasCompacted:  true,
	}
}

// Compactor decides if conversation state should be compacted and performs the compaction.
// Implementations can use any strategy: message count limits, token limits, semantic importance, etc.
// Compactor is the interface expected by Chat when configuring compaction, so is the entry point to
// the system.
//
// Important: Compactors receive StateMessages (conversation history without leading system messages)
// and should return compacted StateMessages. Leading system messages are never compacted.
type Compactor interface {
	// Compact checks if compaction is needed and performs it if necessary.
	//
	// Returns:
	//   - []Message: Potentially compacted state message list (this may be the same slice if no compaction was needed)
	//   - bool: True if compaction was performed
	//   - error: Any error during compaction
	Compact(ctx context.Context, req *CompactionRequest) (*CompactionResponse, error)
}

// CompactionTrigger answers the question of when to compact.
// This is used to build compactors in which the decision of when and how to compact are independently
// customised.
type CompactionTrigger interface {
	ShouldCompact(ctx context.Context, request *CompactionRequest) (bool, error)
}

// CompactionStrategy answers the question of how to compact.
// This is used to build compactors in which the decision of when and how to compact are independently
// customised.
type CompactionStrategy interface {
	CompactMessages(ctx context.Context, request *CompactionRequest) (*CompactionResponse, error)
}

// SplitCompactor allows separate trigger and compaction strategies to be combined.
type SplitCompactor struct {
	Trigger  CompactionTrigger
	Strategy CompactionStrategy
}

func (c *SplitCompactor) CompactMessages(ctx context.Context, request *CompactionRequest) (*CompactionResponse, error) {
	split, err := c.Trigger.ShouldCompact(ctx, request)
	if err != nil {
		return nil, err
	}
	if split {
		return c.Strategy.CompactMessages(ctx, request)
	}
	return NewNotCompactedMessagesResponse(request), nil
}

// AdvanceToFirstUserMessage removes messages from the beginning until a user message is found.
// This ensures conversation state starts at a natural boundary (user input).
// Returns the trimmed slice, or nil if no user message is found.
//
// Following OpenAI's recommendation: always compact at user message boundaries
// to maintain proper conversation structure.
func AdvanceToFirstUserMessage(messages []Message) []Message {
	for i, msg := range messages {
		if msg.Role() == RoleUser {
			return messages[i:]
		}
	}
	// No user message found - return nil (empty conversation state)
	return nil
}

// CompositeCompactor tries its nested compactors in turn until the first compactor triggers
// or an error is returned.
type CompositeCompactor struct {
	Compactors []Compactor
}

// Compact tries each compactor in order, returning the result of the first to compact messages
// or Not Compacted if no compactor compacted the messages.
func (c *CompositeCompactor) Compact(ctx context.Context, req *CompactionRequest) (*CompactionResponse, error) {
	for _, compactor := range c.Compactors {
		response, err := compactor.Compact(ctx, req)
		if err != nil {
			return nil, err
		}
		if response.WasCompacted {
			return response, nil
		}
	}
	return NewNotCompactedMessagesResponse(req), nil
}

// CompositeCompactionTrigger allows multiple trigger conditions to be tried
type CompositeCompactionTrigger struct {
	Triggers []CompactionTrigger
}

// ShouldCompact returns true if any trigger requests compaction. Triggers are tried in order until
// one succeeds.
func (c *CompositeCompactionTrigger) ShouldCompact(ctx context.Context, request *CompactionRequest) (bool, error) {
	for _, trigger := range c.Triggers {
		response, err := trigger.ShouldCompact(ctx, request)
		if response || err != nil {
			return response, err
		}
	}
	return false, nil
}
