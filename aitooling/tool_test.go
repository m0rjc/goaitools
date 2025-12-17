package aitooling

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockTool is a minimal Tool implementation for behavioral testing.
type mockTool struct {
	name        string
	description string
	parameters  json.RawMessage
	executeFunc func(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error)
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string         { return m.description }
func (m *mockTool) Parameters() json.RawMessage { return m.parameters }
func (m *mockTool) Execute(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return req.NewResult("success"), nil
}

// mockLogger for testing
type mockLogger struct {
	logged []ToolAction
}

func (m *mockLogger) Log(action ToolAction) {
	m.logged = append(m.logged, action)
}

func (m *mockLogger) LogAll(actions []ToolAction) {
	m.logged = append(m.logged, actions...)
}

type mockAction struct{ desc string }

func (m mockAction) Description() string { return m.desc }

// Test: ToolRequest.NewResult creates a result with correct CallId
func TestToolRequest_NewResult_PreservesCallId(t *testing.T) {
	req := &ToolRequest{
		Name:   "test_tool",
		CallId: "call_12345",
		Args:   `{}`,
	}

	result := req.NewResult("operation successful")

	if result.CallId != req.CallId {
		t.Errorf("Expected CallId=%s, got %s", req.CallId, result.CallId)
	}

	if result.Result != "operation successful" {
		t.Errorf("Expected Result='operation successful', got '%s'", result.Result)
	}
}

// Test: ToolRequest.NewErrorResult formats error correctly
func TestToolRequest_NewErrorResult_FormatsError(t *testing.T) {
	req := &ToolRequest{
		Name:   "test_tool",
		CallId: "call_12345",
		Args:   `{}`,
	}

	testErr := errors.New("something went wrong")
	result := req.NewErrorResult(testErr)

	if result.CallId != req.CallId {
		t.Errorf("Expected CallId=%s, got %s", req.CallId, result.CallId)
	}

	expectedResult := "Error: something went wrong"
	if result.Result != expectedResult {
		t.Errorf("Expected Result='%s', got '%s'", expectedResult, result.Result)
	}
}

// Test: ToolSet.Runner finds and executes tools by name
func TestToolSet_Runner_FindsToolByName(t *testing.T) {
	executedTool := ""

	tools := ToolSet{
		&mockTool{
			name: "tool_a",
			executeFunc: func(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error) {
				executedTool = "tool_a"
				return req.NewResult("a executed"), nil
			},
		},
		&mockTool{
			name: "tool_b",
			executeFunc: func(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error) {
				executedTool = "tool_b"
				return req.NewResult("b executed"), nil
			},
		},
	}

	logger := &mockLogger{}
	runner := tools.Runner(context.Background(), logger)

	// Execute tool_b
	result, err := runner(&ToolRequest{
		Name:   "tool_b",
		CallId: "call_1",
		Args:   `{}`,
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if executedTool != "tool_b" {
		t.Errorf("Expected tool_b to execute, got %s", executedTool)
	}

	if result.Result != "b executed" {
		t.Errorf("Expected result='b executed', got '%s'", result.Result)
	}
}

// Test: ToolSet.Runner returns error result for unknown tool
func TestToolSet_Runner_UnknownTool_ReturnsErrorResult(t *testing.T) {
	tools := ToolSet{
		&mockTool{name: "existing_tool"},
	}

	logger := &mockLogger{}
	runner := tools.Runner(context.Background(), logger)

	result, err := runner(&ToolRequest{
		Name:   "nonexistent_tool",
		CallId: "call_1",
		Args:   `{}`,
	})

	// Should return error result, NOT an error
	if err != nil {
		t.Errorf("Expected nil error (error as result), got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Result should contain error message
	if result.Result != "Error: tool not found" {
		t.Errorf("Expected 'Error: tool not found', got '%s'", result.Result)
	}
}

// Test: ToolExecuteContext provides Logger to tools
func TestToolSet_Runner_ProvidesLoggerToTools(t *testing.T) {
	var receivedLogger Logger

	tools := ToolSet{
		&mockTool{
			name: "test_tool",
			executeFunc: func(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error) {
				receivedLogger = ctx.Logger
				return req.NewResult("ok"), nil
			},
		},
	}

	logger := &mockLogger{}
	runner := tools.Runner(context.Background(), logger)

	runner(&ToolRequest{
		Name:   "test_tool",
		CallId: "call_1",
		Args:   `{}`,
	})

	if receivedLogger != logger {
		t.Error("Tool did not receive the correct logger")
	}
}

// Test: ToolExecuteContext provides Context for cancellation
func TestToolSet_Runner_ProvidesContextToTools(t *testing.T) {
	var receivedContext context.Context

	tools := ToolSet{
		&mockTool{
			name: "test_tool",
			executeFunc: func(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error) {
				receivedContext = ctx.Context
				return req.NewResult("ok"), nil
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &mockLogger{}
	runner := tools.Runner(ctx, logger)

	runner(&ToolRequest{
		Name:   "test_tool",
		CallId: "call_1",
		Args:   `{}`,
	})

	if receivedContext != ctx {
		t.Error("Tool did not receive the correct context")
	}
}

// Test: Tool can propagate infrastructure errors
func TestToolSet_Runner_PropagatesInfrastructureErrors(t *testing.T) {
	infraError := errors.New("database connection failed")

	tools := ToolSet{
		&mockTool{
			name: "failing_tool",
			executeFunc: func(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error) {
				return nil, infraError // Infrastructure failure
			},
		},
	}

	logger := &mockLogger{}
	runner := tools.Runner(context.Background(), logger)

	result, err := runner(&ToolRequest{
		Name:   "failing_tool",
		CallId: "call_1",
		Args:   `{}`,
	})

	// Infrastructure errors should be propagated
	if err != infraError {
		t.Errorf("Expected infrastructure error to be propagated, got %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil result for infrastructure error, got %v", result)
	}
}

// Test: Tool can handle domain errors via NewErrorResult
func TestToolSet_Runner_DomainErrorsReturnedAsResults(t *testing.T) {
	domainError := errors.New("invalid game code")

	tools := ToolSet{
		&mockTool{
			name: "business_tool",
			executeFunc: func(ctx ToolExecuteContext, req *ToolRequest) (*ToolResult, error) {
				// Domain error returned as result
				return req.NewErrorResult(domainError), nil
			},
		},
	}

	logger := &mockLogger{}
	runner := tools.Runner(context.Background(), logger)

	result, err := runner(&ToolRequest{
		Name:   "business_tool",
		CallId: "call_1",
		Args:   `{}`,
	})

	// No infrastructure error
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Error embedded in result
	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	expectedMsg := "Error: invalid game code"
	if result.Result != expectedMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, result.Result)
	}
}

// Test: Empty ToolSet works correctly
func TestToolSet_Runner_EmptyToolSet(t *testing.T) {
	tools := ToolSet{}

	logger := &mockLogger{}
	runner := tools.Runner(context.Background(), logger)

	result, err := runner(&ToolRequest{
		Name:   "any_tool",
		CallId: "call_1",
		Args:   `{}`,
	})

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if result.Result != "Error: tool not found" {
		t.Errorf("Expected tool not found error, got '%s'", result.Result)
	}
}
