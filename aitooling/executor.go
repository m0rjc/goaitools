package aitooling

import (
	"context"
	"fmt"
)

var (
	ErrToolNotFound = fmt.Errorf("tool not found")
)

// ToolRunner is a function that executes a tool request.
type ToolRunner func(request *ToolRequest) (*ToolResult, error)

func (ts ToolSet) getTool(name string) Tool {
	// Using a linear search on a small array (10 to 15 items) will be faster than using
	// a map or a hash table
	for _, tool := range ts {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
}

// Runner returns a function that executes tools.
// Errors are typically returned as ToolResults via NewErrorResult().
// The error return path is reserved for unexpected infrastructure failures.
//
// Parameters:
//   - ctx: Standard Go context for cancellation and deadlines
//   - log: Logger for recording tool actions
func (ts ToolSet) Runner(ctx context.Context, log Logger) ToolRunner {
	return func(request *ToolRequest) (*ToolResult, error) {
		executeContext := ToolExecuteContext{
			Context: ctx,
			Logger:  log,
		}

		tool := ts.getTool(request.Name)
		if tool == nil {
			return request.NewErrorResult(ErrToolNotFound), nil
		}

		return tool.Execute(executeContext, request)
	}
}
