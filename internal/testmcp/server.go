// Package testmcp provides a minimal fake MCP server for integration tests.
package testmcp

import (
	"encoding/json"
	"io"
	"os"
	"sync/atomic"

	"github.com/shunvel/mcp-breaker/pkg/protocol"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

var toolCallCount atomic.Int32

// PortErrorTexts are paraphrased bind failures used for semantic stagnation demos.
var PortErrorTexts = []string{
	"Port 8080 bound",
	"Error: listen EADDRINUSE: address already in use 8080",
	"Cannot bind network to 8080",
}

// RunFakeServer reads MCP frames from stdin and writes responses to stdout.
func RunFakeServer() error {
	return RunFakeServerWithIO(os.Stdin, os.Stdout)
}

// RunFakeServerWithIO runs the fake MCP server on the given streams (testing aid).
func RunFakeServerWithIO(in io.Reader, out io.Writer) error {
	reader := proxy.NewFrameReader(in, nil)
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
			_ = proxy.WriteFrame(out, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Result: result})
		case "tools/list":
			result, _ := json.Marshal(map[string]any{"tools": []any{}})
			_ = proxy.WriteFrame(out, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Result: result})
		case "tools/call":
			toolCallCount.Add(1)
			text := "ok"
			var params protocol.ToolsCallParams
			if err := json.Unmarshal(frame.Params, &params); err == nil && params.Name == "simulate_port_error" {
				text = PortErrorTexts[(toolCallCount.Load()-1)%int32(len(PortErrorTexts))]
			}
			result, _ := json.Marshal(map[string]any{
				"content": []map[string]string{{"type": "text", "text": text}},
				"isError": params.Name == "simulate_port_error",
			})
			_ = proxy.WriteFrame(out, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Result: result})
		default:
			if frame.IsRequest() {
				errObj, _ := json.Marshal(map[string]any{"code": -32601, "message": "method not found"})
				_ = proxy.WriteFrame(out, proxy.Frame{JSONRPC: "2.0", ID: frame.ID, Error: errObj})
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
