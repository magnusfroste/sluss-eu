# Runbook

## Provider error rate ökar

1. Kontrollera dashboard för provider error rate.
2. Kontrollera om felet är rate limit, timeout eller 5xx.
3. Sätt provider health till degraded om automatiken inte gjort det.
4. Kontrollera att fallback fungerar.
5. Informera användare om påverkan vid behov.
6. Efter incident: skapa regressionstest om routing påverkades.

## Audit-export till revisor/tillsyn

Med `ROUTER_DATA_DIR` satt skrivs varje auditpost även till en
**hash-kedjad** fil (`audit-chain.jsonl`) — manipulationssäker i meningen att
varje ändrad/borttagen/insatt rad bryter kedjan verifierbart (ISSUE-075).

1. Exportera: `GET /router/audit/export` (admin-gate: Bearer admin-nyckel
   eller dashboard-lösenord) → laddar ner `audit-chain.jsonl`.
2. Verifiera offline: `go build ./cmd/audit-verify && ./audit-verify audit-chain.jsonl`
   → `OK — N record(s), chain intact` eller `CHAIN BROKEN` med radnummer,
   exit 1. Mottagaren (revisorn) kan köra verifieringen själv.
3. Kedjan innehåller aldrig promptar eller nyckelvärden — endast
   identifierare, skäl och typ+antal (t.ex. `pii_types`).
4. Startar routern om fortsätter kedjan där den slutade (en kontinuerlig
   kedja). Är filen korrupt vägrar routern förlänga den — flytta undan filen
   för att börja en ny kedja och spara den gamla som bevis.

## Provider circuit breaker

Efter `ROUTER_CIRCUIT_BREAKER_THRESHOLD` (default 5) **konsekutiva** fel öppnas
providerns circuit: den utesluts ur routing och trafik faller över till en frisk
provider under `ROUTER_CIRCUIT_BREAKER_COOLDOWN_SECONDS` (default 30 s). Efter
cooldown halvöppnas den — nästa anrop är en probe; lyckas det stängs circuiten,
misslyckas det öppnas den igen för en ny cooldown. En öppen circuit visas som
health `0.00` på dashboarden.

1. Kontrollera dashboard: provider med health `0.00` = öppen circuit (eller helt nere).
2. Är avbrottet väntat (planerat providerunderhåll)? Låt breakern hålla trafiken borta.
3. Är det en falsk trip (transienta fel)? Höj `ROUTER_CIRCUIT_BREAKER_THRESHOLD`
   eller korta `ROUTER_CIRCUIT_BREAKER_COOLDOWN_SECONDS` och starta om routern.
4. Vill du inaktivera breakern helt under felsökning? Sätt threshold till `0`
   (endast rullande health-score kvarstår).
5. Verifiera att fallback landar på en provider med tillräcklig tier för uppgiften.

## Router latency p95 över 100 ms

1. Kontrollera traces för var tid spenderas.
2. Leta efter DB reads i fast path.
3. Kontrollera event queue backlog.
4. Kontrollera policy reload eller registry lock contention.
5. Skala API-noder om CPU-bound.
6. Rollbacka senaste change om latency ökade efter release.

## Fel modell väljs

1. Hämta request id.
2. Läs JobDescriptor och matched policy rules.
3. Kontrollera policyversion och registryversion.
4. Kör `/router/decision` dry-run med samma input.
5. Lägg till regressionstest.
6. Uppdatera policy eller classifier.

## Budgetincident

1. Identifiera tenant/project.
2. Kontrollera spend per modell och taskklass.
3. Kontrollera om fallback/retry loop orsakat kostnad.
4. Aktivera budget cap eller conservative downgrade för låg-risk tasks.
5. Informera admin.

## Missklassificering / osäker routing under incident

Om classifiern routar osäkra (låg-confidence) eller okända tasks för billigt under
en incident: aktivera **global conservative mode** (ISSUE-060).

1. Sätt feature-flaggan `ROUTER_CONSERVATIVE_MODE=true` (eller anropa
   `Engine.SetConservative(true)` i drift) för att höja golvet.
2. Effekt: okända/låg-confidence klassificeringar routas till minst `balanced`
   (policy- och task-forcing till `premium` gäller fortfarande — flaggan sänker
   aldrig säkerhetsnivån).
3. Verifiera via `/router/decision` att osäkra prompts nu väljer ≥ balanced.
4. Stäng av flaggan när classifiern/incidenten är åtgärdad.

## Policy reload failure

1. Kontrollera policy validator logs.
2. Behåll senaste fungerande policy.
3. Kör policy tests lokalt/staging.
4. Aktivera korrigerad policy.
5. Logga audit event.
