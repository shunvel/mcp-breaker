package breaker_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shunvel/mcp-breaker/pkg/breaker"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

func TestSessionEchoBlockDoesNotReachServer(t *testing.T) {
	clientInReader, clientInWriter := io.Pipe()
	clientOutReader, clientOutWriter := io.Pipe()
	serverInReader, serverInWriter := io.Pipe()
	serverOutReader, serverOutWriter := io.Pipe()

	var serverCalls atomic.Int32
	go func() {
		defer serverInReader.Close()
		defer serverOutWriter.Close()
		reader := proxy.NewFrameReader(serverInReader, nil)
		for {
			frame, err := reader.Read()
			if err != nil {
				return
			}
			serverCalls.Add(1)
			resp := proxy.Frame{
				JSONRPC: "2.0",
				ID:      frame.ID,
				Result:  json.RawMessage(`{"content":[{"type":"text","text":"ok"}],"isError":false}`),
			}
			_ = proxy.WriteFrame(serverOutWriter, resp)
		}
	}()

	detector := breaker.NewEchoDetector()
	session := &proxy.Session{
		ClientIn:      clientInReader,
		ClientOut:     clientOutWriter,
		ServerIn:      serverInWriter,
		ServerOut:     serverOutReader,
		Intercept:     detector,
		CloseServerIn: func() error { return serverInWriter.Close() },
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(context.Background()) }()

	args := json.RawMessage(`{"path":"/tmp/a.txt","content":"same"}`)
	for i := 1; i <= 3; i++ {
		params, _ := json.Marshal(map[string]any{
			"name":      "write_file",
			"arguments": json.RawMessage(args),
		})
		req := proxy.Frame{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
		if err := proxy.WriteFrame(clientInWriter, req); err != nil {
			t.Fatalf("WriteFrame %d: %v", i, err)
		}

		reader := proxy.NewFrameReader(clientOutReader, nil)
		resp, err := reader.Read()
		if err != nil {
			t.Fatalf("Read %d: %v", i, err)
		}
		if i < 3 && resp.Result == nil {
			t.Fatalf("call %d expected forwarded result", i)
		}
		if i == 3 {
			if !strings.Contains(string(resp.Result), "identical failures") {
				t.Fatalf("call 3 expected intervention, got %s", resp.Result)
			}
		}
	}

	if got := serverCalls.Load(); got != 2 {
		t.Fatalf("server should receive 2 calls, got %d", got)
	}

	_ = clientInWriter.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session timeout")
	}
}
