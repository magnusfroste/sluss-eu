package eventlog

import (
	"context"
	"log/slog"

	"github.com/magnusfroste/sluss/internal/metrics"
)

// LoggingHandler writes each event as a structured log line and updates
// Prometheus metrics. It is the default Handler when no DB is configured.
type LoggingHandler struct {
	Logger *slog.Logger
}

func (h *LoggingHandler) Handle(ctx context.Context, e Event) {
	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	switch e.Type {
	case EventTypeDecision:
		if d := e.Decision; d != nil {
			attrs := []any{
				"request_id", d.RequestID,
				"tenant_id", d.TenantID,
				"task_type", d.TaskType,
				"risk_level", d.RiskLevel,
				"selected_model", d.SelectedModel,
				"selected_provider", d.SelectedProvider,
				"policy_version", d.PolicyVersion,
				"estimated_cost_usd", d.EstimatedCostUSD,
				"routing_duration_ms", d.RoutingDurationMs,
				"blocked", d.Blocked,
			}
			if d.ShadowComparison != nil {
				attrs = append(attrs,
					"shadow_changed", d.ShadowComparison.Changed,
					"shadow_route_changed", d.ShadowComparison.RouteChanged,
					"shadow_selected_model", d.ShadowComparison.Secondary.SelectedModel,
					"shadow_selected_provider", d.ShadowComparison.Secondary.SelectedProvider,
					"shadow_policy_version", d.ShadowComparison.Secondary.PolicyVersion,
					"shadow_cost_delta_microusd", d.ShadowComparison.EstimatedCostDeltaMicroUSD,
				)
			}
			logger.InfoContext(ctx, "route_decision", attrs...)
			status := "success"
			if d.Blocked {
				status = "blocked"
			}
			metrics.RequestsTotal.WithLabelValues(d.TaskType, d.SelectedModel, d.SelectedProvider, status).Inc()
			// Observe in fractional milliseconds from the microsecond capture so
			// sub-millisecond routing is recorded instead of rounding to zero.
			metrics.RoutingOverheadMs.Observe(float64(d.RoutingDurationMicros) / 1000.0)
		}
	case EventTypeAttempt:
		if a := e.Attempt; a != nil {
			successStr := "false"
			if a.Success {
				successStr = "true"
			}
			attrs := []any{
				"request_id", a.RequestID,
				"provider_id", a.ProviderID,
				"model_id", a.ModelID,
				"attempt_index", a.AttemptIndex,
				"success", a.Success,
				"error_code", a.ErrorCode,
				"duration_ms", a.DurationMs,
				"input_tokens", a.InputTokens,
				"output_tokens", a.OutputTokens,
				"actual_cost_usd", a.ActualCostUSD,
				"first_token_ms", a.FirstTokenMs,
			}
			// Failures log at WARN with a distinct message so they stand out in a
			// busy container log (EasyPanel etc.); successes stay at INFO.
			if a.Success {
				logger.InfoContext(ctx, "route_attempt", attrs...)
			} else {
				logger.WarnContext(ctx, "route_attempt_failed", attrs...)
			}
			metrics.ProviderDurationMs.WithLabelValues(a.ModelID, a.ProviderID, successStr).Observe(float64(a.DurationMs))
			if a.FirstTokenMs > 0 {
				metrics.FirstTokenMs.WithLabelValues(a.ModelID, a.ProviderID).Observe(float64(a.FirstTokenMs))
			}
		}
	}
}
