package breaker

import (
	"encoding/json"
	"testing"

	"github.com/shunvel/mcp-breaker/pkg/config"
	"github.com/shunvel/mcp-breaker/pkg/protocol"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

func TestGraphLedger_ABAB(t *testing.T) {
	g := NewGraphLedger(config.Default(), nil)
	frames := []proxy.Frame{
		toolsCall("write_file", `{"p":"a"}`),
		toolsCall("read_file", `{"p":"b"}`),
		toolsCall("write_file", `{"p":"a"}`),
		toolsCall("read_file", `{"p":"b"}`),
	}
	for i := 0; i < 3; i++ {
		if d := g.OnClientFrame(frames[i]); !d.Forward {
			t.Fatalf("call %d should forward", i+1)
		}
	}
	if d := g.OnClientFrame(frames[3]); d.Forward {
		t.Fatal("4th ABAB call should block")
	}
	if !g.Paused() {
		t.Fatal("ledger should be paused")
	}
}

func TestGraphLedger_Resume(t *testing.T) {
	g := NewGraphLedger(config.Default(), nil)
	trip := []proxy.Frame{
		toolsCall("a", `{}`),
		toolsCall("b", `{}`),
		toolsCall("a", `{}`),
		toolsCall("b", `{}`),
	}
	for _, f := range trip {
		_ = g.OnClientFrame(f)
	}
	if !g.Paused() {
		t.Fatal("expected paused")
	}
	g.Resume()
	if g.Paused() {
		t.Fatal("expected resumed")
	}
	if d := g.OnClientFrame(toolsCall("c", `{}`)); !d.Forward {
		t.Fatal("after resume should forward")
	}
}

func TestGraphLedger_DifferentArgsNoTrip(t *testing.T) {
	g := NewGraphLedger(config.Default(), nil)
	for i := 0; i < 4; i++ {
		args, _ := json.Marshal(map[string]int{"n": i})
		f := toolsCall("write_file", string(args))
		if d := g.OnClientFrame(f); !d.Forward {
			t.Fatalf("call %d should forward", i)
		}
	}
}

func toolsCall(tool, args string) proxy.Frame {
	params, _ := json.Marshal(protocol.ToolsCallParams{
		Name:      tool,
		Arguments: json.RawMessage(args),
	})
	return proxy.Frame{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodToolsCall,
		Params:  params,
	}
}

func TestDetectGraphLoop(t *testing.T) {
	keys := []TurnKey{"a:h1", "b:h2", "a:h1", "b:h2"}
	pattern, trip := detectGraphLoop(keys)
	if !trip {
		t.Fatal("expected trip")
	}
	if pattern == "" {
		t.Fatal("expected pattern")
	}
}
