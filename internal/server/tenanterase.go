package server

// Audited tenant erasure (GDPR art. 17). A customer/tenant asks to be gone:
// this deletes the tenant's OPERATIONAL footprint — durable request-history
// rows and the in-memory spend attribution — and can revoke the tenant's API
// keys in the same act. Same evidence stance as the demo reset (ISSUE-096):
// the audit chain is NEVER selectively deleted, and the erasure itself is
// recorded in it — you can delete data, but never the fact that you deleted it.

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/spend"
)

// actionTenantErasure audits a per-tenant erasure.
const actionTenantErasure = audit.Action("data.tenant_erasure")

// TenantEraseOptions carries the surfaces holding tenant-attributed data.
type TenantEraseOptions struct {
	History    *history.Store
	Spend      *spend.Tracker
	KeyManager *apikeys.Manager // optional; enables the revoke-keys step
	Auditor    audit.Sink
}

// TenantEraseHandler performs the audited erasure and bounces back to the
// policy console with the outcome.
func TenantEraseHandler(o TenantEraseOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if o.History == nil {
			redirectPolicy(w, r, "", "tenant erasure needs a data dir (ROUTER_DATA_DIR)")
			return
		}
		_ = r.ParseForm()
		tenantID := strings.TrimSpace(r.FormValue("tenant_id"))
		if tenantID == "" {
			redirectPolicy(w, r, "", "provide a tenant id to erase")
			return
		}

		rows, err := o.History.EraseTenant(tenantID)
		if err != nil {
			redirectPolicy(w, r, "", "erase: "+err.Error())
			return
		}
		o.Spend.EraseTenant(tenantID)

		// Optionally revoke every key minted for the tenant, so the erased
		// tenant cannot quietly re-accumulate data a minute later.
		keysRevoked := 0
		if r.FormValue("revoke_keys") != "" && o.KeyManager != nil && o.KeyManager.Enabled() {
			if keys, err := o.KeyManager.List(); err == nil {
				actor := AdminUserFromContext(r.Context())
				for _, k := range keys {
					if k.TenantID != tenantID || !k.RevokedAt.IsZero() {
						continue
					}
					if ok, err := o.KeyManager.Revoke(k.KeyID, actor); err == nil && ok {
						keysRevoked++
					}
				}
			}
		}

		audit.Record(r.Context(), o.Auditor, audit.Entry{
			Action:   actionTenantErasure,
			Actor:    AdminUserFromContext(r.Context()),
			TenantID: tenantID,
			Target:   tenantID,
			Reason:   fmt.Sprintf("tenant erasure: %d request rows deleted, %d keys revoked; audit chain untouched", rows, keysRevoked),
			Detail: map[string]string{
				"rows_deleted": fmt.Sprint(rows),
				"keys_revoked": fmt.Sprint(keysRevoked),
			},
		})
		msg := fmt.Sprintf("Tenant %s erased: %d request rows deleted", tenantID, rows)
		if keysRevoked > 0 {
			msg += fmt.Sprintf(", %d keys revoked", keysRevoked)
		}
		redirectPolicy(w, r, msg+". The erasure is recorded in the audit chain.", "")
	}
}
