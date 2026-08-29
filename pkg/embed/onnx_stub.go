//go:build !onnx

package embed

import "github.com/shunvel/mcp-breaker/pkg/config"

// NewFromConfig returns an ONNX embedder when built with -tags=onnx, else unavailable.
func NewFromConfig(cfg config.Config) (Embedder, error) {
	st := Status(cfg.ModelDir())
	if !st.Ready {
		return nil, ErrNotAvailable
	}
	return nil, ErrNotAvailable
}
