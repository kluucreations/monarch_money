package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

type contextKey string

const monarchTokenKey contextKey = "monarch_token"

func main() {
	monarchToken := os.Getenv("MONARCH_TOKEN")
	if monarchToken == "" {
		log.Fatal("MONARCH_TOKEN env var is required")
	}

	clientSecret := os.Getenv("OAUTH_CLIENT_SECRET")
	if clientSecret == "" {
		log.Fatal("OAUTH_CLIENT_SECRET env var is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://0.0.0.0:%s", port)
	}

	s := server.NewMCPServer("monarch-money", "1.0.0", server.WithToolCapabilities(false))
	registerTools(s, monarchToken)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	registerOAuthHandlers(mux, monarchToken, clientSecret, baseURL)
	registerMCPHandlers(mux, s, baseURL)

	log.Printf("monarch-money MCP server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func registerMCPHandlers(mux *http.ServeMux, s *server.MCPServer, baseURL string) {
	streamable := server.NewStreamableHTTPServer(s, server.WithEndpointPath("/mcp"))
	mux.Handle("/mcp", withCORS(withAuth(streamable)))
	mux.Handle("/mcp/", withCORS(withAuth(streamable)))

	sse := server.NewSSEServer(s, server.WithBaseURL(baseURL))
	mux.Handle("/sse", withCORS(withAuth(sse)))
	mux.Handle("/message", withCORS(withAuth(sse)))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","server":"monarch-money"}`)
}

func withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token == auth {
			log.Printf("[auth] missing or invalid Authorization header")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("[auth] token received: %s...%s", token[:4], token[len(token)-4:])
		ctx := context.WithValue(r.Context(), monarchTokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s", r.Method, r.URL.Path)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Mcp-Session-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
