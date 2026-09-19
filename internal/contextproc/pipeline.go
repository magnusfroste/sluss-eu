package contextproc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/router"
)

const (
	DefaultTotalTimeout     = 20 * time.Millisecond
	DefaultPerProcessorSoft = 10 * time.Millisecond
	DefaultPerProcessorHard = 15 * time.Millisecond
)

type Pipeline struct {
	Processors       []Processor
	TotalTimeout     time.Duration
	PerProcessorSoft time.Duration
	PerProcessorHard time.Duration
	Logger           *slog.Logger
}

type AppliedEntry struct {
	Name        string   `json:"name"`
	TokensSaved int      `json:"tokens_saved"`
	Rules       []string `json:"rules,omitempty"`
}

type SkippedEntry struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type PipelineResult struct {
	TotalTokensSaved int
	Applied          []AppliedEntry
	Skipped          []SkippedEntry
}

func NewNoopPipeline() *Pipeline {
	return &Pipeline{
		TotalTimeout:     DefaultTotalTimeout,
		PerProcessorSoft: DefaultPerProcessorSoft,
		PerProcessorHard: DefaultPerProcessorHard,
	}
}

func (p *Pipeline) Run(ctx context.Context, req *provider.NormalizedModelRequest, job *router.JobDescriptor) PipelineResult {
	if p == nil || len(p.Processors) == 0 {
		return PipelineResult{}
	}

	totalTimeout := durationOrDefault(p.TotalTimeout, DefaultTotalTimeout)
	totalCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()

	var out PipelineResult
	for _, processor := range p.Processors {
		if processor == nil {
			continue
		}
		if totalCtx.Err() != nil {
			out.Skipped = append(out.Skipped, SkippedEntry{Name: processor.Name(), Reason: "pipeline_timeout"})
			p.log(totalCtx, processor.Name(), Result{SkippedReason: "pipeline_timeout"})
			continue
		}

		result, processedReq, err := p.runOne(totalCtx, processor, req, job)
		if err != nil {
			result.SkippedReason = err.Error()
		}
		if result.SkippedReason != "" {
			out.Skipped = append(out.Skipped, SkippedEntry{Name: processor.Name(), Reason: result.SkippedReason})
			p.log(totalCtx, processor.Name(), result)
			continue
		}
		if processedReq != nil {
			*req = *processedReq
		}
		out.TotalTokensSaved += result.TokensSaved
		out.Applied = append(out.Applied, AppliedEntry{
			Name:        processor.Name(),
			TokensSaved: result.TokensSaved,
			Rules:       append([]string(nil), result.AppliedRules...),
		})
		p.log(totalCtx, processor.Name(), result)
	}
	return out
}

func (p *Pipeline) runOne(ctx context.Context, processor Processor, req *provider.NormalizedModelRequest, job *router.JobDescriptor) (Result, *provider.NormalizedModelRequest, error) {
	softTimeout := durationOrDefault(p.PerProcessorSoft, DefaultPerProcessorSoft)
	hardTimeout := durationOrDefault(p.PerProcessorHard, DefaultPerProcessorHard)
	if hardTimeout < softTimeout {
		hardTimeout = softTimeout
	}

	softCtx, cancel := context.WithTimeout(ctx, softTimeout)
	defer cancel()
	hardCtx, hardCancel := context.WithTimeout(ctx, hardTimeout)
	defer hardCancel()

	type processorResult struct {
		result Result
		req    *provider.NormalizedModelRequest
		err    error
	}
	done := make(chan processorResult, 1)
	processorReq := req.Clone()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- processorResult{err: fmt.Errorf("panic: %v", r)}
			}
		}()
		result, err := processor.Process(softCtx, processorReq, job)
		done <- processorResult{result: result, req: processorReq, err: err}
	}()

	select {
	case <-ctx.Done():
		return Result{SkippedReason: "pipeline_timeout"}, nil, nil
	case <-hardCtx.Done():
		if ctx.Err() != nil {
			return Result{SkippedReason: "pipeline_timeout"}, nil, nil
		}
		return Result{SkippedReason: "processor_timeout"}, nil, nil
	case got := <-done:
		if ctx.Err() != nil {
			return Result{SkippedReason: "pipeline_timeout"}, nil, nil
		}
		if hardCtx.Err() != nil {
			return Result{SkippedReason: "processor_timeout"}, nil, nil
		}
		if got.err != nil {
			return got.result, nil, got.err
		}
		return got.result, got.req, nil
	}
}

func (p *Pipeline) log(ctx context.Context, name string, result Result) {
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(ctx, "context_processor",
		"name", name,
		"saved", result.TokensSaved,
		"skipped", result.SkippedReason,
	)
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
