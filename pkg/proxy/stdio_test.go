package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestSessionBidirectionalInitialize(t *testing.T) {
	clientInReader, clientInWriter := io.Pipe()
	clientOutReader, clientOutWriter := io.Pipe()
	serverInReader, serverInWriter := io.Pipe()
	serverOutReader, serverOutWriter := io.Pipe()

	go func() {
		defer serverInReader.Close()
		defer serverOutWriter.Close()
		reader := NewFrameReader(serverInReader, nil)
		frame, err := reader.Read()
		if err != nil {
			return
		}
		if frame.Method != "initialize" {
			t.Errorf("server got method %q", frame.Method)
			return
		}
		resp := Frame{
			JSONRPC: "2.0",
			ID:      frame.ID,
			Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"fake","version":"0.0.1"}}`),
		}
		_ = WriteFrame(serverOutWriter, resp)
	}()

	session := &Session{
		ClientIn:      clientInReader,
		ClientOut:     clientOutWriter,
		ServerIn:      serverInWriter,
		ServerOut:     serverOutReader,
		Intercept:     PassThroughInterceptor{},
		CloseServerIn: func() error { return serverInWriter.Close() },
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(context.Background()) }()

	req := Frame{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize", Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}`)}
	if err := WriteFrame(clientInWriter, req); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	reader := NewFrameReader(clientOutReader, nil)
	resp, err := reader.Read()
	if err != nil {
		t.Fatalf("Read response: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}

	_ = clientInWriter.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session did not stop after client EOF")
	}
}

func TestSessionChildStderrNotOnClientStdout(t *testing.T) {
	// When using RunWrap with a command that writes stderr, stdout stays clean.
	// Here we simulate by ensuring proxy only writes JSON to clientOut.
	clientInReader, clientInWriter := io.Pipe()
	var clientOut bytes.Buffer
	serverInReader, serverInWriter := io.Pipe()
	serverOutReader, serverOutWriter := io.Pipe()

	go func() {
		defer serverInReader.Close()
		defer serverOutWriter.Close()
		reader := NewFrameReader(serverInReader, nil)
		_, _ = reader.Read()
		resp := Frame{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: json.RawMessage(`{"tools":[]}`)}
		_ = WriteFrame(serverOutWriter, resp)
	}()

	session := &Session{
		ClientIn:      clientInReader,
		ClientOut:     &clientOut,
		ServerIn:      serverInWriter,
		ServerOut:     serverOutReader,
		Intercept:     PassThroughInterceptor{},
		CloseServerIn: func() error { return serverInWriter.Close() },
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(context.Background()) }()

	req := Frame{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list"}
	_ = WriteFrame(clientInWriter, req)
	time.Sleep(100 * time.Millisecond)
	_ = clientInWriter.Close()
	<-done

	out := clientOut.String()
	if !strings.HasPrefix(out, "{") {
		t.Fatalf("client stdout must be JSON only, got %q", out)
	}
}
