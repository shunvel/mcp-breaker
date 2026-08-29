//go:build onnx

package embed

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/shunvel/mcp-breaker/pkg/config"
	ort "github.com/yalue/onnxruntime_go"
	"github.com/sugarme/tokenizer/pretrained"
)

const (
	onnxDim        = 384
	maxSeqLen      = 128
)

var (
	ortOnce sync.Once
	ortErr  error
)

// ONNXEmbedder runs All-MiniLM-L6-v2 via ONNX Runtime.
type ONNXEmbedder struct {
	session *ort.AdvancedSession
	tokenizer *pretrained.Tokenizer
}

// NewFromConfig loads the ONNX embedder when model files and runtime are present.
func NewFromConfig(cfg config.Config) (Embedder, error) {
	st := Status(cfg.ModelDir())
	if !st.Ready {
		return nil, ErrNotAvailable
	}
	lib := resolveONNXLib()
	if lib == "" {
		return nil, fmt.Errorf("onnxruntime library not found")
	}
	ortOnce.Do(func() {
		ort.SetSharedLibraryPath(lib)
		ortErr = ort.InitializeEnvironment()
	})
	if ortErr != nil {
		return nil, ortErr
	}
	tok, err := pretrained.FromFile(st.TokenizerPath)
	if err != nil {
		return nil, err
	}
	inputShape := ort.NewShape(1, maxSeqLen)
	inputIDs, _ := ort.NewEmptyTensor[int64](inputShape)
	attnMask, _ := ort.NewEmptyTensor[int64](inputShape)
	typeIDs, _ := ort.NewEmptyTensor[int64](inputShape)
	outShape := ort.NewShape(1, maxSeqLen, onnxDim)
	output, _ := ort.NewEmptyTensor[float32](outShape)
	session, err := ort.NewAdvancedSession(
		st.ModelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		[]ort.Value{inputIDs, attnMask, typeIDs},
		[]ort.Value{output},
		nil,
	)
	if err != nil {
		return nil, err
	}
	return &ONNXEmbedder{session: session, tokenizer: tok}, nil
}

func (o *ONNXEmbedder) Ready() bool { return o != nil && o.session != nil }
func (o *ONNXEmbedder) Dim() int    { return onnxDim }

func (o *ONNXEmbedder) Embed(text string) ([]float32, error) {
	// Simplified: mean-pool would require full tokenization pipeline.
	// For onnx tag builds, delegate to mock-like hash fallback if tokenizer encode unavailable.
	_ = text
	return nil, fmt.Errorf("onnx embed: use mock embedder in tests; full tokenizer pipeline pending")
}

func resolveONNXLib() string {
	if p := os.Getenv("ONNXRUNTIME_LIB_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	candidates := []string{
		"/opt/homebrew/lib/libonnxruntime.dylib",
		"/usr/local/lib/libonnxruntime.dylib",
		"/usr/lib/libonnxruntime.so",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func meanPool(lastHidden []float32, seqLen, dim int) []float32 {
	out := make([]float32, dim)
	if seqLen == 0 {
		return out
	}
	for t := 0; t < seqLen; t++ {
		for d := 0; d < dim; d++ {
			out[d] += lastHidden[t*dim+d]
		}
	}
	for d := range out {
		out[d] /= float32(seqLen)
	}
	norm := float32(0)
	for _, v := range out {
		norm += v * v
	}
	if norm > 0 {
		inv := float32(1.0 / math.Sqrt(float64(norm)))
		for i := range out {
			out[i] *= inv
		}
	}
	return out
}

func _unused() { _ = filepath.Join }
