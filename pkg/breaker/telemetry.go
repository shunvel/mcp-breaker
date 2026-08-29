package breaker

import (
	"github.com/shunvel/mcp-breaker/pkg/proxy"
	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

// TelemetryInterceptor publishes frame and trip events to a telemetry bus.
type TelemetryInterceptor struct {
	bus  *telemetry.Bus
	next proxy.Interceptor
}

// NewTelemetryInterceptor wraps an interceptor with telemetry publishing.
func NewTelemetryInterceptor(bus *telemetry.Bus, next proxy.Interceptor) *TelemetryInterceptor {
	return &TelemetryInterceptor{bus: bus, next: next}
}

// OnClientFrame implements proxy.Interceptor.
func (t *TelemetryInterceptor) OnClientFrame(frame proxy.Frame) proxy.Decision {
	if t.bus != nil && frame.IsRequest() {
		ev := telemetry.Event{Type: telemetry.EventFrame, Method: frame.Method}
		if frame.IsToolsCall() {
			ev.Tool = toolNameFromFrame(frame)
		}
		t.bus.Publish(ev)
	}
	if t.next == nil {
		return proxy.Decision{Forward: true}
	}
	return t.next.OnClientFrame(frame)
}

// OnServerFrame implements proxy.Interceptor.
func (t *TelemetryInterceptor) OnServerFrame(frame proxy.Frame) proxy.Frame {
	if t.next == nil {
		return frame
	}
	return t.next.OnServerFrame(frame)
}

func toolNameFromFrame(frame proxy.Frame) string {
	// best-effort; detailed parsing lives in echo/ledger
	return ""
}
