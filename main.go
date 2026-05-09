package main

import (
	"log"

	"github.com/kluu/monarch-mcp/monarch"
	"github.com/mark3labs/mcp-go/server"
)

var monarchToken string

func main() {
	if monarchToken == "" {
		log.Fatal("MONARCH_TOKEN not set at build time")
	}

	s := server.NewMCPServer("monarch-money", "1.0.0", server.WithToolCapabilities(false))
	registerTools(s, monarch.NewClient(monarchToken))
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
