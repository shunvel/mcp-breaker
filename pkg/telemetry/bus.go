package telemetry

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Bus publishes telemetry events to local subscribers and a Unix socket.
type Bus struct {
	sessionID string
	socket    string
	mu        sync.RWMutex
	metrics   Metrics
	listeners []chan Event
	server    net.Listener
	stop      chan struct{}
}

// NewBus creates a telemetry bus for the given session.
func NewBus(sessionID, eventSocket string) *Bus {
	return &Bus{
		sessionID: sessionID,
		socket:    eventSocket,
		stop:      make(chan struct{}),
	}
}

// Subscribe returns a channel that receives published events.
func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.listeners = append(b.listeners, ch)
	b.mu.Unlock()
	return ch
}

// Metrics returns a snapshot of current counters.
func (b *Bus) Metrics() Metrics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.metrics
}

// Publish emits an event to subscribers and the socket.
func (b *Bus) Publish(ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	if ev.SessionID == "" {
		ev.SessionID = b.sessionID
	}

	b.mu.Lock()
	switch ev.Type {
	case EventFrame:
		b.metrics.TotalFrames++
		if ev.Method == "tools/call" {
			b.metrics.ToolsCallCount++
		}
	case EventEchoTrip:
		b.metrics.EchoTrips++
		b.metrics.TokensSaved += 500
	case EventSemanticTrip:
		b.metrics.SemanticTrips++
		b.metrics.TokensSaved += 500
	case EventGraphTrip:
		b.metrics.GraphTrips++
		b.metrics.TokensSaved += 500
	}
	ev.Metrics = b.metrics
	b.mu.Unlock()

	b.mu.RLock()
	listeners := append([]chan Event(nil), b.listeners...)
	b.mu.RUnlock()
	for _, ch := range listeners {
		select {
		case ch <- ev:
		default:
		}
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_ = writeToSocket(b.socket, data)
}

func writeToSocket(socketPath string, data []byte) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write(data)
	return err
}

// StartEventServer listens on a Unix socket and accepts publisher connections.
func StartEventServer(socketPath string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, err
	}
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return ln, nil
}

// ServeEvents accepts connections and broadcasts incoming NDJSON events to subscribers.
func ServeEvents(ln net.Listener, out chan<- Event, stop <-chan struct{}) {
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					continue
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				dec := json.NewDecoder(c)
				for dec.More() {
					var ev Event
					if err := dec.Decode(&ev); err != nil {
						return
					}
					select {
					case out <- ev:
					default:
					}
				}
			}(conn)
		}
	}()
}

// DialEvents connects to the event socket as a subscriber fan-in writer.
func DialEvents(socketPath string) (net.Conn, error) {
	return net.Dial("unix", socketPath)
}
