package main

import (
	"encoding/json"
	"time"

	"github.com/m0rjc/goaitools/aitooling"
)

// ReadGameTool allows the AI to read the current game properties.
// This tool has no parameters and returns all game properties as JSON.
// It does not log any actions since read operations don't modify state.
type ReadGameTool struct {
	game *Game
}

// NewReadGameTool creates a new ReadGameTool for the given game.
func NewReadGameTool(game *Game) *ReadGameTool {
	return &ReadGameTool{game: game}
}

// Name returns the tool name for OpenAI function calling.
func (t *ReadGameTool) Name() string {
	return "read_game"
}

// Description returns a description of what this tool does.
// This is sent to the AI to help it decide when to use the tool.
func (t *ReadGameTool) Description() string {
	return "Read the current game properties including title, start date, duration, and grid dimensions"
}

// Parameters returns the JSON Schema for this tool's parameters.
// This tool takes no parameters, so we return an empty schema.
func (t *ReadGameTool) Parameters() json.RawMessage {
	return aitooling.EmptyJsonSchema()
}

// Execute reads the game properties and returns them as JSON.
// No actions are logged since this is a read-only operation.
func (t *ReadGameTool) Execute(ctx aitooling.ToolExecuteContext, req *aitooling.ToolRequest) (*aitooling.ToolResult, error) {
	// No action logged for read operations

	resultData := map[string]interface{}{
		"title":            t.game.Title,
		"start_date":       t.game.StartDate.Format(time.RFC3339),
		"duration_minutes": t.game.DurationMinutes,
		"grid_m":           t.game.GridM,
		"grid_n":           t.game.GridN,
	}

	resultJSON, err := json.Marshal(resultData)
	if err != nil {
		return req.NewErrorResult(err), nil
	}

	return req.NewResult(string(resultJSON)), nil
}
