package embed

import (
	"os"

	"github.com/shunvel/mcp-breaker/pkg/config"
)

// NewEmbedder returns the best available embedder for the environment.
func NewEmbedder(cfg config.Config) Embedder {
	if os.Getenv("MCP_BREAKER_MOCK_EMBED") == "1" {
		return NewMockEmbedder()
	}
	e, err := NewFromConfig(cfg)
	if err != nil || e == nil {
		return nil
	}
	if e.Ready() {
		return e
	}
	return nil
}
