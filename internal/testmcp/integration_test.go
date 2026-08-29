package testmcp_test

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shunvel/mcp-breaker/pkg/breaker"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

func TestIntegrationWrapBlocksThirdIdenticalToolsCall(t *testing.T) {
	fakeMCP := buildFakeMCPBinary(t)
	proxyBin := buildProxyBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, proxyBin, "wrap", "--", fakeMCP)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()

	writeFrame(t, stdin, proxy.Frame{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}`),
	})
	readFrame(t, stdout)

	args := json.RawMessage(`{"path":"/tmp/x","content":"same payload"}`)
	var forwarded int
	for i := 1; i <= 3; i++ {
		params, _ := json.Marshal(map[string]any{"name": "write_file", "arguments": json.RawMessage(args)})
		writeFrame(t, stdin, proxy.Frame{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`2`),
			Method:  "tools/call",
			Params:  params,
		})
		resp := readFrame(t, stdout)
		if strings.Contains(string(resp.Result), `"text":"ok"`) {
			forwarded++
		}
		if i == 3 {
			if !strings.Contains(string(resp.Result), "identical failures") {
				t.Fatalf("call 3 expected intervention, got %s", resp.Result)
			}
		}
	}

	if forwarded != 2 {
		t.Fatalf("expected 2 forwarded tool results, got %d", forwarded)
	}
}

func TestIntegrationEchoDetectorDirectWithFakeChild(t *testing.T) {
	clientInReader, clientInWriter := io.Pipe()
	clientOutReader, clientOutWriter := io.Pipe()

	fakeMCP := buildFakeMCPBinary(t)
	child := exec.Command(fakeMCP)
	childIn, _ := child.StdinPipe()
	childOut, _ := child.StdoutPipe()
	child.Stderr = io.Discard
	if err := child.Start(); err != nil {
		t.Fatalf("child Start: %v", err)
	}
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()

	session := &proxy.Session{
		ClientIn:      clientInReader,
		ClientOut:     clientOutWriter,
		ServerIn:      childIn,
		ServerOut:     childOut,
		Intercept:     breaker.NewEchoDetector(),
		CloseServerIn: childIn.Close,
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(context.Background()) }()

	writeFrame(t, clientInWriter, proxy.Frame{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}`),
	})
	readFrame(t, clientOutReader)

	args := json.RawMessage(`{"content":"repeat"}`)
	var forwarded int
	for i := 1; i <= 3; i++ {
		params, _ := json.Marshal(map[string]any{"name": "write_file", "arguments": json.RawMessage(args)})
		writeFrame(t, clientInWriter, proxy.Frame{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`2`),
			Method:  "tools/call",
			Params:  params,
		})
		resp := readFrame(t, clientOutReader)
		if strings.Contains(string(resp.Result), `"text":"ok"`) {
			forwarded++
		}
	}

	if forwarded != 2 {
		t.Fatalf("expected 2 forwarded tool results, got %d", forwarded)
	}

	_ = clientInWriter.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session timeout")
	}
}

func buildFakeMCPBinary(t *testing.T) string {
	t.Helper()
	root := moduleRoot(t)
	out := filepath.Join(t.TempDir(), "fakemcp")
	cmd := exec.Command(goBinary(), "build", "-o", out, "./internal/testmcp/cmd/fakemcp")
	cmd.Dir = root
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fakemcp: %v\n%s", err, outBytes)
	}
	return out
}

func buildProxyBinary(t *testing.T) string {
	t.Helper()
	root := moduleRoot(t)
	out := filepath.Join(t.TempDir(), "mcp-breaker")
	cmd := exec.Command(goBinary(), "build", "-o", out, "./cmd/mcp-breaker")
	cmd.Dir = root
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build proxy: %v\n%s", err, outBytes)
	}
	return out
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command(goBinary(), "list", "-m", "-f", "{{.Dir}}")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("module root: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func goBinary() string {
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	return "go"
}

func writeFrame(t *testing.T, w io.Writer, frame proxy.Frame) {
	t.Helper()
	if err := proxy.WriteFrame(w, frame); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
}

func readFrame(t *testing.T, r io.Reader) proxy.Frame {
	t.Helper()
	reader := proxy.NewFrameReader(r, nil)
	frame, err := reader.Read()
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	return frame
}
