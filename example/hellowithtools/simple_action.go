package main

// SimpleAction is a basic ToolAction implementation that stores a description string.
// It implements the aitooling.ToolAction interface.
type SimpleAction struct {
	description string
}

// NewSimpleAction creates a new SimpleAction with the given description.
func NewSimpleAction(description string) SimpleAction {
	return SimpleAction{description: description}
}

// Description returns the action description.
// This implements the aitooling.ToolAction interface.
func (a SimpleAction) Description() string {
	return a.description
}
