package aitooling

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolExecuteContext provides everything a tool needs to execute.
//
// This is designed to be generic and reusable across projects:
//   - Context: Standard Go context for HTTP client, cancellation, deadlines
//   - Logger: For logging tool actions
type ToolExecuteContext struct {
	Context context.Context // Go context for cancellation/deadlines
	Logger  Logger          // For logging tool actions
}

type ToolRequest struct {
	Name   string
	CallId string
	Args   string
}

type ToolResult struct {
	CallId string
	Result string
}

// NewResult creates a successful tool result.
func (req *ToolRequest) NewResult(result string) *ToolResult {
	return &ToolResult{
		CallId: req.CallId,
		Result: result,
	}
}

// NewErrorResult creates an error tool result.
func (req *ToolRequest) NewErrorResult(err error) *ToolResult {
	return &ToolResult{
		CallId: req.CallId,
		Result: fmt.Sprintf("Error: %v", err),
	}
}

type Tool interface {
	// Name is the name of the tool.
	Name() string
	// Description is a short description of the tool for the AI assistant.
	Description() string
	// Parameters is a JSON Schema describing the parameters accepted by the tool.
	Parameters() json.RawMessage
	// Execute executes the tool.
	Execute(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error)
}

type ToolSet []Tool
