# ISSUE-084: Residens-nekan ska vara en auditerad block, inte no_route

## Labels
- `epic: EPIC-13`
- `priority: P2`
- `type: backend`
- `sprint: sprint-09`
- `state: done` (2026-07-06)

## Bakgrund

Upptäckt under CISO-demon (ISSUE-082): när `require_provider_tags` gör att
**ingen** kandidat återstår returnerar motorn idag ett `no_route` (502) — samma
väg som "alla providers nere". Requesten serveras inte (fail-closed fungerar),
men händelsen **auditeras inte** som en policy-block och syns inte i
blockeringsräknaren/kontrollrapporten.

## Mål

För compliance-berättelsen bör en residens-/compliance-nekan vara en
**förstklassig, auditerad block** (403, `ErrBlocked`, block-kod t.ex.
`residency_no_compliant_provider`) — inte en generisk gateway-error — så att:

- auditkedjan får en post ("nekad: ingen provider med tag `eu-resident`"),
- kontrollrapporten räknar den som blockerad,
- decision reasons förklarar den.

## Skiss

- I filtreringen: skilj "0 kandidater för att tag-kraven uteslöt alla" från
  "0 kandidater av andra skäl". Det förra → `ErrBlocked` med residens-block-kod;
  det senare → befintligt `no_route`.
- `auditBlocked` fångar redan `ErrBlocked` → posten hash-kedjas automatiskt.

## Acceptanskriterier

- En PII/residens-policy utan compliant provider → 403 block, auditerad, syns i
  rapporten; icke-residens no-route beter sig oförändrat.
- Golden policy-case + engine-test för den nya block-koden.
