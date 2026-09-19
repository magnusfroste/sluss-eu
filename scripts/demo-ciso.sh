#!/usr/bin/env bash
# CISO pitch demo (ISSUE-082) — the whole value chain end-to-end against the
# mock provider, deterministic and credential-free (like scripts/smoke.sh).
#
# Story it proves, in order:
#   1. Provisioning      — mint one API key per department (ekonomi, utveckling).
#   2. Cloud when safe    — an ordinary prompt routes to a cloud model.
#   3. Local when not     — a prompt containing a (synthetic) personnummer is
#                           detected as PII and routed to a LOCAL model; the
#                           switch is visible (X-Router-Egress: local). ★ signature
#   4. Fail-closed        — a security review requires an air-gapped model nobody
#                           has → blocked, never a silent fallback to cloud.
#   5. Evidence           — export the tamper-evident audit chain, verify it,
#                           change one byte, watch verification fail.
#   6. Report             — generate the control report (kontrollrapport).
#   7. ROI                — savings vs all-premium on the dashboard.
#
# ALL demo data is synthetic. The personnummer is fake (Luhn-valid, not a real
# person). Usage: make demo-ciso   (or: scripts/demo-ciso.sh)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

KEY="demo_admin_key"                 # bootstrap admin key (env), gates the control plane
MOCK_ADDR=":18097"; MOCK_URL="http://localhost:18097"
ROUTER_ADDR=":8097"; BASE="http://localhost:8097"
DATA_DIR="$(mktemp -d)"
PN="811218-9876"                     # SYNTHETIC personnummer (fake, Luhn-valid)

pass() { echo "  ok: $1"; }
fail() { echo "DEMO FAIL: $1" >&2; exit 1; }
say()  { echo; echo "── $1"; }

echo "building binaries..."
go build -o bin/mock-provider ./cmd/mock-provider
go build -o bin/router ./cmd/router
go build -o bin/audit-verify ./cmd/audit-verify

MOCK_PROVIDER_ADDR="$MOCK_ADDR" ./bin/mock-provider >/tmp/demo-mock.log 2>&1 &
MOCK_PID=$!
LOCAL_API_KEY="$KEY" MOCK_PROVIDER_URL="$MOCK_URL" ROUTER_ADDR="$ROUTER_ADDR" \
  ROUTER_DATA_DIR="$DATA_DIR" \
  ROUTER_POLICY_PATH="examples/demo-ciso/policy.yaml" \
  ROUTER_PROVIDER_TAGS="anthropic=local,eu-resident;openai=cloud" \
  ./bin/router >/tmp/demo-router.log 2>&1 &
ROUTER_PID=$!
cleanup() { kill "$MOCK_PID" "$ROUTER_PID" 2>/dev/null || true; rm -rf "$DATA_DIR"; }
trap cleanup EXIT

echo "waiting for router..."
ready=""
for _ in $(seq 1 50); do
  if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.2
done
[ -n "$ready" ] || fail "router did not become ready (see /tmp/demo-router.log)"

admin=(-H "Authorization: Bearer $KEY")

# --- helpers -------------------------------------------------------------
# mint_key <tenant> -> prints the plaintext key (tk_...)
mint_key() {
  local resp
  resp=$(curl -s "${admin[@]}" -X POST "$BASE/router/keys/mint" \
    -d "{\"tenant_id\":\"$1\",\"project_id\":\"prod\"}")
  echo "$resp" | grep -o '"key":"[^"]*"' | head -1 | sed 's/"key":"//;s/"$//'
}
# chat <key> <content> <stream> -> writes headers to /tmp/demo-h, body to /tmp/demo-b; prints HTTP code
chat() {
  curl -s -o /tmp/demo-b -D /tmp/demo-h -w '%{http_code}' \
    -H "Authorization: Bearer $1" -H "Content-Type: application/json" \
    -X POST "$BASE/v1/chat/completions" \
    -d "{\"model\":\"auto\",\"stream\":false,\"messages\":[{\"role\":\"user\",\"content\":$(printf '%s' "$2" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')}]}"
}
hdr() { grep -i "^$1:" /tmp/demo-h | head -1 | sed "s/^$1: *//I" | tr -d '\r'; }

# --- 1. Provisioning: one key per department ----------------------------
say "1. Provisioning — en nyckel per avdelning"
EKO_KEY=$(mint_key "ekonomi");  [ -n "$EKO_KEY" ] || fail "mint ekonomi key"
UTV_KEY=$(mint_key "utveckling"); [ -n "$UTV_KEY" ] || fail "mint utveckling key"
pass "mintade nycklar för ekonomi + utveckling (visas en gång, hash i vila)"

# --- 2. Cloud when it's safe --------------------------------------------
say "2. Molnets kraft när det är ofarligt"
code=$(chat "$UTV_KEY" "write a concise git commit message for a bugfix" false)
[ "$code" = "200" ] || fail "ordinary prompt should route (got $code)"
[ "$(hdr X-Router-Egress)" = "cloud" ] || fail "ordinary prompt should egress cloud (got '$(hdr X-Router-Egress)')"
pass "vanlig prompt → moln (X-Router-Egress: cloud)"

# --- 3. ★ Signature moment: local when it's not -------------------------
say "3. ★ Husets trygghet när det inte är det (moln → lokal)"
code=$(chat "$EKO_KEY" "Sammanfatta ärendet för kund $PN kortfattat" false)
[ "$code" = "200" ] || fail "PII prompt should be served locally (got $code)"
[ "$(hdr X-Router-Egress)" = "local" ] || fail "PII prompt must egress LOCAL (got '$(hdr X-Router-Egress)')"
reason="$(hdr X-Router-Route-Reason)"
[ -n "$reason" ] || fail "PII prompt should carry a route reason"
# The route-reason header is an ASCII machine code ("pii" / "pii:<types>") — HTTP
# headers aren't UTF-8 safe, so the localized sentence is rendered client-side.
case "$reason" in pii*) : ;; *) fail "reason should be a pii code (got '$reason')";; esac
pass "personnummer upptäckt → LOKAL modell, datan lämnade aldrig huset (skäl: $reason)"

# --- 4. Fail-closed ------------------------------------------------------
say "4. Fail-closed — inget läcker till moln"
code=$(chat "$EKO_KEY" "security review this internal login flow for auth vulnerabilities and secret leakage" false)
[ "$code" != "200" ] || fail "security review (air-gapped required) must NOT be served (got 200)"
pass "säkerhetsgranskning kräver air-gapped → blockerat ($code), aldrig tyst fallback till moln"

# --- 5. Evidence: tamper-evident audit chain ----------------------------
say "5. Beviset — manipulationssäker audit-kedja"
curl -s "${admin[@]}" "$BASE/router/audit/export" -o "$DATA_DIR/export.jsonl"
[ -s "$DATA_DIR/export.jsonl" ] || fail "audit export empty"
./bin/audit-verify "$DATA_DIR/export.jsonl" >/dev/null || fail "fresh chain should verify"
pass "audit-verify: kedjan intakt"
sed -i 's/ekonomi/ekonomiX/' "$DATA_DIR/export.jsonl"   # tamper one field
if ./bin/audit-verify "$DATA_DIR/export.jsonl" >/dev/null 2>&1; then
  fail "tampered chain must fail verification"
fi
pass "en byte ändrad → audit-verify: CHAIN BROKEN (revisorn kan bevisa det själv)"

# --- 6. Control report ---------------------------------------------------
say "6. Kontrollrapporten"
curl -s "${admin[@]}" "$BASE/router/compliance/report" -o "$DATA_DIR/report.md"
grep -q "Control report" "$DATA_DIR/report.md" || fail "report missing title"
grep -qi "eu-resident" "$DATA_DIR/report.md" || fail "report should show the egress surface with compliance tags"
grep -qi "not a legal attestation" "$DATA_DIR/report.md" || fail "report must keep the not-a-legal-certificate disclaimer"
grep -q "811218" "$DATA_DIR/report.md" && fail "report leaked a PII value!"
pass "rapport genererad (egress-yta med taggar; nyckelhändelser; aldrig PII-värden)"

# --- 7. ROI --------------------------------------------------------------
say "7. ROI — kontrollen betalar sig själv"
curl -s "${admin[@]}" "$BASE/router/dashboard/data" -o "$DATA_DIR/dash.json"
grep -q '"egress_compliance"' "$DATA_DIR/dash.json" || fail "dashboard missing egress section"
python3 - "$DATA_DIR/dash.json" <<'PY' || true
import json,sys
d=json.load(open(sys.argv[1]))
s=d.get("savings",{})
t=d.get("total_requests",0)
print(f"  ok: {t} requests · besparing ${s.get('saved_usd',0):.6f} ({s.get('saved_pct',0):.0f}%) vs all-premium")
PY

echo; echo "DEMO OK — hela värdekedjan grön mot mock (blockering → moln→lokal → audit → rapport → ROI)."
