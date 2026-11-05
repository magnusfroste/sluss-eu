package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

// MockAdapter is a thin HTTP client against the mock-provider binary. It is
// used in dev and tests. Real provider adapters will follow the same shape.
// With an empty BaseURL it answers in-process instead, so a container that
// ships only the router still answers before any provider is configured.
type MockAdapter struct {
	BaseURL string
	Client  *http.Client
}

func (m *MockAdapter) Name() string { return "mock" }

func (m *MockAdapter) Complete(ctx context.Context, req *NormalizedModelRequest) (*openai.ChatResponse, error) {
	if m.BaseURL == "" {
		return inProcessMock(req), nil
	}
	if _, err := url.Parse(m.BaseURL); err != nil {
		return nil, fmt.Errorf("mock adapter: invalid base url: %w", err)
	}
	body, err := json.Marshal(req.ToOpenAI())
	if err != nil {
		return nil, fmt.Errorf("mock adapter: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := m.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrProviderTimeout
		}
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrProviderRateLimit
	case resp.StatusCode >= 500:
		return nil, ErrProvider5xx
	case resp.StatusCode >= 400:
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mock adapter: %d: %s", resp.StatusCode, string(raw))
	}

	var out openai.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, ErrProviderBadResp
	}
	return &out, nil
}

// inProcessMock is the built-in test answer: it says plainly that no real model
// is connected, so nobody mistakes it for an answer.
func inProcessMock(req *NormalizedModelRequest) *openai.ChatResponse {
	r := req.ToOpenAI()
	var prompt strings.Builder
	for i, msg := range r.Messages {
		if i > 0 {
			prompt.WriteByte('\n')
		}
		prompt.WriteString(msg.Content)
	}
	preview := prompt.String()
	if len(preview) > 60 {
		preview = preview[:60] + "..."
	}
	text := "Test answer from Sluss's built-in mock — no real model is connected yet. " +
		"Add one under Providers and Models, or set OPENROUTER_API_KEY. You asked: " + preview
	in := (prompt.Len() + 3) / 4
	return &openai.ChatResponse{
		ID:      "chatcmpl_mock_" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   r.Model,
		Choices: []openai.Choice{{
			Index:        0,
			Message:      openai.Message{Role: "assistant", Content: text},
			FinishReason: "stop",
		}},
		Usage: openai.Usage{PromptTokens: in, CompletionTokens: 24, TotalTokens: in + 24},
	}
}
