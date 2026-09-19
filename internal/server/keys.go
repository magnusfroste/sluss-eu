package server

// API-key provisioning endpoints (ISSUE-079): admin-gated JSON API to mint,
// list and revoke DB-backed keys. The minted secret is returned exactly once in
// the mint response and never again; list responses carry only ids/metadata.

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/magnusfroste/sluss/internal/apikeys"
)

// keyListItem is the safe (secretless) representation of a key for listing.
type keyListItem struct {
	KeyID      string   `json:"key_id"`
	TenantID   string   `json:"tenant_id"`
	ProjectID  string   `json:"project_id"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"created_at"`
	Revoked    bool     `json:"revoked"`
	RevokedAt  string   `json:"revoked_at,omitempty"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// KeysListHandler returns all keys (including revoked), secretless.
func KeysListHandler(mgr *apikeys.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recs, err := mgr.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "keys_list_failed", err.Error())
			return
		}
		items := make([]keyListItem, 0, len(recs))
		for _, k := range recs {
			items = append(items, keyListItem{
				KeyID: k.KeyID, TenantID: k.TenantID, ProjectID: k.ProjectID,
				Role: k.Role, Scopes: k.Scopes, CreatedAt: fmtTime(k.CreatedAt),
				Revoked: k.Revoked(), RevokedAt: fmtTime(k.RevokedAt), LastUsedAt: fmtTime(k.LastUsedAt),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": items})
	}
}

// KeysMintHandler creates a key and returns the plaintext ONCE.
func KeysMintHandler(mgr *apikeys.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TenantID  string   `json:"tenant_id"`
			ProjectID string   `json:"project_id"`
			Role      string   `json:"role"`
			Scopes    []string `json:"scopes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		plaintext, rec, err := mgr.Mint(apikeys.MintRequest{
			TenantID: req.TenantID, ProjectID: req.ProjectID, Role: req.Role, Scopes: req.Scopes,
			Actor: AdminUserFromContext(r.Context()),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "key_mint_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key_id":     rec.KeyID,
			"key":        plaintext, // shown once — never returned again
			"tenant_id":  rec.TenantID,
			"project_id": rec.ProjectID,
			"role":       rec.Role,
			"scopes":     rec.Scopes,
			"warning":    "store this key now — it is shown only once and cannot be recovered",
		})
	}
}

// KeysRevokeHandler revokes a key by id.
func KeysRevokeHandler(mgr *apikeys.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyID := r.PathValue("key_id")
		if keyID == "" {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "key_id is required")
			return
		}
		ok, err := mgr.Revoke(keyID, AdminUserFromContext(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "key_revoke_failed", err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "key_not_found", "no active key with that id")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"key_id": keyID, "revoked": true})
	}
}
