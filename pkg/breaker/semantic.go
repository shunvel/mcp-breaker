package breaker

import (
	"encoding/json"
	"sync"

	"github.com/shunvel/mcp-breaker/pkg/embed"
	"github.com/shunvel/mcp-breaker/pkg/protocol"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

const semanticHistorySize = 5

type semanticEntry struct {
	text   string
	vector []float32
}

// SemanticDetector flags trajectory violations using embedding cosine similarity.
type SemanticDetector struct {
	embedder  embed.Embedder
	threshold float64
	bus       *telemetry.Bus
	mu        sync.Mutex
	history   []semanticEntry
	lastSim   float64
}

// NewSemanticDetector creates a semantic stagnation detector.
func NewSemanticDetector(e embed.Embedder, threshold float64, bus *telemetry.Bus) *SemanticDetector {
	if threshold <= 0 {
		threshold = 0.88
	}
	return &SemanticDetector{embedder: e, threshold: threshold, bus: bus}
}

// OnClientFrame implements proxy.Interceptor.
func (d *SemanticDetector) OnClientFrame(frame proxy.Frame) proxy.Decision {
	return proxy.Decision{Forward: true}
}

// OnServerFrame implements proxy.Interceptor.
func (d *SemanticDetector) OnServerFrame(frame proxy.Frame) proxy.Frame {
	if d.embedder == nil || !d.embedder.Ready() || frame.Result == nil {
		return frame
	}
	text := protocol.ExtractResultText(frame.Result)
	if text == "" {
		return frame
	}
	vec, err := d.embedder.Embed(text)
	if err != nil {
		return frame
	}

	d.mu.Lock()
	d.history = append(d.history, semanticEntry{text: text, vector: vec})
	if len(d.history) > semanticHistorySize {
		d.history = d.history[len(d.history)-semanticHistorySize:]
	}
	n := len(d.history)
	var trip bool
	var sim float64
	if n >= 3 {
		sim = embed.CosineSimilarity(d.history[n-1].vector, d.history[n-3].vector)
		d.lastSim = sim
		if sim >= d.threshold {
			trip = true
		}
	}
	d.mu.Unlock()

	if !trip {
		return frame
	}

	var existing protocol.ToolCallResult
	_ = json.Unmarshal(frame.Result, &existing)
	updated := protocol.BuildSemanticInterventionResult(existing)
	raw, err := protocol.MarshalToolCallResult(updated)
	if err != nil {
		return frame
	}
	out := frame
	out.Result = raw

	if d.bus != nil {
		d.bus.Publish(telemetry.Event{
			Type:       telemetry.EventSemanticTrip,
			Message:    protocol.SemanticInterventionMessage(),
			Similarity: sim,
		})
	}
	return out
}

// LastSimilarity returns the most recent N vs N-2 cosine score (testing aid).
func (d *SemanticDetector) LastSimilarity() float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastSim
}
