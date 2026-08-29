package testmcp

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

func TestSimulatePortErrorRotatesTexts(t *testing.T) {
	ResetToolCallCount()
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	done := make(chan error, 1)
	go func() { done <- RunFakeServerWithIO(inReader, outWriter) }()

	writeToolsCall(t, inWriter, 1, "simulate_port_error", `{"attempt":1}`)
	resp1 := readResultText(t, outReader)
	writeToolsCall(t, inWriter, 2, "simulate_port_error", `{"attempt":2}`)
	resp2 := readResultText(t, outReader)
	writeToolsCall(t, inWriter, 3, "simulate_port_error", `{"attempt":3}`)
	resp3 := readResultText(t, outReader)

	if resp1 == resp2 || resp2 == resp3 {
		t.Fatalf("expected distinct paraphrases, got %q %q %q", resp1, resp2, resp3)
	}
	for _, text := range []string{resp1, resp2, resp3} {
		if !strings.Contains(text, "8080") && !strings.Contains(text, "bind") && !strings.Contains(text, "EADDRINUSE") {
			t.Fatalf("unexpected port error text: %q", text)
		}
	}

	_ = inWriter.Close()
	<-done
}

func writeToolsCall(t *testing.T, w io.Writer, id int, tool, args string) {
	t.Helper()
	params, _ := json.Marshal(map[string]any{
		"name":      tool,
		"arguments": json.RawMessage(args),
	})
	_ = proxy.WriteFrame(w, proxy.Frame{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
		Method:  "tools/call",
		Params:  params,
	})
}

func readResultText(t *testing.T, r io.Reader) string {
	t.Helper()
	frame, err := proxy.NewFrameReader(r, nil).Read()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(frame.Result, &result); err != nil || len(result.Content) == 0 {
		t.Fatalf("parse result: %v %s", err, frame.Result)
	}
	return result.Content[0].Text
}
