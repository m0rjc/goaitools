package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0rjc/goaitools/aitooling"
)

// WriteGameTool allows the AI to update game properties.
// Only the properties specified in the tool call arguments are updated.
// Actions are logged for each property that is actually changed.
type WriteGameTool struct {
	game *Game
}

// NewWriteGameTool creates a new WriteGameTool for the given game.
func NewWriteGameTool(game *Game) *WriteGameTool {
	return &WriteGameTool{game: game}
}

// Name returns the tool name for OpenAI function calling.
func (t *WriteGameTool) Name() string {
	return "write_game"
}

// Description returns a description of what this tool does.
// This is sent to the AI to help it decide when to use the tool.
func (t *WriteGameTool) Description() string {
	return "Update game properties. Only provide the properties you want to change. Accepts title (string), start_date (RFC3339 format), duration_minutes (integer), grid_m (integer), grid_n (integer)"
}

// Parameters returns the JSON Schema for this tool's parameters.
// All parameters are optional - only provided parameters will be updated.
func (t *WriteGameTool) Parameters() json.RawMessage {
	return aitooling.MustMarshalJSON(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "The game title",
			},
			"start_date": map[string]interface{}{
				"type":        "string",
				"description": "The game start date in RFC3339 format (e.g., 2024-01-15T14:30:00Z)",
			},
			"duration_minutes": map[string]interface{}{
				"type":        "integer",
				"description": "The game duration in minutes",
			},
			"grid_m": map[string]interface{}{
				"type":        "integer",
				"description": "The grid M dimension (rows)",
			},
			"grid_n": map[string]interface{}{
				"type":        "integer",
				"description": "The grid N dimension (columns)",
			},
		},
	})
}

// Execute updates the game properties based on the provided arguments.
// Only parameters present in the request are updated.
// Logs an action for each property that is changed.
// Returns a result indicating which properties were updated.
//
// This demonstrates a simple transactional pattern:
//  1. Clone the game state
//  2. Apply changes to the clone
//  3. If all validations pass, commit the clone back to the original
//  4. Flush the accumulated action log
func (t *WriteGameTool) Execute(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(req.Args), &params); err != nil {
		return req.NewErrorResult(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	// Clone the game for transactional updates
	workingCopy := t.game.Clone()

	// Create a log accumulator to collect actions during the update operation.
	// If we return an error, these actions will be discarded as appropriate given a database rollback.
	logAccumulator := aitooling.NewLogAccumulator()
	// The updates list is used to feed back changes to the AI. This is independent of the human facing Tool Logger.
	updates := make([]string, 0, 5)

	if title, ok := params["title"].(string); ok {
		workingCopy.Title = title
		updates = append(updates, fmt.Sprintf("title to '%s'", title))
		logAccumulator.Log(NewSimpleAction(fmt.Sprintf("Updated title to '%s'", title)))
	}

	if startDateStr, ok := params["start_date"].(string); ok {
		startDate, err := time.Parse(time.RFC3339, startDateStr)
		if err != nil {
			// Return error without committing - working copy is discarded
			return req.NewErrorResult(fmt.Errorf("invalid start_date format: %w", err)), nil
		}
		workingCopy.StartDate = startDate
		updates = append(updates, fmt.Sprintf("start_date to %s", startDateStr))
		logAccumulator.Log(NewSimpleAction(fmt.Sprintf("Updated start_date to %s", startDateStr)))
	}

	if durationMinutes, ok := params["duration_minutes"].(float64); ok {
		workingCopy.DurationMinutes = int(durationMinutes)
		updates = append(updates, fmt.Sprintf("duration_minutes to %d", int(durationMinutes)))
		logAccumulator.Log(NewSimpleAction(fmt.Sprintf("Updated duration_minutes to %d", int(durationMinutes))))
	}

	if gridM, ok := params["grid_m"].(float64); ok {
		workingCopy.GridM = int(gridM)
		updates = append(updates, fmt.Sprintf("grid_m to %d", int(gridM)))
		logAccumulator.Log(NewSimpleAction(fmt.Sprintf("Updated grid_m to %d", int(gridM))))
	}

	if gridN, ok := params["grid_n"].(float64); ok {
		workingCopy.GridN = int(gridN)
		updates = append(updates, fmt.Sprintf("grid_n to %d", int(gridN)))
		logAccumulator.Log(NewSimpleAction(fmt.Sprintf("Updated grid_n to %d", int(gridN))))
	}

	resultData := map[string]interface{}{
		"success": true,
		"updated": updates,
	}

	resultJSON, err := json.Marshal(resultData)
	if err != nil {
		return req.NewErrorResult(err), nil
	}

	// Commit the working copy back to the original game
	t.game.CommitFrom(workingCopy)

	// Flush accumulated actions to the logger
	logAccumulator.SendTo(ctx.Logger)

	return req.NewResult(string(resultJSON)), nil
}
