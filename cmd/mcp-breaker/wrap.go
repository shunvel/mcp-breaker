package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shunvel/mcp-breaker/pkg/breaker"
	"github.com/shunvel/mcp-breaker/pkg/config"
	"github.com/shunvel/mcp-breaker/pkg/proxy"
	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

func runWrap(args []string) error {
	fs := flag.NewFlagSet("wrap", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	noSemantic := fs.Bool("no-semantic", false, "disable semantic stagnation detector")
	threshold := fs.Float64("semantic-threshold", config.DefaultSemanticThreshold, "cosine similarity trip threshold")
	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("wrap requires a command after --")
	}

	command := remaining[0]
	childArgs := remaining[1:]

	cfg := config.Default()
	cfg.SemanticEnabled = !*noSemantic
	cfg.SemanticThreshold = *threshold

	if err := os.MkdirAll(cfg.RunDir, 0o755); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := log.New(os.Stderr, "mcp-breaker: ", log.LstdFlags)
	logger.Println(breaker.EchoDetectorDescription())
	logger.Println(breaker.GraphLedgerDescription())

	stack := breaker.NewStack(cfg, logger)
	control := telemetry.NewControlServer(cfg.ControlSocket, cfg.SessionID, stack.Ledger.ResumeFunc())
	if err := control.Start(); err != nil {
		logger.Printf("control server: %v (dashboard resume unavailable)", err)
	} else {
		defer control.Stop()
	}

	return proxy.RunWrap(ctx, proxy.WrapConfig{
		Command:     command,
		Args:        childArgs,
		Stderr:      os.Stderr,
		Logger:      logger,
		Interceptor: stack.Interceptor(),
	})
}
