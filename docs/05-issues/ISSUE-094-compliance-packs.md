# ISSUE-094 — Compliance-paket (moduler per marknad/regim)

- `epic: EPIC-12`
- `priority: P1`
- `type: product`
- `sprint: backlog`
- `state: ready-for-agent`

## Bakgrund

Marknadsanalys `00-product/10-market-nis2-eu-global.md` + beslut 2026-07-09:
produkten är EU-bred/global, inte Sverige-only. Olika marknader/regimer ska kunna
**enabla olika moduler beroende på behov** (global publik vs svensk). En "modul"
(compliance-paket) = tre delar:

1. **Regelverk** = policy-mall (LEVERERAT: `builtin:nis2-baseline`,
   `builtin:dora-baseline`, `builtin:gdpr-sovereign`, `builtin:pii-local`;
   listade på Policy-sidan, väljs via `ROUTER_POLICY_PATH=builtin:<namn>`).
2. **Rapportprofil** — LEVERERAT 2026-07-09: `internal/regime` med profiler
   nis2/dora/gdpr/eu; rapporttitel, regim-rad (lag + tillsynskontext) och framing
   styrs av `ROUTER_PROFILE` eller härleds från aktiv policy-pack.
3. **Framing** — DELVIS: landningens hero-label följer profilen. Djupare
   copy-lokalisering (FAQ, connect, språk) KVAR.

## Scope (kvarvarande)

- `ROUTER_PROFILE`/UI-val som binder ihop {policy-mall + rapportprofil + framing}.
- Rapportgeneratorn (`internal/server/report.go`) tar en profil-parameter.
- Landing/connect: marknads-/regim-parametriserad framing (behåll motorn neutral).
- Ev. UI-aktivering av modul (idag kräver omstart via env).

## Icke-mål

Juridiska compliance-påståenden. LLM i routing-beslutet.

## Levererat 2026-07-09

Fyra paketmallar (regelverk-delen) + Policy-sidans modul-lista + marknadsanalys +
beslutslogg.
