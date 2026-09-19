package provider_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
)

type countingAdapter struct {
	calls  atomic.Int32
	errSeq []error // return errSeq[i] on attempt i; last entry repeated
	resp   *openai.ChatResponse
}

func (c *countingAdapter) Name() string { return "counting" }

func (c *countingAdapter) Complete(_ context.Context, _ *provider.NormalizedModelRequest) (*openai.ChatResponse, error) {
	i := int(c.calls.Add(1)) - 1
	if i >= len(c.errSeq) {
		i = len(c.errSeq) - 1
	}
	if c.errSeq[i] != nil {
		return nil, c.errSeq[i]
	}
	return c.resp, nil
}

func TestRetryAdapter_SuccessOnFirstAttempt(t *testing.T) {
	inner := &countingAdapter{
		errSeq: []error{nil},
		resp:   &openai.ChatResponse{ID: "ok"},
	}
	ra := &provider.RetryAdapter{Inner: inner, MaxRetries: 2, BaseDelay: time.Millisecond}
	resp, err := ra.Complete(context.Background(), &provider.NormalizedModelRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "ok" {
		t.Fatalf("unexpected response id %q", resp.ID)
	}
	if inner.calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", inner.calls.Load())
	}
}

func TestRetryAdapter_RetryOnTimeout(t *testing.T) {
	inner := &countingAdapter{
		errSeq: []error{provider.ErrProviderTimeout, provider.ErrProviderTimeout, nil},
		resp:   &openai.ChatResponse{ID: "after-retry"},
	}
	ra := &provider.RetryAdapter{Inner: inner, MaxRetries: 2, BaseDelay: time.Millisecond}
	resp, err := ra.Complete(context.Background(), &provider.NormalizedModelRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "after-retry" {
		t.Fatalf("unexpected response id %q", resp.ID)
	}
	if inner.calls.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", inner.calls.Load())
	}
}

func TestRetryAdapter_NoRetryOnAuthError(t *testing.T) {
	inner := &countingAdapter{
		errSeq: []error{provider.ErrProviderAuth},
	}
	ra := &provider.RetryAdapter{Inner: inner, MaxRetries: 3, BaseDelay: time.Millisecond}
	_, err := ra.Complete(context.Background(), &provider.NormalizedModelRequest{})
	if !errors.Is(err, provider.ErrProviderAuth) {
		t.Fatalf("expected ErrProviderAuth, got %v", err)
	}
	if inner.calls.Load() != 1 {
		t.Fatalf("non-retriable error should not be retried, got %d calls", inner.calls.Load())
	}
}

func TestRetryAdapter_ExhaustsRetries(t *testing.T) {
	inner := &countingAdapter{
		errSeq: []error{provider.ErrProvider5xx},
	}
	ra := &provider.RetryAdapter{Inner: inner, MaxRetries: 2, BaseDelay: time.Millisecond}
	_, err := ra.Complete(context.Background(), &provider.NormalizedModelRequest{})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if inner.calls.Load() != 3 {
		t.Fatalf("expected 3 calls (1 + 2 retries), got %d", inner.calls.Load())
	}
}

func TestRetryAdapter_RespectsContextCancellation(t *testing.T) {
	inner := &countingAdapter{
		errSeq: []error{provider.ErrProviderTimeout},
	}
	ra := &provider.RetryAdapter{Inner: inner, MaxRetries: 10, BaseDelay: 50 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := ra.Complete(ctx, &provider.NormalizedModelRequest{})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
