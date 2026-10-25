# ISSUE-132 — Sluss Egress: AI-API-proxy via MDM

- **Epic:** EPIC-12 (compliance-yta)
- **Priority:** P1
- **State:** proposed (2026-10-25) — design i ADR-0015, beslut hos ägaren

## Mål

All AI-API-trafik från hanterade enheter passerar Sluss, även från appar som
inte går att konfigurera med base-URL. MDM:en pekar ut Sluss som proxy för
AI-leverantörernas API-värdar (PAC). Webbchattar berörs inte: de blockeras
av kundens filter och erbjuds Sluss-chatten (ISSUE-128a/131).

## Faser

1. **E1:** `CONNECT`-proxy, värdlista, avvisa allt annat, audit per
   anslutning, PAC-generator på Rollout. Ingen dekryptering.
2. **E2:** opt-in TLS-terminering med kundens namnbegränsade mellan-CA,
   mappning till `/v1`-pipelinen, klientnyckel aldrig vidare (bara
   fingeravtryck), 403 på okända sökvägar.
3. **E3:** enhetsidentitet via mTLS med MDM-cert.
4. **E4 (tillval):** DNS-läge utan PAC.

Se `docs/02-adr/ADR-0015-sluss-egress-ai-proxy-via-mdm.md` för beslut,
risker (TLS-inspektion, DPIA, MBL §11, cert-pinning) och alternativ.
