// Mock provider used in local dev and tests. It exposes /v1/chat/completions
// in the OpenAI shape and simulates failure modes via the X-Mock-Behavior
// request header: "timeout", "rate_limit", "5xx", "bad_response".
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", chatHandler(logger))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := os.Getenv("MOCK_PROVIDER_ADDR")
	if addr == "" {
		addr = ":18080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("mock provider listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("mock provider failed", "err", err)
		os.Exit(1)
	}
}

func chatHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":{"message":"bad json","type":"invalid_request_error"}}`, http.StatusBadRequest)
			return
		}

		switch r.Header.Get("X-Mock-Behavior") {
		case "timeout":
			time.Sleep(10 * time.Second)
		case "rate_limit":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`))
			return
		case "5xx":
			w.WriteHeader(http.StatusBadGateway)
			return
		case "bad_response":
			_, _ = w.Write([]byte("not json"))
			return
		}

		prompt := promptText(req.Messages)
		preview := prompt
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}

		resp := openai.ChatResponse{
			ID:      "chatcmpl_mock_" + strconv.FormatInt(time.Now().UnixNano(), 36),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []openai.Choice{{
				Index: 0,
				Message: openai.Message{
					Role:    "assistant",
					Content: "mock response to: " + preview,
				},
				FinishReason: "stop",
			}},
			Usage: openai.Usage{
				PromptTokens:     estimateTokens(prompt),
				CompletionTokens: 12,
				TotalTokens:      estimateTokens(prompt) + 12,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func promptText(messages []openai.Message) string {
	var b strings.Builder
	for i, m := range messages {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.Content)
	}
	return b.String()
}

// estimateTokens is a quick 4-chars-per-token heuristic, fine for the mock.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}
