# Kontrollrapport (compliance report, ISSUE-076)

`GET /router/compliance/report` (admin-gate: Bearer admin-nyckel eller
dashboard-lösenord) genererar en läsbar **kontrollrapport** — artefakten en
CISO lämnar till ledning eller revisor. `?format=json` ger samma data som JSON
(för BI/agenter). Rapporten byggs uteslutande från befintliga källor och rör
aldrig hot path.

```bash
curl -s -u admin:$ROUTER_DASHBOARD_PASSWORD \
  http://localhost:8080/router/compliance/report > kontrollrapport.md
```

## Sektioner

1. **Kontrollstatus** — aktiv policyversion + regelantal, deterministisk
   routing (ingen LLM i beslutet), incident-spakar (conservative mode,
   budgettak, öppna kretsar), auditkedjans status, och leverantörsinventariet
   med compliance-taggar (egress-ytan).
2. **Händelser** — totalt/blockerade requests (durabelt, ur historiken) samt
   ett auditfönster med blockeringar per orsak och PII-träffar per **typ**
   (aldrig värden). Fönstret är tydligt märkt som bundet — hela den
   verifierbara kedjan är exporten (`/router/audit/export`).
3. **Per avdelning** — requests + kostnad per tenant.
4. **Förklarbarhet och bevis** — pekar på decision reasons och den
   hash-kedjade auditloggen + `audit-verify`.
5. **Kostnad och miljö** — besparing vs all-premium + CO₂e, tydligt märkta
   som estimat.

## Språkdisciplin

Rubriken är **kontrollrapport**, aldrig "compliance-intyg". Första stycket
säger uttryckligen att den inte är ett juridiskt intyg — vi redovisar
kontroller och bevis, lagen tolkar någon annan
(se `00-product/08-positioning-ciso.md`).

Tomtillstånd är ärliga ("inga händelser ännu") — rapporten hittar aldrig på
siffror.
