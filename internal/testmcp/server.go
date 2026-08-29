// Package testmcp provides a minimal fake MCP server for integration tests.
package testmcp

import (
	"encoding/json"
	"io"
	"os"
	"sync/atomic"

	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

var toolCallCount atomic.Int32

// RunFakeServer reads MCP frames from stdin and writes responses to stdout.
func RunFakeServer() error {
	reader := proxy.NewFrameReader(os.Stdin, nil)
	for {
		frame, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch frame.Method {
		case "initialize":
			result, _ := json.Marshal(map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{},
				"serverInfo":      map[string]string{"name": "testmcp", "version": "0.0.1"},
			})
			_ = proxy.WriteFrame(os.Stdout, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Result: result})
		case "tools/list":
			result, _ := json.Marshal(map[string]any{"tools": []any{}})
			_ = proxy.WriteFrame(os.Stdout, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Result: result})
		case "tools/call":
			toolCallCount.Add(1)
			result, _ := json.Marshal(map[string]any{
				"content": []map[string]string{{"type": "text", "text": "ok"}},
				"isError": false,
			})
			_ = proxy.WriteFrame(os.Stdout, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Result: result})
		default:
			if frame.IsRequest() {
				errObj, _ := json.Marshal(map[string]any{"code": -32601, "message": "method not found"})
				_ = proxy.WriteFrame(os.Stdout, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Error: errObj})
			}
		}
	}
}

// ToolCallCount returns how many tools/call requests the fake server handled.
func ToolCallCount() int32 {
	return toolCallCount.Load()
}

// ResetToolCallCount clears the fake server counter (testing aid).
func ResetToolCallCount() {
	toolCallCount.Store(0)
}
