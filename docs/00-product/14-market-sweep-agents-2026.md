# 14 — Marknadssvep: gateway, observability, agent control plane (2026-07-25)

> Beställt efter en partnerförfrågan från en välfinansierad enterprise-agent-
> leverantör. Två delar: (1) vad leverantören gör och om Sluss adderar värde,
> (2) svep över LLM-proxy/observability/agent-harness-marknaden. Källor listade
> sist. Samma ärlighetsregler som resten: inga compliance-löften, inga
> påhittade siffror.

## Del 1 — Leverantören: vad de gör

**Bolaget.** Grundat 2025, EU-baserat, ett par hundra miljoner dollar rest på ~1 år till en miljardvärdering (tier-1-investerare).

**Produkten.** Enterprise-agenter för **kundvända flöden** (röst, chatt,
e-post) och **interna flöden** (onboarding, compliance, IT-support). Uttalade
resultat: >80 % containment, upp till −60 % handläggningstid.

**Verticals.** Telekom, **finans**, tillverkning, **sjukvård**.

**Marknader.** 30+ länder — Italien, Schweiz, Nederländerna, Grekland, Polen,
Rumänien, Baltikum, Adriatiska, UAE; expansion Tyskland, Österrike, **Norden**,
Portugal.

**Affärsmodellen är det intressanta:** inte "sälj mjukvara och lycka till" utan
**forward-deployed, samlokaliserade team** som bygger in agenterna i kundens
miljö. De anpassar per marknad för "språk, kulturella normer **och
regulatoriska miljöer**" — pilot→produktion på dagar/veckor.

**Vad de INTE säger publikt:** vilka modeller/providers de kör, dataresidens,
säkerhetsposition, subprocessor-kedja. Inget om GDPR/NIS2/DORA i deras
kommunikation.

## Del 2 — Kan Sluss addera värde? (bedömning)

**Ja — och passformen är ovanligt exakt.** Fyra skäl:

1. **Deras verticals ÄR de reglerade.** Telekom = NIS2 essential entity. Finans
   = DORA. Sjukvård = NIS2 + GDPR art. 9. Deras agenter hanterar kund-PII i
   **massiv volym** (röst/chatt/e-post, tiotusentals ärenden/dag). Det är exakt
   den trafik Sluss är byggd för att styra och bevisa.
2. **Deras differentiator är vår produkt.** De säljer per-marknads-anpassning
   inkl. *regulatoriska miljöer* — men gör det med **människor** (lokala team).
   Regime-packs (NIS2/DORA/GDPR-sovereign) gör samma sak som **produkt**. Det
   skalar deras hyper-lokala löfte utan linjär headcount.
3. **Procurement är deras flaskhals, inte tekniken.** 88 % av enterprise-
   agentprojekt når aldrig produktion. Ett $2B-startup som säger till en nordisk
   bank "låt våra agenter prata med era kunder" möter en säkerhetsgranskning:
   *vart går prompterna, vilka subprocessorer, kan ni bevisa det?* Sluss svarar
   med **artefakter** (fail-closed egress, hash-kedjad audit, 24/72h
   incident-evidens) istället för löften. Kortare säljcykel = deras KPI.
4. **Forward-deployed = redan i kundens infra.** Deras leveransmodell placerar
   dem *inne* hos kunden — precis där en self-hosted gateway hör hemma. Sluss
   blir underlaget de kör ovanpå, inte ännu en SaaS i kedjan.

**Formen på ett samarbete** (olika saker — reda ut vilket de menar):
- **OEM/embed** — Sluss som styrskikt under deras agenter, deras varumärke.
  Störst volym, minst kontroll för oss.
- **Referens-/reseller** — de tar in Sluss när kunden kräver suveränitet.
  Lättast att börja med.
- **Design partner** — vi bygger agent-specifika kontroller mot deras verklighet.
  Störst produktvärde för oss.

**Bonus:** 80 % containment × miljontals interaktioner = verklig tokenspend.
Sluss routing (billigt→billig/lokal modell) är ren marginal för dem, och
CO₂-kvittot är ESG-material för deras enterprise-kunder.

**Risker att gå in med öppna ögon:**
- **De kan bygga själva** — en gateway är inte raketforskning. Vår moat är
  regime-packs + audit-kedjan, inte proxyn. Sälj packs och bevis, inte plumbing.
- **Asymmetri.** $2B-bolag vs solo founder. "Samarbete" kan i praktiken betyda
  acquihire, gratis integrationsarbete, eller roadmap-kapning. Bestäm i förväg
  vad som INTE är förhandlingsbart (självhostad kärna, ingen prompt-data till
  oss, packs-korpusen).
- **Fokusrisk.** En partner ska inte forma roadmapen. Men signalen — att en
  agent-leverantör i våra verticals söker upp oss — är validering av lagret.

## Del 3 — Marknadssvep

### 3a. Gateway-lagret commoditiseras nedåt

Plumbing är löst och gratis: **LiteLLM** (MIT, 100+ providers, virtuella
nycklar, budgetar), **Cloudflare AI Gateway** (gratis på edge), **Kong**,
**Bifrost** (throughput), **Martian** (modellval), **Helicone**
(observability-först). Kategorin har **fragmenterat i specialiseringar**.

**Portkey** är den närmaste konkurrenten i vår riktning: "starkaste
guardrails-storyn — PII-redaction, jailbreak-filter och audit trails i
gateway-lagret". Skillnaden kvarstår: SaaS-kontrollplan, ingen fail-closed
egress per promptklass, inga EU-regimpacks, ingen tamper-evident kedja.

**Slutsats:** att sälja "vi är en proxy" är dött. Att sälja *kontroll + bevis*
ovanpå proxyn är där värdet är — vilket är exakt vår positionering.

### 3b. Observability konsolideras in i datainfrastruktur

**Langfuse köptes av ClickHouse** (jan 2026) — infrastrukturspelare köper
telemetrilagret. **Braintrust $80M B** (feb 2026), **Arize $70M C**,
**LangSmith** (Sequoia/Benchmark/IVP). Alla tre stora har nu agent-tracing.

**Slutsats för oss:** bygg **inte** en observability-produkt — det lagret har
kapitaliserade vinnare och köps upp av datainfra. Vår audit-kedja är något
annat: *bevis för en regulator*, inte *debugging för en utvecklare*. Håll den
distinktionen skarp i pitchen (och överväg export/integration mot Langfuse/OTel
istället för att konkurrera).

### 3c. 🔥 Ny kategori: "agent gateway" som control plane

Detta är den stora nyheten sedan förra svepet (doc 11). Forbes (juli 2026):
*"Agent gateways are becoming the control plane for enterprise AI"* — ett lager
mellan agenter och de modeller/verktyg de når.

- **Nutanix Agent Gateway** (GA) — "centraliserad framdörr" mellan agenter,
  LLM:er och enterprise-verktyg; access-policies + tokenförbrukning. Kommer från
  **privat inferens/hybrid-infra** — närmast vår position av alla.
- **Arcade** — delegerad användarauktoritet, omprövad vid *varje* handling.
- **Manufact** — MCP-serverlivscykel (deploy/test/monitor).

Parallellt: **harness-tesen** — fokus flyttas från modellen till
**harnessen**: "det deterministiska runtime-lagret som validerar, auktoriserar,
exekverar och **loggar varje handling** modellen föreslår". Det är ordagrant
Sluss vokabulär, applicerad på agenter istället för prompts.

### 3d. Kapitalvågen har flyttat till agent-säkerhet

- **>$392M** i ny agentic-AI-säkerhetsfinansiering på två veckor kring RSAC
  2026; **Oasis Security $120M B** (non-human identity).
- **Check Point köpte Lakera.** Plattformsleverantörer köper MCP-säkerhet innan
  startups ens når Series A.
- MCP-säkerhet: ~$40M över Operant AI, Runlayer, Helmet Security, Manufact.
- Supply chain-risk är verklig: Snyk fann att **36,8 %** av skannade
  agent-skills hade minst en säkerhetsbrist. **0,01 %** av non-human identities
  kontrollerar 80 % av molnresurser.

### 3e. Suveränitets-medvinden är starkare än vid förra svepet

- **Sovereign cloud ~$80B 2026, +35,6 % YoY** — reglerade AI-workloads flyttar
  från amerikanska hyperscalers. GAIA-X 400+ certifierade leverantörer.
- **EU AI Act: loggningskrav för högrisk-AI träder i kraft 2 aug 2026**
  (sekundärkälla — verifiera exakt scope innan det används i säljmaterial).
  Om det stämmer är det en **daterad forcing function ~1 vecka bort**.
- Distinktionen marknaden nu artikulerar: **residens ≠ suveränitet** — data i
  Frankfurt hos en amerikansk leverantör lyder fortfarande under CLOUD Act.
  Det är precis vår "true zero trust includes your vendors"-poäng, nu som
  allmängods.
- EU-konkurrenter att bevaka: **Radicalbit AI Gateway** (orkestreringslager med
  PII-detektion/maskning före modellanrop + granulära per-provider-policies) och
  **NeuralTrust**. Närmast oss geografiskt och tematiskt.

## Del 4 — Konsekvenser för Sluss

1. **Marknaden har rört sig MOT vår tes** på sex månader: control plane +
   evidens, inte kostnadsrouting. Positioneringen (doc 08) håller — men
   **språket bör absorbera "agent gateway"/"harness"**, för det är sökordet
   köpare och investerare använder nu.
2. **Ny konkurrensbild att skriva in i decket:** Nutanix (infra-incumbent med
   agent gateway), Portkey (guardrails), Radicalbit/NeuralTrust (EU). Vår
   kombination — självhostad + deterministisk innehållsklassning + fail-closed
   per promptklass + tamper-evident + EU-regimpacks — är fortfarande osamlad
   hos andra, men marginalen krymper.
3. **Produktgap att stänga (kandidater):** agent-/verktygsnivå-governance (vilka
   MCP-verktyg en agent får anropa), per-agent identitet, och godkännandeflöden.
   Idag styr Sluss **prompt→modell**; marknaden frågar nu även **agent→verktyg**.
   Vi har redan MCP-ytan — steget är kortare för oss än för de flesta.
4. **AI Act-loggningen (2 aug 2026)** är en skarpare säljkrok än NIS2 för
   *agent*-köpare. Verifiera scope, lägg sedan in i landing + pitch.
5. **Observability: integrera, konkurrera inte.** OTel/Langfuse-export ut ur
   audit-kedjan är billigare än att bygga en tracing-produkt, och gör oss till
   en bra granne i stacken.

## Källor

- [Forbes: Agent gateways are becoming the control plane](https://www.forbes.com/sites/janakirammsv/2026/07/05/agent-gateways-are-becoming-the-control-plane-for-enterprise-ai/) ·
  [Nutanix Agent Gateway](https://www.nutanix.com/blog/introducing-nutanix-agent-gateway) ·
  [Agent harness engineering](https://medium.com/@adnanmasood/agent-harness-engineering-the-rise-of-the-ai-control-plane-938ead884b1d)
- [Agentic AI security funding/M&A (RSAC 2026)](https://softwarestrategiesblog.com/2026/03/28/agentic-ai-security-startups-funding-mna-rsac-2026/) ·
  [Snowflake: MCP governance](https://www.snowflake.com/en/blog/enterprise-ai-security-agentic-mcp-governance/) ·
  [Agent identity 2026](https://www.idenhq.com/en/blog/ai-agent-identity-management-2026)
- Gateway-landskap: [Zuplo buyer's guide](https://zuplo.com/learning-center/best-ai-gateway-buyers-guide) ·
  [LLM gateway guide](https://klymentiev.com/blog/llm-gateway-guide) ·
  [Lushbinary comparison](https://lushbinary.com/blog/ai-gateway-llm-routing-comparison-litellm-portkey-cloudflare/)
- Observability: [LangSmith vs Arize vs Braintrust](https://anudeepsri.medium.com/langsmith-vs-arize-vs-braintrust-e397e4728a76) ·
  [Aiprosol index](https://aiprosol.com/llm-observability)
- Suveränitet: [CSA: EU tech sovereignty](https://labs.cloudsecurityalliance.org/research/eu-tech-sovereignty-cloud-ai-enterprise-risk-v1-0-csa-styled/) ·
  [NeuralTrust: data sovereignty](https://neuraltrust.ai/blog/data-sovereignty-complete-guide) ·
  [Radicalbit](https://radicalbit.ai/resources/blog/digital-sovereignty-in-the-ai-era-how-european-companies-can-take-back-control/) ·
  [SoftwareSeni: DORA/NIS2/AI Act](https://www.softwareseni.com/dora-nis2-and-the-eu-ai-act-are-making-sovereign-cloud-mandatory-for-some-workloads/)
