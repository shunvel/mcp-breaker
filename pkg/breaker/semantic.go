package breaker

import "testing"

// SemanticDetector will evaluate cosine similarity across tool outputs using a
// local ONNX embedding engine (spec section 3.2). It is not implemented yet.
type SemanticDetector struct{}

// NewSemanticDetector creates a placeholder semantic stagnation detector.
func NewSemanticDetector() *SemanticDetector {
	return &SemanticDetector{}
}

// TestSemanticStagnationPlaceholder documents spec Test Case B expectations.
func TestSemanticStagnationPlaceholder(t *testing.T) {
	t.Skip("semantic stagnation detection (ONNX All-MiniLM-L6-v2) is planned for the next increment")
}
