# Positionering: LLM-egress-kontroll för CISO (NIS2)

Beslutad 2026-07-05 (se DECISION_LOG). Detta dokument är strategins minne och
styr feature-prioritering: **nästa feature är den som stänger ett
compliance-gap, inte den som är roligast att koda.**

## Tesen

Vi konkurrerar inte som "ännu en LLM-proxy" — LiteLLM och dussintals gratis
proxies äger redan "unified API + routing + cost tracking", och det spåret är
ett race to the bottom. Vi säljer en **tvingande köp-anledning**:

> **LLM-egress-kontroll för företag under NIS2. Bevisa vart varje prompt tog
> vägen, blockera känslig data från fel modeller/leverantörer, och kapa
> kostnad + CO2 på köpet — med en revisionslogg du kan lämna till tillsynen.**

- **Köparen** är CISO/DPO/ledning — inte utvecklaren. NIS2 (art. 20/21) och
  cybersäkerhetslagen (2026-01) lägger **personligt ansvar** på ledningsorganet
  för riskhanteringsåtgärder. Det gör köpet icke-diskretionärt: det finns en
  lag, en deadline och en namngiven ansvarig.
- **Kostnad + CO2** ("token kill bill") är ROI-förstärkaren som tar bort
  invändningen — *"och den betalar sig själv"* — men aldrig rubriken.
- **Per-avdelnings-nycklar** (tenants) är accountability-enheten: uppföljning,
  statistik och säkerhet per avdelning. (Se `.scratch/api-key-provisioning/`.)

## Signaturögonblicket: "molnets kraft när det är ofarligt, husets trygghet när det inte är det"

Det känslomässiga demo-ögonblicket (ägarreflektion,
`.scratch/ciso-demo/signature-moment.md`): default kör en **extern molnmodell**
(billig/snabb); när något flaggas i kontexten (PII/personnummer/känsligt) routar
vi till en **lokal/on-prem-modell** så datan inte lämnar huset — **och bytet
syns**, med skäl, i chatten. Automatiskt, och köparen ser exakt *när* och
*varför*. Mekaniken är redan byggd (PII-detektion #77 + residens-taggar #78; en
lokal modell = en provider med taggen `local`); det som återstår är att
synliggöra bytet (ISSUE-083) och väva in det i pitchdemon (ISSUE-082). Elegant
just för att det inte är en ny motor — det är *synliggörandet* av egress-
kontrollen vi redan har.

## Varför vi kan vinna just den här wedgen

Routerns designprinciper råkar vara exakt vad en revisor vill se:

- **Ingen LLM i routingbeslutet** → varje beslut är deterministiskt, loggat och
  förklarbart. En proxy som låter en LLM avgöra vart data går är i praktiken
  oreviderbar. Vår latency-princip blev en compliance-USP.
- Policy → block/force/constraints per task/risk/känslighet = egress-policy.
- Audit, masking, budget/kill-switchar, shadow/A-B-bevis finns redan.

## Zero Trust för AI — och vändningen mot leverantören (USP, 2026-07-17)

Zero Trust (NIST 800-207) är en **budgetkategori** hos CISO:n — hylla, språk
och pengar finns redan. Våra mekanismer mappar exakt på principerna, så vi
lånar vokabulären som *beskrivning* (aldrig som brand — termen är sliten):

| Zero Trust-princip | Vad vi redan gör |
|---|---|
| **Verify explicitly** | Varje prompt klassas deterministiskt per request — ingen stående tillit till användare, uppgift eller modell |
| **Least privilege** | Billigaste/mest lokala modellen som klarar uppgiften; residens-taggar begränsar vart data *får* gå |
| **Assume breach** | Fail-closed (aldrig tyst fallback) + manipulationssäker audit — byggt för att bevisa efteråt |

Säljraden: **"Zero Trust, applied to every prompt: no prompt trusted until
classified, no model trusted until policy allows it, no decision trusted until
logged."**

**Vändningen — vår viktigaste USP-formulering:** alla SaaS-konkurrenters zero
trust (Cloudflare AI Gateway, nexos.ai, hela SSE-fältet) har ett blint fläck —
kunden måste lita på *dem*; all trafik transiterar deras (amerikanska) moln.
Vi driver logiken ett steg längre än deras affärsmodell tillåter dem att följa:

> **"True zero trust includes your vendors. The gateway runs in *your*
> infrastructure — we never see a prompt."**

Används i varje jämförelse mot SaaS-gateways och i investerardecket.

**GTM-mekanik lånad från Cloudflare** (utförande, inte arkitektur):
1. **Mognadstrappan som resa**: observera (monitor mode) → coacha
   (gap-rapporten) → enforcea (aktivera paketet). Rita den som en onboarding-
   resa i pitch och konsol — trappan finns redan i produkten.
2. **Platt produktfamilje-namngivning** (Access/Gateway/Tunnel-stilen): när
   varumärket är valt döps modulerna funktionellt — *X Gateway, X Evidence,
   X Monitor*.
3. **"Gratis att mäta, betala för att enforcea + bevisa"** som prisprincip —
   monitor mode är den riskfria landningen, gap-rapporten säljer uppgraderingen.

## Trovärdighetsregel

Vi positionerar oss som **"kontroll + bevis"** — aldrig "vi gör dig compliant".
Lagen är forcing function; vi är verktyget. Inga juridiska utfästelser.

## NIS2-förmågekarta (har vi / gap)

| CISO-behov (NIS2-språk) | Vad vi har idag | Gap → epic |
|---|---|---|
| Data-egress-kontroll (vart får data gå?) | Policy engine: block/force, allowed/denied providers & models, capability-deny | Residens-/compliance-taggar per provider (EU-only, DPA-godkänd) som policyvillkor → **EPIC-13** |
| DLP / PII-skydd innan data lämnar huset | Secrets-masking, sensitivity-klassning, intent-eskalering | Riktig PII-detektering (personnummer, e-post, IBAN, kort) + eskalering → **EPIC-13** |
| Bevisbar spårbarhet för tillsyn | Audit-sink (beslut, block, nyckelhändelser), request-logg utan råprompts, decision reasons | Manipulationssäker (hash-kedjad) + exporterbar logg, compliance-rapport → **EPIC-12** |
| Förklarbara beslut (ej black-box) | Deterministisk routing, `route_explain` (MCP), decision reasons, shadow-diff | Paketeras i rapport/dashboard för icke-tekniker → **EPIC-12/15** |
| Incident-spakar / kill-switch | Conservative mode, circuit breaker, budget-block, policy-hotreload | Synliggöra spakar + status i CISO-vy → **EPIC-15** |
| Accountability per enhet | Tenants/projekt, scopes/roller, budget per tenant, spend-by-tenant | Nyckel-provisioning i DB (mint/rotera/återkalla, hash i vila) → **EPIC-14** |
| Leverantörsstyrning | Roster i SQLite, prissync, health/circuit per provider | Compliance-metadata på provider (region, DPA, certifieringar) → **EPIC-13** |
| ESG/CSRD-rapportering | Green receipt (Wh/CO2e-estimat vs all-premium) | Exporterbar CSRD-siffra i compliance-rapporten → **EPIC-12** (P2) |
| Kostnadskontroll ("token kill bill") | Realized cost, savings-baseline, budgetar, eval-driven/bandit-routing, autopilot | Per-nyckel-budget/rate limit → **EPIC-14** (P2) |

## Milstolpe

**Sprint 09 = "CISO pitch MVP"** (`04-sprints/sprint-09-ciso-pitch.md`):
en demo där ett läckageförsök blockeras live, revisionsloggen exporteras och
verifieras, egress-kartan visas per avdelning, och besparingen (kr + CO2)
landar som avslut. Allt byggt genom att **återanvända** befintliga subsystem.
