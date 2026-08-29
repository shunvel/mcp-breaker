package breaker

import (
	"encoding/json"
	"testing"

	"github.com/shunvel/mcp-breaker/pkg/config"
)

func TestChain_PriorityEchoBeforeLedger(t *testing.T) {
	echo := NewEchoDetector()
	ledger := NewGraphLedger(config.Default(), nil)
	chain := NewChain(echo, ledger)

	args := json.RawMessage(`{"content":"same"}`)
	for i := 0; i < 3; i++ {
		f := toolsCallFrame(i, "write_file", args)
		d := chain.OnClientFrame(f)
		if i < 2 && !d.Forward {
			t.Fatalf("call %d should forward", i+1)
		}
		if i == 2 && d.Forward {
			t.Fatal("echo should block 3rd identical call before ledger")
		}
	}
}
