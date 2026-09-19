package contextproc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/router"
)

type testProcessor struct {
	name string
	fn   func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error)
}

func (p testProcessor) Name() string { return p.name }

func (p testProcessor) Process(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
	return p.fn(ctx, req, job)
}

func TestPipelineRunsNoopProcessor(t *testing.T) {
	pipeline := &Pipeline{
		Processors: []Processor{testProcessor{
			name: "noop",
			fn: func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
				return Result{TokensSaved: 0}, nil
			},
		}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	got := pipeline.Run(context.Background(), &provider.NormalizedModelRequest{}, &router.JobDescriptor{})

	if got.TotalTokensSaved != 0 {
		t.Fatalf("expected 0 tokens saved, got %d", got.TotalTokensSaved)
	}
	if len(got.Applied) != 1 || got.Applied[0].Name != "noop" || got.Applied[0].TokensSaved != 0 {
		t.Fatalf("unexpected applied entries: %+v", got.Applied)
	}
	if len(got.Skipped) != 0 {
		t.Fatalf("unexpected skipped entries: %+v", got.Skipped)
	}
}

func TestPipelineHardTimeoutSkipsSlowProcessor(t *testing.T) {
	pipeline := &Pipeline{
		Processors: []Processor{testProcessor{
			name: "slow",
			fn: func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
				time.Sleep(30 * time.Millisecond)
				req.Model = "mutated-too-late"
				return Result{TokensSaved: 10}, nil
			},
		}},
		TotalTimeout:     50 * time.Millisecond,
		PerProcessorSoft: 2 * time.Millisecond,
		PerProcessorHard: 5 * time.Millisecond,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	req := &provider.NormalizedModelRequest{Model: "auto"}

	start := time.Now()
	got := pipeline.Run(context.Background(), req, &router.JobDescriptor{})

	if elapsed := time.Since(start); elapsed > 25*time.Millisecond {
		t.Fatalf("expected hard timeout to return quickly, took %s", elapsed)
	}
	if got.TotalTokensSaved != 0 || len(got.Applied) != 0 {
		t.Fatalf("expected no applied savings, got %+v", got)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "processor_timeout" {
		t.Fatalf("expected processor timeout skip, got %+v", got.Skipped)
	}
	time.Sleep(35 * time.Millisecond)
	if req.Model != "auto" {
		t.Fatalf("request should be unchanged before fail-open continuation, got %q", req.Model)
	}
}

func TestPipelineDoesNotCommitProcessorResultAfterTotalTimeout(t *testing.T) {
	pipeline := &Pipeline{
		Processors: []Processor{testProcessor{
			name: "deadline-aware",
			fn: func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
				<-ctx.Done()
				req.Model = "mutated-after-deadline"
				return Result{TokensSaved: 10}, nil
			},
		}},
		TotalTimeout:     5 * time.Millisecond,
		PerProcessorSoft: 20 * time.Millisecond,
		PerProcessorHard: 20 * time.Millisecond,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	req := &provider.NormalizedModelRequest{Model: "auto"}

	got := pipeline.Run(context.Background(), req, &router.JobDescriptor{})

	if req.Model != "auto" {
		t.Fatalf("request should not commit after pipeline timeout, got %q", req.Model)
	}
	if got.TotalTokensSaved != 0 || len(got.Applied) != 0 {
		t.Fatalf("expected no applied result after pipeline timeout, got %+v", got)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "pipeline_timeout" {
		t.Fatalf("expected pipeline timeout skip, got %+v", got.Skipped)
	}
}

func TestPipelineRecoversPanicAndContinues(t *testing.T) {
	pipeline := &Pipeline{
		Processors: []Processor{
			testProcessor{
				name: "panic",
				fn: func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
					panic("bad processor")
				},
			},
			testProcessor{
				name: "second",
				fn: func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
					req.Model = "processed"
					return Result{TokensSaved: 7, AppliedRules: []string{"trim"}}, nil
				},
			},
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	req := &provider.NormalizedModelRequest{Model: "auto"}

	got := pipeline.Run(context.Background(), req, &router.JobDescriptor{})

	if req.Model != "processed" {
		t.Fatalf("expected second processor to run, model=%q", req.Model)
	}
	if got.TotalTokensSaved != 7 {
		t.Fatalf("expected 7 tokens saved, got %+v", got)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].Name != "panic" {
		t.Fatalf("expected panic processor skipped, got %+v", got.Skipped)
	}
	if len(got.Applied) != 1 || got.Applied[0].Name != "second" {
		t.Fatalf("expected second processor applied, got %+v", got.Applied)
	}
}

func TestPipelineErrorsFailOpenAndContinue(t *testing.T) {
	pipeline := &Pipeline{
		Processors: []Processor{
			testProcessor{
				name: "error",
				fn: func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
					return Result{}, errors.New("nope")
				},
			},
			testProcessor{
				name: "second",
				fn: func(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, error) {
					return Result{TokensSaved: 3}, nil
				},
			},
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	got := pipeline.Run(context.Background(), &provider.NormalizedModelRequest{}, &router.JobDescriptor{})

	if got.TotalTokensSaved != 3 {
		t.Fatalf("expected second processor savings, got %+v", got)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "nope" {
		t.Fatalf("expected error skip, got %+v", got.Skipped)
	}
}
