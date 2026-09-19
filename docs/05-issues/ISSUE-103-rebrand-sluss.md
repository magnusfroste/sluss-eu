# ISSUE-103 — Rebrand Tokenizer → Sluss (sluss.eu)

- category: enhancement
- state: done
- epic: EPIC-15
- priority: P1
- type: product|frontend
- sprint: 09
- klar: 2026-07-18

## Beslut

Varumärke **Sluss**, domän **sluss.eu** (registrerad 2026-07-18). Se
DECISION_LOG för namnprocessen (Tillit upptaget; fornnordiska kandidater fällda
på kollisioner; sluss vald framför slussen — obestämd form, metaforen = produkten,
ordet reser genom sluss/Schleuse/sluis/sluice).

## Levererat (denna svep)

- All användarriktad yta: landing (brandName/desc/heroSub/unlike/FAQ), alla
  admin-sidtitlar, chattens brand + "who"-etiketter, login-sidan, /connect,
  llms.txt, Anthropic-shims modellista ("Sluss (auto-route)"), MCP-namnet
  (`sluss`), `/v1/models` owned_by. Test-asserts uppdaterade.
- README (produktnamn; repo-sökvägar orörda), investerardeckets källa + ny
  genererad .pptx (SLUSS/sluss.eu), DECISION_LOG-post.

## Medvetet INTE ändrat (separata ägarbeslut)

- Go-modulsökväg + repo-namn `github.com/magnusfroste/tokenizer` (stor churn;
  kräver repo-rename — eget beslut, jfr ISSUE-061).
- DNS: tokenizer.froste.eu → sluss.eu (EasyPanel-domän + `ROUTER_PUBLIC_URL`).
- EUIPO-ansökan, defensivdomäner (.ai/slussen-varianter).
