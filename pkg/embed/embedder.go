package embed

import "errors"

// ErrNotAvailable indicates the embedder backend is unavailable.
var ErrNotAvailable = errors.New("embedder not available")

// Embedder converts text into a dense vector representation.
type Embedder interface {
	Ready() bool
	Embed(text string) ([]float32, error)
	Dim() int
}
