package main

import "github.com/shunvel/mcp-breaker/internal/testmcp"

func main() {
	if err := testmcp.RunFakeServer(); err != nil {
		panic(err)
	}
}
