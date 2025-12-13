package aitooling

import "testing"

// Test: Logger interface contract - implementations must accept actions
func TestLogger_InterfaceContract(t *testing.T) {
	// Verify LogAccumulator implements Logger
	var _ Logger = &LogAccumulator{}

	acc := NewLogAccumulator()
	action := mockAction{desc: "test action"}

	// Should not panic
	acc.Log(action)
	acc.LogAll([]ToolAction{action})
}

// Test: LogAccumulator accumulates actions for later processing
func TestLogAccumulator_AccumulatesActionsForLaterProcessing(t *testing.T) {
	acc := NewLogAccumulator()
	target := &mockLogger{}

	// Add actions during operation
	acc.Log(mockAction{desc: "step 1"})
	acc.Log(mockAction{desc: "step 2"})

	// Target should not have received anything yet
	if len(target.logged) != 0 {
		t.Error("Target should not receive actions before SendTo")
	}

	// Send accumulated actions
	acc.SendTo(target)

	// Now target should have all actions
	if len(target.logged) != 2 {
		t.Errorf("Expected 2 actions in target, got %d", len(target.logged))
	}
}

// Test: LogAccumulator.Clear allows accumulator reuse
func TestLogAccumulator_Clear_AllowsReuse(t *testing.T) {
	acc := NewLogAccumulator()

	// First batch of operations
	acc.Log(mockAction{desc: "old action"})
	acc.Clear()

	// Second batch
	acc.Log(mockAction{desc: "new action"})

	target := &mockLogger{}
	acc.SendTo(target)

	// Should only have new action
	if len(target.logged) != 1 {
		t.Errorf("Expected 1 action after clear, got %d", len(target.logged))
	}

	if target.logged[0].Description() != "new action" {
		t.Error("Should only have new action after clear")
	}
}

// Test: SendTo with empty accumulator is a no-op
func TestLogAccumulator_SendTo_EmptyIsNoOp(t *testing.T) {
	acc := NewLogAccumulator()
	target := &mockLogger{}

	acc.SendTo(target)

	if len(target.logged) != 0 {
		t.Errorf("Empty accumulator should not send actions, got %d", len(target.logged))
	}
}

// Test: LogAll preserves order of actions
func TestLogAccumulator_LogAll_PreservesOrder(t *testing.T) {
	acc := NewLogAccumulator()

	actions := []ToolAction{
		mockAction{desc: "first"},
		mockAction{desc: "second"},
		mockAction{desc: "third"},
	}

	acc.LogAll(actions)

	target := &mockLogger{}
	acc.SendTo(target)

	// Verify order is preserved
	for i, expected := range actions {
		if target.logged[i].Description() != expected.Description() {
			t.Errorf("Action %d: expected '%s', got '%s'",
				i, expected.Description(), target.logged[i].Description())
		}
	}
}

// Test: Multiple SendTo calls send the same accumulated actions
func TestLogAccumulator_SendTo_CanSendMultipleTimes(t *testing.T) {
	acc := NewLogAccumulator()
	acc.Log(mockAction{desc: "action"})

	target1 := &mockLogger{}
	target2 := &mockLogger{}

	acc.SendTo(target1)
	acc.SendTo(target2)

	// Both targets should receive the action
	if len(target1.logged) != 1 || len(target2.logged) != 1 {
		t.Error("Both targets should receive accumulated actions")
	}
}
