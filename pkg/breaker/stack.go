package breaker

import (
	"log"

	"github.com/shunvel/mcp-breaker/pkg/config"
	"github.com/shunvel/mcp-breaker/pkg/embed"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

// Stack wires the full interceptor chain for wrap sessions.
type Stack struct {
	Ledger      *GraphLedger
	Bus         *telemetry.Bus
	interceptor proxy.Interceptor
}

// NewStack builds echo, ledger, and semantic interceptors with telemetry.
func NewStack(cfg config.Config, logger *log.Logger) *Stack {
	bus := telemetry.NewBus(cfg.SessionID, cfg.EventSocket)

	echo := NewEchoDetectorWithBus(bus)
	ledger := NewGraphLedger(cfg, bus)

	interceptors := []proxy.Interceptor{echo, ledger}

	if cfg.SemanticEnabled {
		if emb := embed.NewEmbedder(cfg); emb != nil {
			semantic := NewSemanticDetector(emb, cfg.SemanticThreshold, bus)
			interceptors = append(interceptors, semantic)
			if logger != nil {
				logger.Println("semantic detector: enabled")
			}
		} else if logger != nil {
			logger.Println("semantic detector: disabled (run `mcp-breaker models download`; set MCP_BREAKER_MOCK_EMBED=1 for dev without ONNX)")
		}
	}

	chain := NewChain(interceptors...)
	wrapped := NewTelemetryInterceptor(bus, chain)

	return &Stack{
		Ledger:      ledger,
		Bus:         bus,
		interceptor: wrapped,
	}
}

// Interceptor returns the proxy-facing interceptor.
func (s *Stack) Interceptor() proxy.Interceptor {
	if s == nil || s.interceptor == nil {
		return proxy.PassThroughInterceptor{}
	}
	return s.interceptor
}
