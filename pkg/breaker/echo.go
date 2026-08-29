package breaker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/shunvel/mcp-breaker/pkg/protocol"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

const (
	echoRingSize       = 5
	echoTripThreshold  = 3
)

type toolState struct {
	lastHash         string
	consecutiveCount int
	ring             []string
}

// EchoDetector blocks repeated identical tools/call argument payloads.
type EchoDetector struct {
	tools map[string]*toolState
	bus   *telemetry.Bus
}

// NewEchoDetector creates an echo detector with per-tool hash tracking.
func NewEchoDetector() *EchoDetector {
	return &EchoDetector{tools: make(map[string]*toolState)}
}

// NewEchoDetectorWithBus creates an echo detector that publishes trip events.
func NewEchoDetectorWithBus(bus *telemetry.Bus) *EchoDetector {
	return &EchoDetector{tools: make(map[string]*toolState), bus: bus}
}

// OnClientFrame implements proxy.Interceptor.
func (d *EchoDetector) OnClientFrame(frame proxy.Frame) proxy.Decision {
	if !frame.IsToolsCall() {
		return proxy.Decision{Forward: true}
	}

	var params protocol.ToolsCallParams
	if err := json.Unmarshal(frame.Params, &params); err != nil || params.Name == "" {
		return proxy.Decision{Forward: true}
	}

	hash := hashToolCall(params.Name, params.Arguments)
	state := d.tools[params.Name]
	if state == nil {
		state = &toolState{}
		d.tools[params.Name] = state
	}

	d.pushRing(state, hash)

	if hash == state.lastHash {
		state.consecutiveCount++
	} else {
		state.lastHash = hash
		state.consecutiveCount = 1
	}

	if state.consecutiveCount < echoTripThreshold {
		return proxy.Decision{Forward: true}
	}

	if d.bus != nil {
		d.bus.Publish(telemetry.Event{
			Type:    telemetry.EventEchoTrip,
			Tool:    params.Name,
			Message: protocol.EchoInterventionMessage(params.Name),
		})
	}

	result, err := json.Marshal(protocol.BuildEchoInterventionResult(params.Name))
	if err != nil {
		return proxy.Decision{Forward: true}
	}

	response := proxy.Frame{
		JSONRPC: "2.0",
		ID:      frame.ID,
		Result:  result,
	}
	return proxy.Decision{Forward: false, Response: &response}
}

// OnServerFrame implements proxy.Interceptor.
func (d *EchoDetector) OnServerFrame(frame proxy.Frame) proxy.Frame {
	return frame
}

func (d *EchoDetector) pushRing(state *toolState, hash string) {
	state.ring = append(state.ring, hash)
	if len(state.ring) > echoRingSize {
		state.ring = state.ring[len(state.ring)-echoRingSize:]
	}
}

func hashToolCall(toolName string, arguments json.RawMessage) string {
	if len(arguments) == 0 {
		arguments = json.RawMessage("{}")
	}
	sum := sha256.Sum256(append([]byte(toolName+":"), arguments...))
	return hex.EncodeToString(sum[:])
}

// ConsecutiveCount returns the current consecutive identical count for a tool (testing aid).
func (d *EchoDetector) ConsecutiveCount(toolName string) int {
	state := d.tools[toolName]
	if state == nil {
		return 0
	}
	return state.consecutiveCount
}

// RingHashes returns the tracked hash ring for a tool (testing aid).
func (d *EchoDetector) RingHashes(toolName string) []string {
	state := d.tools[toolName]
	if state == nil {
		return nil
	}
	out := make([]string, len(state.ring))
	copy(out, state.ring)
	return out
}

// HashToolCall exposes the hash function for tests.
func HashToolCall(toolName string, arguments json.RawMessage) string {
	return hashToolCall(toolName, arguments)
}

// EchoDetectorDescription documents the detector for diagnostics.
func EchoDetectorDescription() string {
	return fmt.Sprintf(
		"echo detector: trips after %d consecutive identical tools/call payloads per tool (ring size %d)",
		echoTripThreshold,
		echoRingSize,
	)
}
