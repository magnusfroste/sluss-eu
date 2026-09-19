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

	"github.com/magnusfroste/sluss/internal/openai"
)

// MockAdapter is a thin HTTP client against the mock-provider binary. It is
// used in dev and tests. Real provider adapters will follow the same shape.
type MockAdapter struct {
	BaseURL string
	Client  *http.Client
}

func (m *MockAdapter) Name() string { return "mock" }

func (m *MockAdapter) Complete(ctx context.Context, req *NormalizedModelRequest) (*openai.ChatResponse, error) {
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
