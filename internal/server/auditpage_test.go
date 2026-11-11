package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
)

// The Audit log page shows the log and seals it; nothing on it deletes.
func TestAuditPageSealsAndNeverDeletes(t *testing.T) {
	chain, err := audit.NewChainSink(filepath.Join(t.TempDir(), "audit-chain.jsonl"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()
	for i := 0; i < 3; i++ {
		chain.Record(context.Background(), audit.Entry{Time: time.Now(), Action: "test.event", Outcome: "ok"})
	}
	page := AuditPageHandler(chain, "", nil)
	seal := AuditSealHandler(chain)

	get := func() string {
		rec := httptest.NewRecorder()
		page(rec, httptest.NewRequest(http.MethodGet, "/router/audit", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("page: %d", rec.Code)
		}
		return rec.Body.String()
	}
	body := get()
	for _, want := range []string{"Audit log", "Seal now", "Download the whole log", "records 1 to 3", "never sealed by itself"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if low := strings.ToLower(body); strings.Contains(low, ">delete") || strings.Contains(low, "remove") {
		t.Error("the page offers a way to delete")
	}

	rec := httptest.NewRecorder()
	seal(rec, httptest.NewRequest(http.MethodPost, "/router/audit/seal", nil))
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "ok=") {
		t.Fatalf("seal: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	body = get()
	if !strings.Contains(body, "audit-chain-000000001-000000004.jsonl") {
		t.Errorf("sealed file not listed:\n%s", body)
	}
	// Only the continuing record is in the new file: the button is off, and
	// a second press is refused instead of sealing an empty file.
	if !strings.Contains(body, "disabled") {
		t.Error("the button is still active with nothing to seal")
	}
	rec = httptest.NewRecorder()
	seal(rec, httptest.NewRequest(http.MethodPost, "/router/audit/seal", nil))
	if !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Fatalf("second seal was not refused: %s", rec.Header().Get("Location"))
	}
	if segs, _ := chain.Segments(); len(segs) != 1 {
		t.Fatalf("%d sealed files, want 1", len(segs))
	}
	// Every record is still there and the chain is whole.
	var all bytes.Buffer
	_ = chain.ExportTo(&all)
	if n, err := audit.VerifyChain(&all); err != nil || n != 5 {
		t.Fatalf("chain: %d records, err %v", n, err)
	}
}
