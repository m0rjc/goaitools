package openai

import (
	"encoding/json"
	"fmt"

	"github.com/m0rjc/goaitools"
)

// message wraps the OpenAI-specific Message type.
// This preserves ALL OpenAI fields (including future unknown fields) for round-tripping.
type message struct {
	rawJSON json.RawMessage // Complete original JSON bytes
	parsed  Message         // Parsed known fields for interface access
}

// Compile-time interface check
var _ goaitools.Message = (*message)(nil)

// Interface implementation - read-only views of what Chat needs

func (m *message) Role() goaitools.Role {
	return goaitools.Role(m.parsed.Role)
}

func (m *message) Content() string {
	return m.parsed.Content
}

func (m *message) ToolCalls() []goaitools.ToolCall {
	if len(m.parsed.ToolCalls) == 0 {
		return nil
	}

	result := make([]goaitools.ToolCall, len(m.parsed.ToolCalls))
	for i, tc := range m.parsed.ToolCalls {
		result[i] = goaitools.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		}
	}
	return result
}

func (m *message) ToolCallID() string {
	return m.parsed.ToolCallID
}

// MarshalJSON returns the original JSON bytes, preserving ALL fields
// (including unknown future fields like reasoning_content, confidence, etc.)
func (m *message) MarshalJSON() ([]byte, error) {
	return m.rawJSON, nil
}

// newMessage creates a message from a parsed struct (for factory methods).
// This marshals the struct to get the rawJSON representation.
func newMessage(parsed Message) (goaitools.Message, error) {
	rawJSON, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}
	return &message{
		rawJSON: rawJSON,
		parsed:  parsed,
	}, nil
}

// unmarshalMessage creates a message from raw JSON bytes (for state deserialization).
// This preserves the exact JSON for round-tripping.
func unmarshalMessage(data []byte) (goaitools.Message, error) {
	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal OpenAI message: %w", err)
	}
	return &message{
		rawJSON: data,
		parsed:  parsed,
	}, nil
}
