package breaker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shunvel/mcp-breaker/pkg/protocol"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

func toolsCallFrame(id int, tool string, args json.RawMessage) proxy.Frame {
	params, _ := json.Marshal(protocol.ToolsCallParams{
		Name:      tool,
		Arguments: args,
	})
	return proxy.Frame{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodToolsCall,
		Params:  params,
	}
}

func TestEchoDetectorTestCaseA(t *testing.T) {
	d := NewEchoDetector()
	args := json.RawMessage(`{"path":"/tmp/a.txt","content":"identical content"}`)

	for i := 1; i <= 3; i++ {
		decision := d.OnClientFrame(toolsCallFrame(i, "write_file", args))
		switch i {
		case 1, 2:
			if !decision.Forward {
				t.Fatalf("call %d should forward", i)
			}
		case 3:
			if decision.Forward {
				t.Fatal("call 3 should block")
			}
			if decision.Response == nil {
				t.Fatal("call 3 should include synthetic response")
			}
			if !strings.Contains(string(decision.Response.Result), "write_file") {
				t.Fatalf("unexpected result: %s", decision.Response.Result)
			}
			if !strings.Contains(string(decision.Response.Result), "identical failures") {
				t.Fatalf("unexpected result: %s", decision.Response.Result)
			}
		}
	}
}

func TestEchoDetectorDifferentContentNeverTrips(t *testing.T) {
	d := NewEchoDetector()
	for i := 0; i < 5; i++ {
		args, _ := json.Marshal(map[string]string{"content": "value-" + string(rune('a'+i))})
		decision := d.OnClientFrame(toolsCallFrame(i, "write_file", args))
		if !decision.Forward {
			t.Fatalf("call %d should forward", i)
		}
	}
}

func TestEchoDetectorIndependentTools(t *testing.T) {
	d := NewEchoDetector()
	argsA := json.RawMessage(`{"cmd":"npm test"}`)
	argsB := json.RawMessage(`{"cmd":"npm run build"}`)

	for i := 0; i < 2; i++ {
		if d := d.OnClientFrame(toolsCallFrame(i, "execute_command", argsA)); !d.Forward {
			t.Fatalf("tool A call %d should forward", i+1)
		}
	}
	for i := 0; i < 2; i++ {
		if d := d.OnClientFrame(toolsCallFrame(i, "execute_command", argsB)); !d.Forward {
			t.Fatalf("tool B call %d should forward", i+1)
		}
	}
}

func TestEchoDetectorRingDoesNotFalseTripAfterDifferentArg(t *testing.T) {
	d := NewEchoDetector()
	same := json.RawMessage(`{"content":"loop"}`)
	diff := json.RawMessage(`{"content":"break"}`)

	d.OnClientFrame(toolsCallFrame(1, "write_file", same))
	d.OnClientFrame(toolsCallFrame(2, "write_file", same))
	d.OnClientFrame(toolsCallFrame(3, "write_file", diff))
	decision := d.OnClientFrame(toolsCallFrame(4, "write_file", same))
	if !decision.Forward {
		t.Fatal("should not trip after different argument reset consecutive count")
	}
	if d.ConsecutiveCount("write_file") != 1 {
		t.Fatalf("consecutive count: got %d want 1", d.ConsecutiveCount("write_file"))
	}
}

func TestEchoDetectorRingSize(t *testing.T) {
	d := NewEchoDetector()
	for i := 0; i < 7; i++ {
		args, _ := json.Marshal(map[string]int{"n": i})
		d.OnClientFrame(toolsCallFrame(i, "write_file", args))
	}
	ring := d.RingHashes("write_file")
	if len(ring) != echoRingSize {
		t.Fatalf("ring size: got %d want %d", len(ring), echoRingSize)
	}
}

func TestEchoDetectorIgnoresNonToolsCall(t *testing.T) {
	d := NewEchoDetector()
	frame := proxy.Frame{JSONRPC: "2.0", Method: "initialize", Params: json.RawMessage(`{}`)}
	decision := d.OnClientFrame(frame)
	if !decision.Forward {
		t.Fatal("initialize should always forward")
	}
}
