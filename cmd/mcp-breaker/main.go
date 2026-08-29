package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("mcp-breaker: ")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "wrap":
		err = runWrap(os.Args[2:])
	case "dashboard":
		err = runDashboard(os.Args[2:])
	case "models":
		err = runModels(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `mcp-breaker — semantic MCP circuit breaker proxy

Usage:
  mcp-breaker wrap [--no-semantic] [--semantic-threshold=0.88] -- <command> [args...]
  mcp-breaker dashboard
  mcp-breaker models download|status
  mcp-breaker help

Examples:
  mcp-breaker wrap -- npx -y @modelcontextprotocol/server-filesystem /tmp
  mcp-breaker dashboard
  mcp-breaker models download

`)
}
