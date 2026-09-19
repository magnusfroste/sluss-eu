// Command sdk-metadata demonstrates the routing-hint helpers in internal/sdk
// (ISSUE-048). It builds a chat request, attaches project/task/risk hints both
// as request metadata and as X-Router-* headers, and prints both forms.
//
// Run with:
//
//	go run ./examples/sdk-metadata
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/sdk"
)

func main() {
	// Build hints once; reuse for either transport.
	hints := sdk.New().
		Project("prj_payments").
		Task(sdk.TaskSecurityReview).
		Risk(sdk.RiskHigh).
		Latency(sdk.LatencyQuality)

	// Option A: embed hints in the request body's metadata (backward
	// compatible — existing metadata is preserved).
	req := &openai.ChatRequest{
		Model: "auto",
		Messages: []openai.Message{
			{Role: "user", Content: "Review this change to the auth/session handling."},
		},
	}
	hints.Apply(req)

	body, _ := json.MarshalIndent(req, "", "  ")
	fmt.Println("Request body with metadata hints:")
	fmt.Println(string(body))

	// Option B: send the same hints as X-Router-* headers instead.
	fmt.Println("\nEquivalent headers:")
	if err := hints.Headers().Write(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
