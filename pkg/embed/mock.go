package embed

import (
	"hash/fnv"
	"math"
)

const mockDim = 384

// MockEmbedder produces deterministic similar vectors for paraphrases in tests.
type MockEmbedder struct{}

// NewMockEmbedder creates a test embedder.
func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{}
}

func (m *MockEmbedder) Ready() bool { return true }
func (m *MockEmbedder) Dim() int    { return mockDim }

// Embed maps text to a vector; texts sharing semantic bucket (port 8080 errors) are similar.
func (m *MockEmbedder) Embed(text string) ([]float32, error) {
	vec := make([]float32, mockDim)
	bucket := mockBucket(text)
	h := fnv.New64a()
	_, _ = h.Write([]byte(bucket))
	seed := h.Sum64()
	for i := range vec {
		seed = seed*1664525 + 1013904223
		vec[i] = float32(math.Sin(float64(seed%10000)/1000.0)) * 0.1
	}
	// strong shared component per bucket
	for i := 0; i < 32; i++ {
		vec[i] = float32(bucketHash(bucket, i))
	}
	return vec, nil
}

func mockBucket(text string) string {
	lower := text
	if contains(lower, "8080") || contains(lower, "EADDRINUSE") || contains(lower, "bind") {
		return "port8080"
	}
	return "other:" + lower
}

func bucketHash(bucket string, i int) float64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(bucket))
	_, _ = h.Write([]byte{byte(i)})
	return float64(h.Sum64()%1000) / 1000.0
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
