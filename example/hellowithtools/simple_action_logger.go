package main

import (
	"fmt"

	"github.com/m0rjc/goaitools/aitooling"
)

// SimpleToolActionLogger accumulates tool actions and prints them to stdout on demand.
type SimpleToolActionLogger struct {
	actions []aitooling.ToolAction
}

// Log appends a single action to the accumulated list.
func (l *SimpleToolActionLogger) Log(action aitooling.ToolAction) {
	l.actions = append(l.actions, action)
}

// LogAll appends multiple actions to the accumulated list.
func (l *SimpleToolActionLogger) LogAll(actions []aitooling.ToolAction) {
	l.actions = append(l.actions, actions...)
}

// PrintAndClear prints all accumulated actions and clears the list.
// If no actions were logged, it prints a message indicating that.
func (l *SimpleToolActionLogger) PrintAndClear() {
	if len(l.actions) == 0 {
		fmt.Println("  [No tool actions were logged]")
	} else {
		for _, action := range l.actions {
			fmt.Printf("  [TOOL ACTION] %s\n", action.Description())
		}
	}
	l.actions = nil
}
