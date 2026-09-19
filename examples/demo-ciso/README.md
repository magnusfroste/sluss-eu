# CISO pitch demo (ISSUE-082)

Ett reproducerbart demo som visar hela värdekedjan på några minuter — mot
mock-providern, utan credentials. **All data är syntetisk** (personnumret är
påhittat men Luhn-giltigt, inte en riktig person).

```bash
make demo-ciso        # eller: scripts/demo-ciso.sh
```

Positionering: **compliance/NIS2 som spjutspets, kostnad/CO2 som ROI-bevis.** Vi
säljer kontroll + bevis — aldrig "compliance-intyg". Se
`docs/00-product/08-positioning-ciso.md`.

## Vad demon bevisar (i ordning)

| # | Steg | Vad publiken ser |
|---|---|---|
| 1 | **Provisioning** | en API-nyckel per avdelning (ekonomi, utveckling) — visas en gång, hash i vila |
| 2 | **Moln när det är ofarligt** | vanlig prompt → molnmodell (`X-Router-Egress: cloud`) |
| 3 | **★ Lokal när det inte är det** | prompt med personnummer → PII upptäcks → routas till **lokal** modell; bytet syns (`X-Router-Egress: local` + skäl) — signaturögonblicket |
| 4 | **Fail-closed** | säkerhetsgranskning kräver air-gapped modell som ingen har → **blockerat**, aldrig tyst fallback till moln |
| 5 | **Beviset** | exportera hash-kedjad audit → `audit-verify` OK → ändra en byte → `CHAIN BROKEN` |
| 6 | **Kontrollrapporten** | genererad artefakt: egress-yta med compliance-taggar, nyckelhändelser, ROI — aldrig PII-värden |
| 7 | **ROI** | besparing vs all-premium (+ CO₂e), tydligt märkt estimat |

## Talepunkter

- *"Molnets kraft när det är ofarligt, husets trygghet när det inte är det —
  automatiskt, och ni ser exakt när och varför."*
- *"Beslutet fattas deterministiskt, utan LLM — varje val är loggat och
  förklarbart. En proxy som låter en LLM avgöra vart data går är oreviderbar."*
- *"Det här kan ni lämna till tillsynen — och de kan bevisa själva att inget
  rörts."*
- Avsluta med ROI: *"och den betalar sig själv."*

## Byggstenar (sprint-09)

PII-detektor (#77) · residens-/compliance-taggar (#78) · audit-kedja + verify
(#75) · kontrollrapport (#76) · DB-nycklar + Keys-sida (#79/#80) · dashboard-
sektioner (#81) · synligt byte (#83). Demon (`#82`) knyter ihop dem.

## Mot riktiga modeller (och din privata endpoint)

Allt utom själva hemligheten bor i **DB:n (rostern)** — inte i env. Minimera
env i t.ex. Easypanel:

1. **Providers-sidan** (`/router/providers`): lägg till din privata endpoint —
   base_url `https://dgx1.privai.se/v1`, nyckelns env-namn `PRIVAI_API_KEY`,
   compliance-taggar `private, eu-resident`. Sparas i DB.
2. **Models-sidan** (`/router/models`): lägg till en modell på den providern och
   sätt tier. Sparas i DB.
3. **Easypanel-env:** bara `PRIVAI_API_KEY=<hemlighet>` — inget annat.

Då taggas endpointen `private` (en igenkänd "stannar i huset"-tagg vid sidan av
`local`/`on-prem`/`air-gapped`), policyn `require_provider_tags: [private]`
routar PII dit, och bytet blir grönt i chatten.

**Varför bara hemligheten i env:** DB:n lagrar *tenant*-nycklar (klienter som
ringer oss) som **hashar** — en DB-dump avslöjar inget användbart. En
*upstream*-hemlighet (nyckeln vi använder för att ringa DGX/OpenRouter) måste
ligga i klartext för att fungera, så den hör hemma i env (eller en secrets-
manager) refererad **vid namn** — aldrig i DB:n. Det är CISO-posturen: config i
DB, hemligheter utanför. `ROUTER_PROVIDER_TAGS` är bara en demo-genväg för de
inbyggda providrarna; för skarp drift, tagga i rostern.

## Filer

- `policy.yaml` — demopolicyn (PII → local, security_review → air-gapped/block).
- `../../scripts/demo-ciso.sh` — körschemat med assertions (grönt i CI).
