package embed

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	modelFile      = "model.onnx"
	tokenizerFile  = "tokenizer.json"
	modelURL       = "https://huggingface.co/onnx-models/all-MiniLM-L6-v2-onnx/resolve/main/model.onnx"
	tokenizerURL   = "https://huggingface.co/onnx-models/all-MiniLM-L6-v2-onnx/resolve/main/tokenizer.json"
)

// ModelStatus reports whether cached model artifacts exist.
type ModelStatus struct {
	Dir           string
	ModelPath     string
	TokenizerPath string
	ModelReady    bool
	TokenizerReady bool
	Ready         bool
}

// Status returns model cache status for dir.
func Status(dir string) ModelStatus {
	st := ModelStatus{
		Dir:           dir,
		ModelPath:     filepath.Join(dir, modelFile),
		TokenizerPath: filepath.Join(dir, tokenizerFile),
	}
	if _, err := os.Stat(st.ModelPath); err == nil {
		st.ModelReady = true
	}
	if _, err := os.Stat(st.TokenizerPath); err == nil {
		st.TokenizerReady = true
	}
	st.Ready = st.ModelReady && st.TokenizerReady
	return st
}

// Download fetches model artifacts into dir.
func Download(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := downloadFile(modelURL, filepath.Join(dir, modelFile)); err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	if err := downloadFile(tokenizerURL, filepath.Join(dir, tokenizerFile)); err != nil {
		return fmt.Errorf("download tokenizer: %w", err)
	}
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
