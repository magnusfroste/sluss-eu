package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func rolloutRig(t *testing.T) (RolloutOptions, auth.KeyStore) {
	t.Helper()
	ks := auth.NewInMemoryKeyStore()
	ks.Add("dept-finance-key", &tenant.Tenant{ID: "finance", Project: "p", KeyID: "k1", Scopes: auth.AllScopes()})
	return RolloutOptions{History: newUserStore(t), PublicURL: "https://sluss.example.eu"}, ks
}

func postCoverage(t *testing.T, o RolloutOptions, ks auth.KeyStore, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := auth.Middleware(ks)(http.HandlerFunc(CoverageReportHandler(o)))
	req := httptest.NewRequest(http.MethodPost, "/router/coverage", strings.NewReader(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCoverageReportNeedsAKeyAndUpsertsPerDevice(t *testing.T) {
	o, ks := rolloutRig(t)
	if rec := postCoverage(t, o, ks, "", `{"device":"mac-01","direct_ai":"open"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key: want 401, got %d", rec.Code)
	}
	if rec := postCoverage(t, o, ks, "dept-finance-key", `{"device":"","direct_ai":"open"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing device: want 400, got %d", rec.Code)
	}
	if rec := postCoverage(t, o, ks, "dept-finance-key", `{"device":"mac-01","direct_ai":"maybe"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad direct_ai: want 400, got %d", rec.Code)
	}

	if rec := postCoverage(t, o, ks, "dept-finance-key", `{"device":"mac-01","os":"Darwin","direct_ai":"open","sluss":"ok"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("report: want 204, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := postCoverage(t, o, ks, "dept-finance-key", `{"device":"win-07","os":"Windows","direct_ai":"closed","sluss":"nope"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("report 2: want 204, got %d", rec.Code)
	}
	sum := o.History.SummarizeCoverage()
	if sum.Devices != 2 || sum.DirectOpen != 1 || sum.SlussDown != 1 {
		t.Fatalf("summary = %+v, want 2 devices, 1 open, 1 down", sum)
	}

	// The same device reporting again replaces its row — the path got closed.
	if rec := postCoverage(t, o, ks, "dept-finance-key", `{"device":"mac-01","os":"Darwin","direct_ai":"closed","sluss":"ok"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("report 3: want 204, got %d", rec.Code)
	}
	sum = o.History.SummarizeCoverage()
	if sum.Devices != 2 || sum.DirectOpen != 0 {
		t.Fatalf("after re-report: %+v, want 2 devices, 0 open", sum)
	}
	rows, _ := o.History.ListCoverage()
	for _, r := range rows {
		if r.TenantID != "finance" {
			t.Fatalf("row %s attributed to %q, want the key's department", r.Device, r.TenantID)
		}
	}
}

func TestRolloutPageListsDevicesAndFiles(t *testing.T) {
	o, ks := rolloutRig(t)
	postCoverage(t, o, ks, "dept-finance-key", `{"device":"mac-01","os":"Darwin","direct_ai":"open","sluss":"ok"}`)
	rec := httptest.NewRecorder()
	RolloutPageHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/router/rollout", nil))
	body := rec.Body.String()
	for _, want := range []string{"mac-01", "finance", `class="pill open"`, "sluss-env.sh", "coverage.ps1", "genai-domains.csv", "sluss-webclip.mobileconfig", "https://sluss.example.eu/blocked"} {
		if !strings.Contains(body, want) {
			t.Fatalf("rollout page lacks %q", want)
		}
	}
	if strings.Contains(body, "ROUTER_PUBLIC_URL is not set") {
		t.Fatal("public URL is set; the notice must not show")
	}
}

func TestRolloutFilesCarryTheInstanceAndNoSecrets(t *testing.T) {
	o, _ := rolloutRig(t)
	get := func(name string) (int, string, http.Header) {
		req := httptest.NewRequest(http.MethodGet, "/router/rollout/file/"+name, nil)
		req.SetPathValue("name", name)
		rec := httptest.NewRecorder()
		RolloutFileHandler(o)(rec, req)
		return rec.Code, rec.Body.String(), rec.Header()
	}
	for _, f := range rolloutFiles {
		code, body, hdr := get(f.Name)
		if code != http.StatusOK {
			t.Fatalf("%s: status %d", f.Name, code)
		}
		if !strings.Contains(hdr.Get("Content-Disposition"), f.Name) {
			t.Fatalf("%s: not served as a download", f.Name)
		}
		if !strings.Contains(body, "sluss.example.eu") {
			t.Fatalf("%s: does not carry the instance address", f.Name)
		}
		if strings.Contains(body, "dept-finance-key") || strings.Contains(body, "sk_") {
			t.Fatalf("%s: a generated file must never carry a key", f.Name)
		}
	}
	if code, _, _ := get("nope.sh"); code != http.StatusNotFound {
		t.Fatalf("unknown file: want 404, got %d", code)
	}

	// The deny list denies the providers and allows this instance.
	_, csv, _ := get("genai-domains.csv")
	if !strings.Contains(csv, "chatgpt.com,deny") || !strings.Contains(csv, "api.anthropic.com,deny") || !strings.Contains(csv, "sluss.example.eu,allow") {
		t.Fatalf("deny list wrong:\n%s", csv)
	}
	_, js, _ := get("genai-domains.json")
	var parsed struct {
		BlockPage string `json:"block_page"`
		Entries   []struct{ Domain, Action string }
	}
	if err := json.Unmarshal([]byte(js), &parsed); err != nil || parsed.BlockPage != "https://sluss.example.eu/blocked" || len(parsed.Entries) != len(genAIDomains)+1 {
		t.Fatalf("json list wrong: err=%v parsed=%+v", err, parsed)
	}
	// Coverage scripts probe the real-provider 401 signature and post with a placeholder key.
	_, sh, _ := get("coverage.sh")
	if !strings.Contains(sh, "api.openai.com/v1/models") || !strings.Contains(sh, `"401"`) || !strings.Contains(sh, "REPLACE_WITH_DEPARTMENT_KEY") || !strings.Contains(sh, "/router/coverage") {
		t.Fatalf("coverage.sh wrong:\n%s", sh)
	}
	_, ps, _ := get("coverage.ps1")
	if !strings.Contains(ps, "-eq 401") || !strings.Contains(ps, "exit 1") {
		t.Fatalf("coverage.ps1 should flag an open path as non-compliant:\n%s", ps)
	}
	// The web clip is a well-formed profile pointing at the chat with stable UUIDs.
	_, wc, _ := get("sluss-webclip.mobileconfig")
	if !strings.Contains(wc, "com.apple.webClip.managed") || !strings.Contains(wc, "https://sluss.example.eu/chat") {
		t.Fatalf("web clip wrong:\n%s", wc)
	}
	_, wc2, _ := get("sluss-webclip.mobileconfig")
	if wc != wc2 {
		t.Fatal("web clip must be byte-identical across downloads (stable UUIDs)")
	}
	_, bp, _ := get("sluss-browser-policy.json")
	if !strings.Contains(bp, `"DnsOverHttpsMode": "off"`) || !strings.Contains(bp, "/chat") {
		t.Fatalf("browser policy wrong:\n%s", bp)
	}
}

func TestBlockedPageIsPublicAndLinksToChat(t *testing.T) {
	o, _ := rolloutRig(t)
	rec := httptest.NewRecorder()
	BlockedPageHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/blocked?from=chatgpt.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "https://sluss.example.eu/chat") || !strings.Contains(body, "chatgpt.com") {
		t.Fatalf("block page lacks the chat link or the blocked host:\n%s", body)
	}
	if rec.Header().Get("X-Robots-Tag") == "" {
		t.Fatal("block page must be noindex")
	}
	// The from parameter is escaped, never rendered raw.
	rec = httptest.NewRecorder()
	BlockedPageHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/blocked?from=%3Cscript%3Ex", nil))
	if strings.Contains(rec.Body.String(), "<script>x") {
		t.Fatal("from must be HTML-escaped")
	}
}

func TestRolloutBaseFallsBackToRequestHost(t *testing.T) {
	o := RolloutOptions{}
	req := httptest.NewRequest(http.MethodGet, "/router/rollout", nil)
	req.Host = "sluss.internal:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := o.base(req); got != "https://sluss.internal:8080" {
		t.Fatalf("base = %q", got)
	}
	if hostOf("https://sluss.example.eu/x") != "sluss.example.eu" || hostOf("http://h:8080") != "h" {
		t.Fatal("hostOf wrong")
	}
}

func TestRolloutGuidesCarryTheInstanceURLs(t *testing.T) {
	o, _ := rolloutRig(t)
	rec := httptest.NewRecorder()
	RolloutPageHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/router/rollout", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"Microsoft Defender", "Cloudflare Zero Trust", "Cisco Umbrella", "Zscaler Internet Access",
		"https://sluss.example.eu/blocked?from=", "<code>https://sluss.example.eu/blocked</code>",
		"genai-defender-indicators.csv", "redirects to Sluss", "block only",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("guides lack %q", want)
		}
	}
	// Defender can't redirect; the guide must say so rather than imply it.
	for _, g := range guidesFor("https://x") {
		if g.ID == "defender" && (g.Redirect || g.Note == "") {
			t.Fatal("the Defender guide must be block-only with a note on how the offer reaches users")
		}
	}
}

func TestDefenderIndicatorsFormat(t *testing.T) {
	csv := defenderIndicators("https://sluss.example.eu")
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if !strings.HasPrefix(lines[0], "IndicatorType,IndicatorValue,ExpirationTime,Action,") || len(lines) != len(genAIDomains)+1 {
		t.Fatalf("header or row count wrong:\n%s", csv)
	}
	for _, l := range lines[1:] {
		f := strings.Split(l, ",")
		if len(f) != 12 || f[0] != "DomainName" || f[3] != "Block" || f[1] == "" {
			t.Fatalf("bad indicator row %q (%d fields)", l, len(f))
		}
	}
}
