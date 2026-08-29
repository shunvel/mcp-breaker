package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"
)

func TestWriteFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		frame Frame
	}{
		{
			name: "request",
			frame: Frame{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Method:  "initialize",
				Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
			},
		},
		{
			name: "response",
			frame: Frame{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-1"`),
				Result:  json.RawMessage(`{"ok":true}`),
			},
		},
		{
			name: "notification",
			frame: Frame{
				JSONRPC: "2.0",
				Method:  "notifications/initialized",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteFrame(&buf, tt.frame); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}

			reader := NewFrameReader(&buf, nil)
			got, err := reader.Read()
			if err != nil {
				t.Fatalf("Read: %v", err)
			}

			if got.Method != tt.frame.Method {
				t.Fatalf("method: got %q want %q", got.Method, tt.frame.Method)
			}
			if string(got.ID) != string(tt.frame.ID) {
				t.Fatalf("id: got %s want %s", got.ID, tt.frame.ID)
			}
		})
	}
}

func TestReadFramesSplitDelivery(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	first := payload[:len(payload)/2]
	second := payload[len(payload)/2:]

	r := io.MultiReader(strings.NewReader(first), strings.NewReader(second))
	reader := NewFrameReader(r, nil)

	frame, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if frame.Method != "ping" {
		t.Fatalf("method: got %q", frame.Method)
	}
}

func TestReadFramesMultipleInBuffer(t *testing.T) {
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n",
	)
	reader := NewFrameReader(input, nil)

	first, err := reader.Read()
	if err != nil {
		t.Fatalf("first Read: %v", err)
	}
	if first.Method != "ping" {
		t.Fatalf("first method: %q", first.Method)
	}

	second, err := reader.Read()
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if second.Method != "tools/list" {
		t.Fatalf("second method: %q", second.Method)
	}
}

func TestReadFramesEscapedNewlineInJSON(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file","arguments":{"content":"line1\nline2"}}}` + "\n")
	reader := NewFrameReader(input, nil)

	frame, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !frame.IsToolsCall() {
		t.Fatalf("expected tools/call frame")
	}
}

func TestReadFramesSkipsInvalidJSON(t *testing.T) {
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)
	input := strings.NewReader("not json\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	reader := NewFrameReader(input, logger)

	frame, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if frame.Method != "ping" {
		t.Fatalf("method: %q", frame.Method)
	}
	if !strings.Contains(logBuf.String(), "malformed") {
		t.Fatalf("expected malformed log, got %q", logBuf.String())
	}
}

func TestReadFramesRejectsOversizedFrame(t *testing.T) {
	large := strings.Repeat("a", MaxFrameBytes+1)
	input := strings.NewReader(large + "\n")
	reader := NewFrameReader(input, nil)

	_, err := reader.Read()
	if err == nil {
		t.Fatal("expected error for oversized frame")
	}
	if !strings.Contains(err.Error(), "max size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionNonToolsCallAlwaysForwards(t *testing.T) {
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
		resp := Frame{JSONRPC: "2.0", ID: frame.ID, Result: json.RawMessage(`{"capabilities":{}}`)}
		_ = WriteFrame(serverOutWriter, resp)
	}()

	methods := []string{}
	interceptor := &methodRecordingInterceptor{methods: &methods}
	session := &Session{
		ClientIn:      clientInReader,
		ClientOut:     clientOutWriter,
		ServerIn:      serverInWriter,
		ServerOut:     serverOutReader,
		Intercept:     interceptor,
		CloseServerIn: func() error { return serverInWriter.Close() },
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(context.Background()) }()

	req := Frame{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize", Params: json.RawMessage(`{}`)}
	if err := WriteFrame(clientInWriter, req); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	reader := NewFrameReader(clientOutReader, nil)
	resp, err := reader.Read()
	if err != nil {
		t.Fatalf("response Read: %v", err)
	}
	if resp.Result == nil {
		t.Fatalf("expected result frame")
	}

	_ = clientInWriter.Close()
	<-done

	if len(*interceptor.methods) != 1 || (*interceptor.methods)[0] != "initialize" {
		t.Fatalf("intercepted methods: %v", *interceptor.methods)
	}
}

type methodRecordingInterceptor struct {
	methods *[]string
}

func (m *methodRecordingInterceptor) OnClientFrame(frame Frame) Decision {
	if frame.IsRequest() {
		*m.methods = append(*m.methods, frame.Method)
	}
	return Decision{Forward: true}
}

func (m *methodRecordingInterceptor) OnServerFrame(frame Frame) Frame {
	return frame
}
