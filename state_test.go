package goaitools

import (
	"context"
	"encoding/json"
	"testing"
)

// ============================================================================
// Unit tests for pure message manipulation functions
// ============================================================================

// Test: buildMessages with only user messages (no system messages)
func TestBuildMessages_OnlyUserMessages(t *testing.T) {
	optMessages := []Message{
		&mockMessage{role: RoleUser, content: "Question 1"},
		&mockMessage{role: RoleUser, content: "Question 2"},
	}
	stateMessages := []Message{
		&mockMessage{role: RoleUser, content: "Previous message"},
	}

	result := buildMessages(optMessages, stateMessages)

	// Should be: state + opt messages
	expected := []string{"Previous message", "Question 1", "Question 2"}
	if len(result) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(result))
	}
	for i, exp := range expected {
		if result[i].Content() != exp {
			t.Errorf("Message %d: expected '%s', got '%s'", i, exp, result[i].Content())
		}
	}
}

// Test: buildMessages with leading system messages
func TestBuildMessages_WithLeadingSystemMessages(t *testing.T) {
	optMessages := []Message{
		&mockMessage{role: RoleSystem, content: "System 1"},
		&mockMessage{role: RoleSystem, content: "System 2"},
		&mockMessage{role: RoleUser, content: "User message"},
	}
	stateMessages := []Message{
		&mockMessage{role: RoleUser, content: "Previous message"},
	}

	result := buildMessages(optMessages, stateMessages)

	// Should be: leading system + state + other opt messages
	expected := []string{"System 1", "System 2", "Previous message", "User message"}
	if len(result) != 4 {
		t.Fatalf("Expected 4 messages, got %d", len(result))
	}
	for i, exp := range expected {
		if result[i].Content() != exp {
			t.Errorf("Message %d: expected '%s', got '%s'", i, exp, result[i].Content())
		}
	}
}

// Test: buildMessages with mixed system messages
func TestBuildMessages_MixedSystemMessages(t *testing.T) {
	optMessages := []Message{
		&mockMessage{role: RoleSystem, content: "Leading System"},
		&mockMessage{role: RoleUser, content: "User 1"},
		&mockMessage{role: RoleSystem, content: "Inline System"},
	}
	stateMessages := []Message{
		&mockMessage{role: RoleUser, content: "Previous"},
	}

	result := buildMessages(optMessages, stateMessages)

	// Leading system messages go first, then state, then remaining opt messages
	expected := []string{"Leading System", "Previous", "User 1", "Inline System"}
	if len(result) != 4 {
		t.Fatalf("Expected 4 messages, got %d", len(result))
	}
	for i, exp := range expected {
		if result[i].Content() != exp {
			t.Errorf("Message %d: expected '%s', got '%s'", i, exp, result[i].Content())
		}
	}
}

// Test: buildMessages with empty state
func TestBuildMessages_EmptyState(t *testing.T) {
	optMessages := []Message{
		&mockMessage{role: RoleSystem, content: "System"},
		&mockMessage{role: RoleUser, content: "User"},
	}
	stateMessages := []Message{}

	result := buildMessages(optMessages, stateMessages)

	expected := []string{"System", "User"}
	if len(result) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(result))
	}
	for i, exp := range expected {
		if result[i].Content() != exp {
			t.Errorf("Message %d: expected '%s', got '%s'", i, exp, result[i].Content())
		}
	}
}

// Test: buildMessages with nil state
func TestBuildMessages_NilState(t *testing.T) {
	optMessages := []Message{
		&mockMessage{role: RoleUser, content: "User"},
	}

	result := buildMessages(optMessages, nil)

	if len(result) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(result))
	}
	if result[0].Content() != "User" {
		t.Error("Message content not preserved")
	}
}

// Test: buildMessages with only system messages in opts
func TestBuildMessages_OnlySystemMessages(t *testing.T) {
	optMessages := []Message{
		&mockMessage{role: RoleSystem, content: "System 1"},
		&mockMessage{role: RoleSystem, content: "System 2"},
	}
	stateMessages := []Message{
		&mockMessage{role: RoleUser, content: "Previous"},
	}

	result := buildMessages(optMessages, stateMessages)

	// All opt messages are leading system messages, so: system + state
	expected := []string{"System 1", "System 2", "Previous"}
	if len(result) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(result))
	}
	for i, exp := range expected {
		if result[i].Content() != exp {
			t.Errorf("Message %d: expected '%s', got '%s'", i, exp, result[i].Content())
		}
	}
}

// Test: stripLeadingSystemMessages with leading system messages
func TestStripLeadingSystemMessages_WithLeading(t *testing.T) {
	messages := []Message{
		&mockMessage{role: RoleSystem, content: "System 1"},
		&mockMessage{role: RoleSystem, content: "System 2"},
		&mockMessage{role: RoleUser, content: "User"},
		&mockMessage{role: RoleAssistant, content: "Assistant"},
	}

	result := stripLeadingSystemMessages(messages)

	if len(result) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(result))
	}
	if result[0].Role() != RoleUser {
		t.Error("First message should be user")
	}
	if result[1].Role() != RoleAssistant {
		t.Error("Second message should be assistant")
	}
}

// Test: stripLeadingSystemMessages with no leading system messages
func TestStripLeadingSystemMessages_NoLeading(t *testing.T) {
	messages := []Message{
		&mockMessage{role: RoleUser, content: "User"},
		&mockMessage{role: RoleSystem, content: "System"},
	}

	result := stripLeadingSystemMessages(messages)

	// Should return all messages (no leading system)
	if len(result) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(result))
	}
	if result[0].Role() != RoleUser {
		t.Error("First message should be user")
	}
	if result[1].Role() != RoleSystem {
		t.Error("Second message (inline system) should be preserved")
	}
}

// Test: stripLeadingSystemMessages with only system messages
func TestStripLeadingSystemMessages_OnlySystem(t *testing.T) {
	messages := []Message{
		&mockMessage{role: RoleSystem, content: "System 1"},
		&mockMessage{role: RoleSystem, content: "System 2"},
	}

	result := stripLeadingSystemMessages(messages)

	// All messages are system, should return nil
	if result != nil {
		t.Errorf("Expected nil, got %d messages", len(result))
	}
}

// Test: stripLeadingSystemMessages with empty messages
func TestStripLeadingSystemMessages_Empty(t *testing.T) {
	messages := []Message{}

	result := stripLeadingSystemMessages(messages)

	if result != nil {
		t.Error("Expected nil for empty messages")
	}
}

// Test: stripLeadingSystemMessages with nil messages
func TestStripLeadingSystemMessages_Nil(t *testing.T) {
	result := stripLeadingSystemMessages(nil)

	if result != nil {
		t.Error("Expected nil for nil messages")
	}
}

// Test: extractLeadingSystemMessages with leading system messages
func TestExtractLeadingSystemMessages_WithLeading(t *testing.T) {
	messages := []Message{
		&mockMessage{role: RoleSystem, content: "System 1"},
		&mockMessage{role: RoleSystem, content: "System 2"},
		&mockMessage{role: RoleUser, content: "User"},
		&mockMessage{role: RoleSystem, content: "Inline System"},
	}

	result := extractLeadingSystemMessages(messages)

	if len(result) != 2 {
		t.Fatalf("Expected 2 leading system messages, got %d", len(result))
	}
	if result[0].Content() != "System 1" {
		t.Error("First system message content not preserved")
	}
	if result[1].Content() != "System 2" {
		t.Error("Second system message content not preserved")
	}
}

// Test: extractLeadingSystemMessages with no leading system messages
func TestExtractLeadingSystemMessages_NoLeading(t *testing.T) {
	messages := []Message{
		&mockMessage{role: RoleUser, content: "User"},
		&mockMessage{role: RoleSystem, content: "Inline System"},
	}

	result := extractLeadingSystemMessages(messages)

	// No leading system messages
	if result != nil {
		t.Errorf("Expected nil, got %d messages", len(result))
	}
}

// Test: extractLeadingSystemMessages with only system messages
func TestExtractLeadingSystemMessages_OnlySystem(t *testing.T) {
	messages := []Message{
		&mockMessage{role: RoleSystem, content: "System 1"},
		&mockMessage{role: RoleSystem, content: "System 2"},
	}

	result := extractLeadingSystemMessages(messages)

	// All messages are system, should return all
	if len(result) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(result))
	}
	if result[0].Content() != "System 1" || result[1].Content() != "System 2" {
		t.Error("System messages not preserved")
	}
}

// Test: extractLeadingSystemMessages with empty messages
func TestExtractLeadingSystemMessages_Empty(t *testing.T) {
	messages := []Message{}

	result := extractLeadingSystemMessages(messages)

	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %d messages", len(result))
	}
}

// Test: stripLeadingSystemMessages and extractLeadingSystemMessages are inverses
func TestStripAndExtract_AreInverses(t *testing.T) {
	original := []Message{
		&mockMessage{role: RoleSystem, content: "System 1"},
		&mockMessage{role: RoleSystem, content: "System 2"},
		&mockMessage{role: RoleUser, content: "User"},
		&mockMessage{role: RoleAssistant, content: "Assistant"},
	}

	stripped := stripLeadingSystemMessages(original)
	extracted := extractLeadingSystemMessages(original)

	// Recombining should give original
	combined := append(extracted, stripped...)

	if len(combined) != len(original) {
		t.Fatalf("Expected %d messages, got %d", len(original), len(combined))
	}

	for i, msg := range combined {
		if msg.Content() != original[i].Content() {
			t.Errorf("Message %d: expected '%s', got '%s'", i, original[i].Content(), msg.Content())
		}
	}
}

// ============================================================================
// Tests for state encoding/decoding (low-level contract tests)
// ============================================================================

// Test: State encoding/decoding round-trip
func TestChat_StateEncodingDecoding_RoundTrip(t *testing.T) {
	backend := &mockBackend{
		providerName: "test-provider",
	}

	chat := &Chat{Backend: backend}

	originalMessages := []Message{
		backend.NewUserMessage("Hello"),
		&mockMessage{role: RoleAssistant, content: "Hi there!"},
		backend.NewUserMessage("How are you?"),
	}

	// Encode
	state, err := chat.encodeState(originalMessages, len(originalMessages))
	if err != nil {
		t.Fatalf("Failed to encode state: %v", err)
	}

	if state == nil || len(state) == 0 {
		t.Fatal("Encoded state should not be empty")
	}

	// Decode
	decodedMessages, _ := chat.decodeState(context.Background(), state)

	if len(decodedMessages) != len(originalMessages) {
		t.Fatalf("Expected %d messages, got %d", len(originalMessages), len(decodedMessages))
	}

	for i, msg := range decodedMessages {
		if msg.Role() != originalMessages[i].Role() {
			t.Errorf("Message %d: expected role %s, got %s", i, originalMessages[i].Role(), msg.Role())
		}
		if msg.Content() != originalMessages[i].Content() {
			t.Errorf("Message %d: expected content %s, got %s", i, originalMessages[i].Content(), msg.Content())
		}
	}
}

// Test: RoleOther messages are correctly round-tripped through state
func TestChat_StateEncodingDecoding_RoleOtherRoundTrip(t *testing.T) {
	backend := &mockBackend{
		providerName: "test-provider",
	}

	chat := &Chat{Backend: backend}

	// Create conversation with various message types including RoleOther
	originalMessages := []Message{
		backend.NewUserMessage("Hello"),
		&mockMessage{role: RoleAssistant, content: "Hi there!"},
		&mockMessage{role: RoleOther, content: "Thinking: I should respond politely"},
		backend.NewUserMessage("How are you?"),
		&mockMessage{role: RoleAssistant, content: "I'm doing well"},
		&mockMessage{role: RoleOther, content: "Internal reasoning about the conversation"},
	}

	// Encode
	state, err := chat.encodeState(originalMessages, len(originalMessages))
	if err != nil {
		t.Fatalf("Failed to encode state with RoleOther messages: %v", err)
	}

	if state == nil || len(state) == 0 {
		t.Fatal("Encoded state should not be empty")
	}

	// Decode
	decodedMessages, _ := chat.decodeState(context.Background(), state)

	if len(decodedMessages) != len(originalMessages) {
		t.Fatalf("Expected %d messages, got %d", len(originalMessages), len(decodedMessages))
	}

	// Verify all messages including RoleOther are preserved
	for i, msg := range decodedMessages {
		if msg.Role() != originalMessages[i].Role() {
			t.Errorf("Message %d: expected role %s, got %s", i, originalMessages[i].Role(), msg.Role())
		}
		if msg.Content() != originalMessages[i].Content() {
			t.Errorf("Message %d: expected content %s, got %s", i, originalMessages[i].Content(), msg.Content())
		}
	}

	// Specifically verify RoleOther messages
	if decodedMessages[2].Role() != RoleOther {
		t.Errorf("Message 2 should be RoleOther, got %s", decodedMessages[2].Role())
	}
	if decodedMessages[5].Role() != RoleOther {
		t.Errorf("Message 5 should be RoleOther, got %s", decodedMessages[5].Role())
	}
}

// Test: ProcessedLength is correctly serialized in state
func TestChat_StateEncodingDecoding_ProcessedLength(t *testing.T) {
	backend := &mockBackend{
		providerName: "test-provider",
	}

	chat := &Chat{Backend: backend}

	// Create messages and encode with a specific ProcessedLength
	messages := []Message{
		backend.NewUserMessage("Message 1"),
		&mockMessage{role: RoleAssistant, content: "Response 1"},
		backend.NewUserMessage("Message 2"),
		&mockMessage{role: RoleAssistant, content: "Response 2"},
		backend.NewUserMessage("Message 3 - appended via AppendToState"),
	}

	// Encode with ProcessedLength = 4 (first 4 messages were seen by LLM, last message was appended)
	expectedProcessedLength := 4
	state, err := chat.encodeState(messages, expectedProcessedLength)
	if err != nil {
		t.Fatalf("Failed to encode state: %v", err)
	}

	// Decode the state as JSON to verify ProcessedLength was serialized
	var internal conversationStateInternal
	err = json.Unmarshal(state, &internal)
	if err != nil {
		t.Fatalf("Failed to unmarshal state: %v", err)
	}

	// Verify ProcessedLength was correctly serialized
	if internal.ProcessedLength != expectedProcessedLength {
		t.Errorf("ProcessedLength in serialized state: expected %d, got %d", expectedProcessedLength, internal.ProcessedLength)
	}

	// Also verify via decodeState
	_, decodedProcessedLength := chat.decodeState(context.Background(), state)
	if decodedProcessedLength != expectedProcessedLength {
		t.Errorf("ProcessedLength from decodeState: expected %d, got %d", expectedProcessedLength, decodedProcessedLength)
	}
}

// Test: Provider mismatch discards state
func TestChat_DecodeState_ProviderMismatch_DiscardsState(t *testing.T) {
	// Create state with one provider
	backend1 := &mockBackend{providerName: "provider-a"}
	chat1 := &Chat{Backend: backend1}
	state, _ := chat1.encodeState([]Message{
		backend1.NewUserMessage("test"),
	}, 1)

	// Try to decode with different provider
	backend2 := &mockBackend{providerName: "provider-b"}
	chat2 := &Chat{Backend: backend2}
	messages, _ := chat2.decodeState(context.Background(), state)

	// Should return nil (graceful degradation)
	if messages != nil {
		t.Error("Expected nil messages when provider mismatches")
	}
}

// Test: Invalid state is gracefully ignored
func TestChat_DecodeState_InvalidState_ReturnsNil(t *testing.T) {
	backend := &mockBackend{providerName: "test"}
	chat := &Chat{Backend: backend}

	// Corrupt state
	invalidState := ConversationState([]byte("not valid json"))

	messages, _ := chat.decodeState(context.Background(), invalidState)

	// Should return nil (graceful degradation)
	if messages != nil {
		t.Error("Expected nil messages for invalid state")
	}
}
