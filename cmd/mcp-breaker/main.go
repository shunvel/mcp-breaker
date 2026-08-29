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
	"github.com/shunvel/mcp-breaker/pkg/proxy"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("mcp-breaker: ")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "wrap":
		if err := runWrap(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `mcp-breaker — semantic MCP circuit breaker proxy

Usage:
  mcp-breaker wrap -- <command> [args...]

Example:
  mcp-breaker wrap -- npx -y @modelcontextprotocol/server-filesystem /tmp

`)
}

func runWrap(args []string) error {
	fs := flag.NewFlagSet("wrap", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("wrap requires a command after --")
	}

	command := remaining[0]
	childArgs := remaining[1:]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := log.New(os.Stderr, "mcp-breaker: ", log.LstdFlags)
	logger.Println(breaker.EchoDetectorDescription())

	return proxy.RunWrap(ctx, proxy.WrapConfig{
		Command:     command,
		Args:        childArgs,
		Stderr:      os.Stderr,
		Logger:      logger,
		Interceptor: breaker.NewEchoDetector(),
	})
}
