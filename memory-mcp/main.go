package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/server"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/storage"
)

func main() {
	transport := flag.String("transport", "stdio", "Transport mode: stdio or http")
	port := flag.String("port", "8081", "HTTP port (only used with --transport http)")
	dataDir := flag.String("data-dir", "./data", "Directory for SQLite databases")
	flag.Parse()

	// Open the meta store
	meta, err := storage.OpenMeta(*dataDir)
	if err != nil {
		log.Fatalf("Failed to open meta store: %v", err)
	}
	defer meta.Close()

	// Build the MCP server with all tools registered
	srv := server.New(meta)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch *transport {
	case "stdio":
		log.Println("Memory MCP server starting (stdio)")
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case "http":
		addr := ":" + *port
		handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return srv
		}, nil)
		log.Printf("Memory MCP server listening on %s", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	default:
		log.Fatalf("Unknown transport: %s (use stdio or http)", *transport)
	}
}
