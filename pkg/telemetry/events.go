package telemetry

import "time"

// EventType identifies telemetry event kinds.
type EventType string

const (
	EventFrame        EventType = "frame"
	EventEchoTrip     EventType = "echo_trip"
	EventSemanticTrip EventType = "semantic_trip"
	EventGraphTrip    EventType = "graph_trip"
	EventMetrics      EventType = "metrics"
	EventPaused       EventType = "paused"
	EventResumed      EventType = "resumed"
)

// Event is a JSON-serializable telemetry record.
type Event struct {
	Type       EventType `json:"type"`
	SessionID  string    `json:"session_id"`
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	Message    string    `json:"message,omitempty"`
	Pattern    string    `json:"pattern,omitempty"`
	Similarity float64   `json:"similarity,omitempty"`
	Metrics    Metrics   `json:"metrics,omitempty"`
}

// Metrics holds aggregate counters.
type Metrics struct {
	TotalFrames    int `json:"total_frames"`
	ToolsCallCount int `json:"tools_call_count"`
	EchoTrips      int `json:"echo_trips"`
	SemanticTrips  int `json:"semantic_trips"`
	GraphTrips     int `json:"graph_trips"`
	TokensSaved    int `json:"tokens_saved"`
}

// ControlCommand is sent from the dashboard to resume a paused session.
type ControlCommand struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id"`
}
