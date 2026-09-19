# Failure modes

## Provider timeout

### Symptom

- Provider svarar inte inom timeout.
- First token latency ökar.

### Hantering

- Avbryt attempt.
- Välj fallback om före första token.
- Sänk health score.
- Logga `provider_timeout`.

## Provider rate limit

### Symptom

- 429 eller quota error.

### Hantering

- Fallback till annan provider.
- Markera rate limit i health cache.
- Respektera retry-after om tillgängligt.

## Policy reload failure

### Symptom

- Ny policy kan inte kompileras.

### Hantering

- Behåll senaste fungerande policy.
- Larma.
- Blockera aktivering.

## Modell saknar capability

### Symptom

- Request kräver tool calls eller JSON schema men vald modell stöder inte det.

### Hantering

- Capabilityfilter ska förhindra detta.
- Om det ändå händer: fallback och incidentlogg.

## Event queue backlog

### Symptom

- Loggar släpar.
- Cost dashboard blir stale.

### Hantering

- API får fortsätta om kritisk audit inte krävs.
- Skala workers.
- Sätt backpressure om backlog når risknivå.

## SQLite-volym otillgänglig eller full

### Symptom

- Durabel loggning/historik misslyckas; auth och policy fortsätter från minnet.

### Hantering

- Request-pathen påverkas inte (asynkron loggning är best-effort), men evidenskedjan får hål — larma.
- Frigör utrymme på `ROUTER_DATA_DIR`-volymen; verifiera audit-kedjan med `cmd/audit-verify` efteråt.
- Adminändringar (nycklar, regler, roster) stoppas tills volymen är skrivbar.

*(Redis/Postgres-failure-modes hör till Enterprise 2.0-topologin och är inte aktuella i dagens runtime.)*

## Router bug väljer fel modell

### Symptom

- Ökad rejection.
- Ökad under-routing.
- Policy violation.

### Hantering

- Feature flag rollback.
- Aktivera konservativ global policy.
- Kör eval regression.
- Postmortem.

## Streaming bryts

### Symptom

- Klient får partial response.

### Hantering

- Logga stream interruption.
- Fallback endast om klient stödjer restart.
- Annars returnera korrekt error.
