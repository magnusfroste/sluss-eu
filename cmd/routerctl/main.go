// Command routerctl is a small CLI for debugging routing decisions. It posts a
// dry-run request to a running router's /router/decision endpoint and prints the
// selected model, fallback chain and decision explanations — without making any
// provider call (ISSUE-047).
//
// Example:
//
//	routerctl -url http://localhost:8080 -key local_router_key \
//	  -message "Refactor the auth middleware and add tests"
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

func main() {
	var (
		url     = flag.String("url", envOr("ROUTER_URL", "http://localhost:8080"), "router base URL")
		key     = flag.String("key", firstEnv("ROUTER_API_KEY", "LOCAL_API_KEY"), "API key (Bearer)")
		model   = flag.String("model", "auto", "requested model (\"auto\" lets the router choose)")
		message = flag.String("message", "", "user message to classify and route")
		explain = flag.Bool("explain", true, "request decision explanations")
		stream  = flag.Bool("stream", false, "evaluate as a streaming request")
		timeout = flag.Duration("timeout", 10*time.Second, "request timeout")
	)
	flag.Parse()

	if strings.TrimSpace(*message) == "" {
		fmt.Fprintln(os.Stderr, "error: -message is required")
		flag.Usage()
		os.Exit(2)
	}

	req := &openai.ChatRequest{
		Model:    *model,
		Stream:   *stream,
		Messages: []openai.Message{{Role: "user", Content: *message}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := &http.Client{Timeout: *timeout}
	dec, err := fetchDecision(ctx, client, *url, *key, req, *explain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	render(os.Stdout, dec)
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := strings.TrimSpace(os.Getenv(n)); v != "" {
			return v
		}
	}
	return ""
}
