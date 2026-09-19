package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/outcomes"
)

// OutcomeOptions configures the outcome handler.
type OutcomeOptions struct {
	Store  *outcomes.Store
	Logger *slog.Logger
}

// OutcomeHandler handles POST /router/outcomes — clients report whether a
// routed response was accepted, so acceptance rates can be tracked per model
// and task class (ISSUE-039).
func OutcomeHandler(opts OutcomeOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.Store == nil {
			writeError(w, http.StatusServiceUnavailable, "outcomes_unavailable", "outcome store not configured")
			return
		}
		var o outcomes.Outcome
		if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		if err := opts.Store.Record(o); err != nil {
			status, code := mapOutcomeError(err)
			writeError(w, status, code, err.Error())
			return
		}

		if opts.Logger != nil {
			opts.Logger.InfoContext(r.Context(), "outcome_recorded",
				"request_id", middleware.RequestIDFromContext(r.Context()),
				"outcome_request_id", o.RequestID,
				"verdict", o.Verdict,
				"model", o.Model,
				"task_type", o.TaskType,
			)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     "recorded",
			"request_id": o.RequestID,
		})
	}
}

func mapOutcomeError(err error) (status int, code string) {
	switch {
	case errors.Is(err, outcomes.ErrMissingRequestID):
		return http.StatusBadRequest, "missing_request_id"
	case errors.Is(err, outcomes.ErrInvalidVerdict):
		return http.StatusBadRequest, "invalid_verdict"
	case errors.Is(err, outcomes.ErrInvalidRating):
		return http.StatusBadRequest, "invalid_rating"
	default:
		return http.StatusBadRequest, "invalid_outcome"
	}
}
