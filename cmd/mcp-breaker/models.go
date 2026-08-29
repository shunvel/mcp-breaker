package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shunvel/mcp-breaker/pkg/config"
	"github.com/shunvel/mcp-breaker/pkg/dashboard"
	"github.com/shunvel/mcp-breaker/pkg/embed"
)

func runDashboard(args []string) error {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg := config.Default()
	return dashboard.Run(cfg)
}

func runModels(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("models requires a subcommand: download, status")
	}
	cfg := config.Default()
	switch args[0] {
	case "download":
		return runModelsDownload(cfg)
	case "status":
		return runModelsStatus(cfg)
	default:
		return fmt.Errorf("unknown models subcommand %q", args[0])
	}
}

func runModelsDownload(cfg config.Config) error {
	fmt.Fprintf(os.Stderr, "Downloading model to %s...\n", cfg.ModelDir())
	if err := embed.Download(cfg.ModelDir()); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Download complete.")
	return runModelsStatus(cfg)
}

func runModelsStatus(cfg config.Config) error {
	st := embed.Status(cfg.ModelDir())
	fmt.Printf("Model directory: %s\n", st.Dir)
	fmt.Printf("  model.onnx:      %v\n", st.ModelReady)
	fmt.Printf("  tokenizer.json:  %v\n", st.TokenizerReady)
	fmt.Printf("Ready:             %v\n", st.Ready)
	return nil
}
