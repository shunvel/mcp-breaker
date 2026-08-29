package breaker

import (
	"encoding/json"
	"testing"

	"github.com/shunvel/mcp-breaker/pkg/embed"
	"github.com/shunvel/mcp-breaker/pkg/protocol"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

func TestCaseB_SemanticStagnation(t *testing.T) {
	d := NewSemanticDetector(embed.NewMockEmbedder(), 0.88, nil)
	texts := []string{
		"Port 8080 bound",
		"Error: listen EADDRINUSE: address already in use 8080",
		"Cannot bind network to 8080",
	}
	for i, text := range texts {
		result, _ := json.Marshal(protocol.ToolCallResult{
			Content: []protocol.ContentBlock{{Type: "text", Text: text}},
		})
		frame := proxy.Frame{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: result}
		out := d.OnServerFrame(frame)
		if i < 2 {
			if string(out.Result) != string(result) {
				t.Fatalf("turn %d should not mutate yet", i+1)
			}
			continue
		}
		if !contains(string(out.Result), "CRITICAL REASONING ALERT") {
			t.Fatalf("turn 3 expected semantic intervention, got %s", out.Result)
		}
		if d.LastSimilarity() < 0.88 {
			t.Fatalf("expected similarity >= 0.88, got %f", d.LastSimilarity())
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
