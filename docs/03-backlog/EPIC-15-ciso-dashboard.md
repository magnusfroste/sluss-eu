# EPIC-15: CISO-dashboard och rapportering

## Mål

Ge den ansvarige (CISO/DPO/ledning) en vy som svarar på *"har vi kontroll?"*
på 10 sekunder: vad lämnade huset, vart, varför; vad blockerades/maskerades;
per avdelning; incident-spakarnas status — plus besparingen (kr + CO2) som ROI.

## Varför

Allt Fas 4 byggde (bandit, A/B, council, circuit breaker, autopilot) är idag
osynligt i UI:t — det syns bara i headers/CLI/loggar. Och dagens dashboard
talar utvecklarspråk, inte revisorspråk. Pitchen kräver en vy för köparen.

## Scope

- "Egress & Compliance"-sektion på dashboarden: block/mask-räknare,
  residens-träffar, egress per provider/region, per avdelning (tenant)
- Incident-spakar synliga: conservative mode, öppna circuits, budget-status
- "Learning & routing"-panel: bandit-armar (bäst modell per task), A/B-armar,
  council-utfall
- Länk "generera compliance-rapport" (EPIC-12:s artefakt)
- **Synligt routningsbyte** i demo-chatten (ISSUE-083): moln → lokal när PII
  flaggas, med skäl — signaturögonblicket (`.scratch/ciso-demo/signature-moment.md`)

## Out of scope

- Nya datakällor — allt ovan finns redan i tracker/historik/headers; detta är
  presentation
- Extern BI-integration

## Acceptanskriterier

- Vyn är begriplig för icke-tekniker (revisorspråk, inte modell-slugs först).
- Tom-tillstånd är ärliga ("inget blockerat ännu") — inga fejkade siffror.
- Per-avdelningsvy fungerar med ≥2 tenants i demo.
