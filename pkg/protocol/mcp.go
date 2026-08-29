package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	MethodToolsCall = "tools/call"
)

// ToolsCallParams is the MCP tools/call request payload.
type ToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResult is the MCP tools/call response payload.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a single content item in an MCP tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// EchoInterventionMessage formats the echo breaker intervention text.
func EchoInterventionMessage(toolName string) string {
	return fmt.Sprintf(
		"Error: Command [%s] generated identical failures across consecutive loops. Do not retry without modifying parameters.",
		toolName,
	)
}

// BuildEchoInterventionResult builds an MCP tool result for echo trips.
func BuildEchoInterventionResult(toolName string) ToolCallResult {
	return ToolCallResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: EchoInterventionMessage(toolName),
		}},
		IsError: true,
	}
}
