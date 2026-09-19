package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

// RetryAdapter wraps any Adapter and retries on retriable errors.
// Streaming is not retried here — streaming fallback is handled in the chat
// handler (ISSUE-030).
type RetryAdapter struct {
	Inner      Adapter
	MaxRetries int           // additional attempts after first (0 = no retry)
	BaseDelay  time.Duration // delay before first retry; doubles each attempt
	Timeout    time.Duration // per-attempt context timeout (0 = inherit caller ctx)
}

func (r *RetryAdapter) Name() string { return r.Inner.Name() }

func (r *RetryAdapter) Complete(ctx context.Context, req *NormalizedModelRequest) (*openai.ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.BaseDelay << uint(attempt-1)
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, ctx.Err()
			case <-t.C:
			}
		}

		resp, err := r.attemptComplete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetriable(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w (after %d attempts): %w", ErrProviderTimeout, r.MaxRetries+1, lastErr)
}

func (r *RetryAdapter) attemptComplete(ctx context.Context, req *NormalizedModelRequest) (*openai.ChatResponse, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	return r.Inner.Complete(ctx, req)
}

// Stream delegates directly to the inner adapter if it supports streaming.
// Per-attempt retry for streaming is the responsibility of the chat handler.
func (r *RetryAdapter) Stream(ctx context.Context, req *NormalizedModelRequest) (<-chan StreamChunk, error) {
	sa, ok := r.Inner.(StreamingAdapter)
	if !ok {
		return nil, fmt.Errorf("%w: streaming not supported by %s", ErrProviderBadReq, r.Inner.Name())
	}
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		// Relay the cancel into the channel close so callers drain cleanly.
		ch := make(chan StreamChunk, 4)
		go func() {
			defer cancel()
			inner, err := sa.Stream(ctx, req)
			if err != nil {
				select {
				case ch <- StreamChunk{Err: err}:
				case <-ctx.Done():
				}
				close(ch)
				return
			}
			for chunk := range inner {
				select {
				case ch <- chunk:
				case <-ctx.Done():
					close(ch)
					return
				}
				if chunk.Done || chunk.Err != nil {
					break
				}
			}
			close(ch)
		}()
		return ch, nil
	}
	return sa.Stream(ctx, req)
}

// isRetriable returns true for transient provider errors that are safe to retry.
func isRetriable(err error) bool {
	return errors.Is(err, ErrProviderTimeout) ||
		errors.Is(err, ErrProviderRateLimit) ||
		errors.Is(err, ErrProvider5xx)
}
