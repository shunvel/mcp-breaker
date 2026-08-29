package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	MethodToolsCall = "tools/call"

	semanticInterventionPrefix = "[CRITICAL REASONING ALERT]"
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

// SemanticInterventionMessage returns the spec 3.3 system override notice.
func SemanticInterventionMessage() string {
	return semanticInterventionPrefix + " You have entered a semantic loop. You are continuously requesting the same files without creating distinct outcomes. Step back, abandon your current path, check alternate files, or request explicit clarification from the user."
}

// BuildSemanticInterventionResult appends semantic intervention to an existing result.
func BuildSemanticInterventionResult(existing ToolCallResult) ToolCallResult {
	msg := SemanticInterventionMessage()
	if len(existing.Content) == 0 {
		existing.Content = []ContentBlock{{Type: "text", Text: msg}}
	} else {
		last := existing.Content[len(existing.Content)-1]
		if last.Type == "" {
			last.Type = "text"
		}
		if last.Text != "" {
			last.Text += "\n\n" + msg
		} else {
			last.Text = msg
		}
		existing.Content[len(existing.Content)-1] = last
	}
	existing.IsError = true
	return existing
}

// GraphLoopInterventionMessage formats a graph loop block message.
func GraphLoopInterventionMessage(pattern string) string {
	return fmt.Sprintf(
		"Error: Detected tool invocation loop [%s]. Execution paused. Open `mcp-breaker dashboard` and press R to resume, or change your approach.",
		pattern,
	)
}

// BuildGraphLoopInterventionResult builds an MCP tool result for graph loop trips.
func BuildGraphLoopInterventionResult(pattern string) ToolCallResult {
	return ToolCallResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: GraphLoopInterventionMessage(pattern),
		}},
		IsError: true,
	}
}

// ExtractResultText concatenates text fields from a tools/call result JSON payload.
func ExtractResultText(result json.RawMessage) string {
	if len(result) == 0 {
		return ""
	}
	var tr ToolCallResult
	if err := json.Unmarshal(result, &tr); err != nil {
		return strings.TrimSpace(string(result))
	}
	var parts []string
	for _, c := range tr.Content {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// MarshalToolCallResult serializes a tool call result.
func MarshalToolCallResult(r ToolCallResult) (json.RawMessage, error) {
	return json.Marshal(r)
}
