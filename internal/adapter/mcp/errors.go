package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Standard error codes defined in the SpecTest MCP contract.
const (
	ErrCodeInvalidInput      = "invalid_input"
	ErrCodePathOutsideRoot   = "path_outside_root"
	ErrCodeNotFound          = "not_found"
	ErrCodeIntegrityConflict = "integrity_conflict"
	ErrCodeLLMRequired       = "llm_required"
	ErrCodeLockedEntity      = "locked_entity"
	ErrCodeJobNotFound       = "job_not_found"
	ErrCodeJobFailed         = "job_failed"
	ErrCodeCancelled         = "cancelled"
)

// ErrorPayload represents a standardized, machine-readable MCP error response.
type ErrorPayload struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e *ErrorPayload) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ToolError creates a standard MCP error tool result containing serialized ErrorPayload.
func ToolError(code, message string, retryable bool, details map[string]any) *mcp.CallToolResult {
	payload := ErrorPayload{
		Code:      code,
		Message:   message,
		Retryable: retryable,
		Details:   details,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("[%s] %s", code, message))
	}
	return mcp.NewToolResultError(string(bytes))
}
