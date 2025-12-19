package goaitools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ConversationState is an opaque blob representing conversation history.
// Clients should treat this as a black box - store it, retrieve it, but don't inspect it.
type ConversationState []byte

// conversationStateInternal is the internal representation of conversation state.
// This is not exposed to clients - they only see the opaque []byte.
type conversationStateInternal struct {
	Version         int               `json:"version"`          // State format version (current: 1)
	Provider        string            `json:"provider"`         // Backend provider name (e.g., "openai")
	ProcessedLength int               `json:"processed_length"` // The amount of messages that have been processed in a ChatResponse, excluding later appended messages
	Messages        []json.RawMessage `json:"messages"`         // Conversation history (opaque provider-specific messages)
}

// buildMessages constructs the full message list for the API call.
// Order: leading system messages from opts + state history + remaining non-system messages from opts
// This allows fresh system "preamble" on each call while preserving inline system messages in state.
func buildMessages(optMessages []Message, stateMessages []Message) []Message {
	// Separate leading system messages from other messages in opts
	var leadingSystemMessages []Message
	var otherOptMessages []Message

	// Find all leading system messages
	foundNonSystem := false
	for _, msg := range optMessages {
		if !foundNonSystem && msg.Role() == RoleSystem {
			leadingSystemMessages = append(leadingSystemMessages, msg)
		} else {
			foundNonSystem = true
			otherOptMessages = append(otherOptMessages, msg)
		}
	}

	// Build final order: leading system + state + other opts
	result := make([]Message, 0, len(leadingSystemMessages)+len(stateMessages)+len(otherOptMessages))
	result = append(result, leadingSystemMessages...)
	result = append(result, stateMessages...)
	result = append(result, otherOptMessages...)
	return result
}

// stripLeadingSystemMessages removes only the leading system messages from the message list.
// Everything from the first non-system message onward is preserved (including any inline system messages).
// Used when encoding state - allows caller to provide fresh "preamble" system messages on each call
// while preserving mid-conversation system messages (like event notifications).
//
// Example: {1S, 2S, 3U, 4S, 5U} → {3U, 4S, 5U}
func stripLeadingSystemMessages(messages []Message) []Message {
	// Find first non-system message
	firstNonSystem := -1
	for i, msg := range messages {
		if msg.Role() != RoleSystem {
			firstNonSystem = i
			break
		}
	}

	// If all messages are system messages (or empty), return empty
	if firstNonSystem == -1 {
		return nil
	}

	// Return slice from first non-system message onward
	return messages[firstNonSystem:]
}

// extractLeadingSystemMessages returns only the leading system messages from the message list.
// This is the inverse of stripLeadingSystemMessages.
// Used when providing context to compactors.
//
// Example: {1S, 2S, 3U, 4S, 5U} → {1S, 2S}
func extractLeadingSystemMessages(messages []Message) []Message {
	// Find first non-system message
	firstNonSystem := -1
	for i, msg := range messages {
		if msg.Role() != RoleSystem {
			firstNonSystem = i
			break
		}
	}

	// If no system messages at start (or all messages are system), return appropriate slice
	if firstNonSystem == -1 {
		// All messages are system messages
		return messages
	}
	if firstNonSystem == 0 {
		// No leading system messages
		return nil
	}

	// Return slice up to first non-system message
	return messages[:firstNonSystem]
}

// encodeState serializes conversation state to an opaque blob.
func (c *Chat) encodeState(messages []Message, processed_len int) (ConversationState, error) {
	if c.Backend == nil {
		return nil, fmt.Errorf("backend is nil")
	}

	// Serialize each message to json.RawMessage using provider's MarshalJSON
	rawMessages := make([]json.RawMessage, len(messages))
	for i, msg := range messages {
		data, err := msg.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("marshal message %d: %w", i, err)
		}
		rawMessages[i] = data
	}

	internal := conversationStateInternal{
		Version:         1,
		Provider:        c.Backend.ProviderName(),
		Messages:        rawMessages,
		ProcessedLength: processed_len,
	}

	data, err := json.Marshal(internal)
	if err != nil {
		return nil, fmt.Errorf("failed to encode conversation state: %w", err)
	}

	return ConversationState(data), nil
}

// decodeState deserializes conversation state from an opaque blob.
// Return the processed message length stored in the state
// Returns nil messages if state is nil, corrupted, or incompatible with current backend.
func (c *Chat) decodeState(ctx context.Context, state ConversationState) ([]Message, int) {
	if state == nil || len(state) == 0 {
		return nil, 0
	}

	var internal conversationStateInternal
	if err := json.Unmarshal(state, &internal); err != nil {
		c.logError(ctx, "invalid_conversation_state", err)
		return nil, 0 // Graceful degradation: start fresh conversation
	}

	// Validate version
	if internal.Version != 1 {
		c.logError(ctx, "unsupported_state_version", nil, "version", internal.Version)
		return nil, 0 // Graceful degradation: discard incompatible state
	}

	// Validate provider compatibility
	if c.Backend != nil && internal.Provider != c.Backend.ProviderName() {
		c.logError(ctx, "provider_mismatch", nil,
			"state_provider", internal.Provider,
			"current_provider", c.Backend.ProviderName())
		return nil, 0 // Graceful degradation: discard incompatible state
	}

	// Deserialize each message using backend's UnmarshalMessage
	messages := make([]Message, len(internal.Messages))
	for i, raw := range internal.Messages {
		msg, err := c.Backend.UnmarshalMessage(raw)
		if err != nil {
			c.logError(ctx, "message_unmarshal_failed", err, "index", i)
			return nil, 0 // Graceful degradation: discard corrupted state
		}
		messages[i] = msg
	}

	return messages, internal.ProcessedLength
}
