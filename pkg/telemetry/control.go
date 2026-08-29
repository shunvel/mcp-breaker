package telemetry

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// ControlServer handles resume commands from the dashboard.
type ControlServer struct {
	socket    string
	sessionID string
	onResume  func(sessionID string)
	ln        net.Listener
	stop      chan struct{}
	wg        sync.WaitGroup
}

// NewControlServer creates a control socket listener.
func NewControlServer(socketPath, sessionID string, onResume func(string)) *ControlServer {
	return &ControlServer{
		socket:    socketPath,
		sessionID: sessionID,
		onResume:  onResume,
		stop:      make(chan struct{}),
	}
}

// Start begins listening for control commands.
func (s *ControlServer) Start() error {
	if err := os.MkdirAll(filepath.Dir(s.socket), 0o755); err != nil {
		return err
	}
	_ = os.Remove(s.socket)
	ln, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	s.ln = ln
	s.wg.Add(1)
	go s.serve()
	return nil
}

func (s *ControlServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
				continue
			}
		}
		go func(c net.Conn) {
			defer c.Close()
			dec := json.NewDecoder(c)
			for dec.More() {
				var cmd ControlCommand
				if err := dec.Decode(&cmd); err != nil {
					return
				}
				if cmd.Action == "resume" && (cmd.SessionID == "" || cmd.SessionID == s.sessionID) {
					if s.onResume != nil {
						s.onResume(cmd.SessionID)
					}
				}
			}
		}(conn)
	}
}

// Stop shuts down the control server.
func (s *ControlServer) Stop() {
	close(s.stop)
	if s.ln != nil {
		_ = s.ln.Close()
	}
	s.wg.Wait()
	_ = os.Remove(s.socket)
}

// SendResume sends a resume command to the control socket.
func SendResume(socketPath, sessionID string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	cmd := ControlCommand{Action: "resume", SessionID: sessionID}
	return json.NewEncoder(conn).Encode(cmd)
}
