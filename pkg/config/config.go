package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultSemanticThreshold = 0.88
	DefaultLedgerWindow      = 10
	DefaultTokensPerTrip       = 500
)

// Config holds runtime settings for mcp-breaker.
type Config struct {
	SessionID         string
	RunDir            string
	CacheDir          string
	EventSocket       string
	ControlSocket     string
	SemanticThreshold float64
	SemanticEnabled   bool
	LedgerWindow      int
}

// Default returns configuration with standard paths.
func Default() Config {
	home, _ := os.UserHomeDir()
	runDir := filepath.Join(home, ".mcp-breaker", "run")
	cacheDir := filepath.Join(home, ".cache", "mcp-breaker", "models")
	return Config{
		SessionID:         newSessionID(),
		RunDir:            runDir,
		CacheDir:          cacheDir,
		EventSocket:       filepath.Join(runDir, "events.sock"),
		ControlSocket:     filepath.Join(runDir, "control.sock"),
		SemanticThreshold: DefaultSemanticThreshold,
		SemanticEnabled:   true,
		LedgerWindow:      DefaultLedgerWindow,
	}
}

// ModelDir returns the ONNX model cache directory.
func (c Config) ModelDir() string {
	return c.CacheDir
}

func newSessionID() string {
	return fmt.Sprintf("session-%d", os.Getpid())
}
