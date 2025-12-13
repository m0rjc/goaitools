package aitooling

// ToolAction A log of an action executed by a tool.
type ToolAction interface {
	// Description returns a human-readable description of the action, as could be presented in a bulleted list.
	Description() string
}

// Logger logs tool actions.
type Logger interface {
	// Log an action made by the tool. This is best effort and should not fail.
	Log(action ToolAction)
	// LogAll actions made by the tool. This is best effort and should not fail.
	LogAll(actions []ToolAction)
}

// LogAccumulator handles the core job of storing log entries.
type LogAccumulator struct {
	entries []ToolAction
}

// NewLogAccumulator creates a new log accumulator.
func NewLogAccumulator() *LogAccumulator {
	return &LogAccumulator{
		entries: make([]ToolAction, 0),
	}
}

// Log logs a single entry.
func (a *LogAccumulator) Log(entry ToolAction) {
	a.entries = append(a.entries, entry)
}

// LogAll logs multiple entries at once.
func (a *LogAccumulator) LogAll(entries []ToolAction) {
	a.entries = append(a.entries, entries...)
}

// SendTo writes all accumulated entries to the target logger.
// This is intended for situations where a series of actions may roll back.
// Send the accumulated entries to the target logger after committing the transaction.
func (a *LogAccumulator) SendTo(target Logger) {
	if len(a.entries) > 0 {
		target.LogAll(a.entries)
	}
}

// Clear clears the accumulated entries.
func (a *LogAccumulator) Clear() {
	a.entries = a.entries[:0] // Clear the slice
}
