package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
)

const (
	// MaxFrameBytes is the hard cap on a single JSON-RPC frame size.
	MaxFrameBytes = 1 << 20 // 1 MiB
)

// Frame is a JSON-RPC 2.0 message on the MCP stdio wire.
type Frame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// IsRequest reports whether the frame is a client-originated JSON-RPC request.
func (f Frame) IsRequest() bool {
	return f.Method != ""
}

// IsToolsCall reports whether the frame is an MCP tools/call request.
func (f Frame) IsToolsCall() bool {
	return f.Method == "tools/call"
}

// Decision is the outcome of intercepting a client-originated frame.
type Decision struct {
	Forward  bool
	Response *Frame
}

// Interceptor can observe and mutate proxied JSON-RPC frames.
type Interceptor interface {
	OnClientFrame(frame Frame) Decision
	OnServerFrame(frame Frame) Frame
}

// PassThroughInterceptor forwards all frames unchanged.
type PassThroughInterceptor struct{}

func (PassThroughInterceptor) OnClientFrame(frame Frame) Decision {
	return Decision{Forward: true}
}

func (PassThroughInterceptor) OnServerFrame(frame Frame) Frame {
	return frame
}

// FrameReader reads newline-delimited JSON-RPC frames.
type FrameReader struct {
	scanner *bufio.Scanner
	logger  *log.Logger
}

// NewFrameReader creates a reader for MCP stdio framing.
func NewFrameReader(r io.Reader, logger *log.Logger) *FrameReader {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, MaxFrameBytes)
	return &FrameReader{scanner: scanner, logger: logger}
}

// Read reads the next valid frame. Empty lines are skipped.
func (fr *FrameReader) Read() (Frame, error) {
	for fr.scanner.Scan() {
		line := bytes.TrimSpace(fr.scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var frame Frame
		if err := json.Unmarshal(line, &frame); err != nil {
			if fr.logger != nil {
				fr.logger.Printf("proxy: skip malformed frame: %v", err)
			}
			continue
		}
		if frame.JSONRPC == "" {
			frame.JSONRPC = "2.0"
		}
		return frame, nil
	}
	if err := fr.scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return Frame{}, fmt.Errorf("frame exceeds max size (%d bytes)", MaxFrameBytes)
		}
		return Frame{}, err
	}
	return Frame{}, io.EOF
}

// WriteFrame writes a compact JSON-RPC frame terminated by newline.
func WriteFrame(w io.Writer, frame Frame) error {
	if frame.JSONRPC == "" {
		frame.JSONRPC = "2.0"
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

// Session proxies newline-delimited JSON-RPC between a client and server stream.
type Session struct {
	ClientIn      io.Reader
	ClientOut     io.Writer
	ServerIn      io.Writer
	ServerOut     io.Reader
	Logger        *log.Logger
	Intercept     Interceptor
	CloseServerIn func() error
}

// Run starts bidirectional proxying until a stream closes or ctx is cancelled.
func (s *Session) Run(ctx context.Context) error {
	if s.Intercept == nil {
		s.Intercept = PassThroughInterceptor{}
	}
	if s.Logger == nil {
		s.Logger = log.New(io.Discard, "", 0)
	}

	clientReader := NewFrameReader(s.ClientIn, s.Logger)
	serverReader := NewFrameReader(s.ServerOut, s.Logger)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errCh <- s.pumpClientToServer(ctx, clientReader)
	}()
	go func() {
		defer wg.Done()
		errCh <- s.pumpServerToClient(ctx, serverReader)
	}()

	wg.Wait()
	close(errCh)

	var firstErr error
	for err := range errCh {
		if err == nil || errors.Is(err, io.EOF) {
			continue
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Session) pumpClientToServer(ctx context.Context, reader *FrameReader) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frame, err := reader.Read()
		if err != nil {
			if s.CloseServerIn != nil {
				_ = s.CloseServerIn()
			}
			return err
		}

		decision := s.Intercept.OnClientFrame(frame)
		if !decision.Forward {
			if decision.Response == nil {
				return fmt.Errorf("interceptor blocked frame without response")
			}
			if err := WriteFrame(s.ClientOut, *decision.Response); err != nil {
				return err
			}
			continue
		}

		if err := WriteFrame(s.ServerIn, frame); err != nil {
			return err
		}
	}
}

func (s *Session) pumpServerToClient(ctx context.Context, reader *FrameReader) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frame, err := reader.Read()
		if err != nil {
			return err
		}

		out := s.Intercept.OnServerFrame(frame)
		if err := WriteFrame(s.ClientOut, out); err != nil {
			return err
		}
	}
}
