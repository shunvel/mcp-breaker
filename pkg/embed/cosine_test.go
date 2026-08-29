package embed

import "testing"

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if CosineSimilarity(a, b) < 0.99 {
		t.Fatal("identical vectors should have similarity ~1")
	}
	c := []float32{0, 1, 0}
	if CosineSimilarity(a, c) > 0.01 {
		t.Fatal("orthogonal vectors should have similarity ~0")
	}
}

func TestMockEmbedderPort8080Similarity(t *testing.T) {
	m := NewMockEmbedder()
	texts := []string{
		"Port 8080 bound",
		"Error: listen EADDRINUSE: address already in use 8080",
		"Cannot bind network to 8080",
	}
	var vecs [][]float32
	for _, text := range texts {
		v, err := m.Embed(text)
		if err != nil {
			t.Fatal(err)
		}
		vecs = append(vecs, v)
	}
	sim := CosineSimilarity(vecs[2], vecs[0])
	if sim < 0.88 {
		t.Fatalf("expected similar paraphrases >= 0.88, got %f", sim)
	}
}
