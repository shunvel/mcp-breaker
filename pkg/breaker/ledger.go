package breaker

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/shunvel/mcp-breaker/pkg/config"
	"github.com/shunvel/mcp-breaker/pkg/protocol"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

// TurnKey identifies a tool invocation in the ledger.
type TurnKey string

// GraphLedger detects A-B-A-B tool invocation loops in a sliding window.
type GraphLedger struct {
	window    int
	bus       *telemetry.Bus
	sessionID string
	mu        sync.Mutex
	turns     []TurnKey
	paused    bool
	pattern   string
}

// NewGraphLedger creates a graph loop detector.
func NewGraphLedger(cfg config.Config, bus *telemetry.Bus) *GraphLedger {
	w := cfg.LedgerWindow
	if w <= 0 {
		w = config.DefaultLedgerWindow
	}
	return &GraphLedger{
		window:    w,
		bus:       bus,
		sessionID: cfg.SessionID,
		turns:     make([]TurnKey, 0, w),
	}
}

// OnClientFrame implements proxy.Interceptor.
func (g *GraphLedger) OnClientFrame(frame proxy.Frame) proxy.Decision {
	if !frame.IsToolsCall() {
		return proxy.Decision{Forward: true}
	}

	g.mu.Lock()
	if g.paused {
		pattern := g.pattern
		g.mu.Unlock()
		return g.blockFrame(frame, pattern)
	}

	var params protocol.ToolsCallParams
	if err := json.Unmarshal(frame.Params, &params); err != nil || params.Name == "" {
		g.mu.Unlock()
		return proxy.Decision{Forward: true}
	}
	key := TurnKey(params.Name + ":" + hashToolCall(params.Name, params.Arguments))
	g.turns = append(g.turns, key)
	if len(g.turns) > g.window {
		g.turns = g.turns[len(g.turns)-g.window:]
	}

	pattern, trip := detectGraphLoop(g.turns)
	if trip {
		g.paused = true
		g.pattern = pattern
		g.mu.Unlock()
		if g.bus != nil {
			g.bus.Publish(telemetry.Event{
				Type:    telemetry.EventGraphTrip,
				Pattern: pattern,
				Tool:    params.Name,
				Message: protocol.GraphLoopInterventionMessage(pattern),
			})
			g.bus.Publish(telemetry.Event{Type: telemetry.EventPaused, Pattern: pattern})
		}
		return g.blockFrame(frame, pattern)
	}
	g.mu.Unlock()
	return proxy.Decision{Forward: true}
}

func (g *GraphLedger) blockFrame(frame proxy.Frame, pattern string) proxy.Decision {
	result, err := json.Marshal(protocol.BuildGraphLoopInterventionResult(pattern))
	if err != nil {
		return proxy.Decision{Forward: true}
	}
	return proxy.Decision{
		Forward: false,
		Response: &proxy.Frame{
			JSONRPC: "2.0",
			ID:      frame.ID,
			Result:  result,
		},
	}
}

// OnServerFrame implements proxy.Interceptor.
func (g *GraphLedger) OnServerFrame(frame proxy.Frame) proxy.Frame {
	return frame
}

// Resume clears the pause flag (dashboard control).
func (g *GraphLedger) Resume() {
	g.mu.Lock()
	g.paused = false
	g.pattern = ""
	g.mu.Unlock()
	if g.bus != nil {
		g.bus.Publish(telemetry.Event{Type: telemetry.EventResumed, SessionID: g.sessionID})
	}
}

// Paused reports whether the ledger is blocking forwards.
func (g *GraphLedger) Paused() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.paused
}

func detectGraphLoop(turns []TurnKey) (pattern string, trip bool) {
	n := len(turns)
	if n < 4 {
		return "", false
	}
	// Period-2 at tail: A,B,A,B
	A, B, C, D := turns[n-4], turns[n-3], turns[n-2], turns[n-1]
	if A == C && B == D && A != B {
		return formatPattern([]TurnKey{A, B, A, B}), true
	}
	// Extended: A,B,A,B,A,B
	if n >= 6 {
		a, b, c, d, e, f := turns[n-6], turns[n-5], turns[n-4], turns[n-3], turns[n-2], turns[n-1]
		if a == c && c == e && b == d && d == f && a != b {
			return formatPattern([]TurnKey{a, b, a, b, a, b}), true
		}
	}
	return "", false
}

func formatPattern(keys []TurnKey) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = toolLabel(string(k))
	}
	return strings.Join(parts, " → ")
}

func toolLabel(key string) string {
	if idx := strings.Index(key, ":"); idx >= 0 {
		return key[:idx]
	}
	return key
}

// ResumeFunc returns a callback for the control server.
func (g *GraphLedger) ResumeFunc() func(string) {
	return func(_ string) { g.Resume() }
}

// GraphLedgerDescription documents the ledger module.
func GraphLedgerDescription() string {
	return fmt.Sprintf("graph ledger: detects A-B-A-B loops in sliding window (size %d)", config.DefaultLedgerWindow)
}
