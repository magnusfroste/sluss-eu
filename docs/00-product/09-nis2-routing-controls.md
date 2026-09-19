# NIS2 / cybersäkerhetslagen som routing-kontroller — analys & modell

> Statusregel (00-product/08): vi säljer **kontroll + bevis**, aldrig "compliance".
> Lagen är forcing function; tokenizer är verktyget. Inga juridiska påståenden.

## 1. Vad lagen faktiskt kräver — och var routing hakar i

Cybersäkerhetslagen (svensk transponering av NIS2, ikraft 2026-01) ålägger
väsentliga/viktiga entiteter riskhanteringsåtgärder med **personligt ansvar för
ledningen**. De artiklar som en LLM-router kan bära bevis för:

| NIS2-krav | Routing-kontroll i tokenizer |
|---|---|
| Riskhantering & klassning | Klassificera varje prompt (task, risk, **sensitivity**) — regelbaserat, ingen LLM i beslutet |
| Åtkomstkontroll | Per-avdelnings-nycklar (tenant/projekt) + policy som styr vem/vad som når vilken modell |
| Kryptering/dataskydd | Data-egress-kontroll: känsligt stannar lokalt/EU eller blockeras |
| Leverantörskedja | Provider-riskattribut (taggar: eu-resident, dpa-signed, iso27001…) som policy kräver/nekar |
| Incidenthantering & spårbarhet | Manipulationssäker hash-kedjad audit + kontrollrapport |
| Styrning & ansvar | Deterministiska, **förklarbara** beslut (dry-run, "vilken regel föll ut") + namngivna konsol-användare |

Kärnan: **routern är den punkt där data lämnar huset**. Att styra och bevisa det
är exakt vad lagen efterfrågar.

## 2. Vilken kontext ska vi fånga? (klassificerings-taxonomi)

Idag fångar feature-extraktorn (regelbaserat, p95 < 20 ms, ingen LLM):
`task_type`, `risk_level`, `sensitivity ∈ {none, source_code, pii, secrets_possible}`,
kod/SQL/auth/betal-nyckelord, filnamn, stack traces, tokens, tool-use.

**För NIS2-routing bör klassningen utökas** (roadmap, ISSUE-093) med fler
sensitivity-klasser och en aktörsdimension — det är "kontexten att fånga mer":

- **Datakategori (utöka `sensitivity`):**
  `financial` (PCI/betaldata) · `health` (särskild kategori GDPR) ·
  `legal` (avtal/sekretess) · `security_classified` (incident/sårbarhet) ·
  `export_controlled`. (Idag: pii, secrets_possible, source_code.)
- **Jurisdiktion:** EU vs icke-EU datasubjekt / gränsöverskridande.
- **Aktör:** avdelning/roll (tenant/projekt finns redan) — NIS2 åtkomstkontroll,
  t.ex. "juridik får inte nå us-only-providers".
- **Provider-riskattribut (taggar, finns idag):** eu-resident, dpa-signed,
  iso27001, soc2, sub-processor-status, air-gapped.
- **Ekonomi:** tokens/kostnad/budget (finns: budget caps, spend, savings).

Regelbaserat och deterministiskt — så beslutet förblir reviderbart.

## 3. "Brandväggsregler för en CISO" — modellen

Policy-motorn ÄR redan brandväggen. En regel = **villkor → åtgärd**, utvärderade
i fast ordning per regel: **block → force → constraints → hints → defaults**.
Constraints ackumuleras och är **fail-closed** (ingen tyst fallback).

```
# ungefär som iptables/brandväggsregler, men för data-egress
REGEL 1  OM sensitivity = personal_data      → KRÄV provider-tagg [local]      (annars blockera)
REGEL 2  OM sensitivity = secrets            → KRÄV [local]
REGEL 3  OM task = security_review           → KRÄV [air-gapped]               (annars blockera)
REGEL 4  OM avdelning = juridik              → NEKA [us-only]
DEFAULT  → billigaste kapabla moln (kostnad/CO2)
```

Motsvarande i vår DSL (matchbara fält: `task_type, risk_level, sensitivity,
tenant, project, prompt_tokens_gt/lt, contains_any, any_file_matches,
requires_tool_use/json_schema/vision, router_mode`; åtgärder: `block, force,
constraints{require_provider_tags, deny_provider_tags, denied_providers},
defaults, hints`):

```yaml
- id: personal_data_stays_local
  when: { sensitivity: pii }
  route: { constraints: { require_provider_tags: [local] } }
```

**Egenskaper som gör det till en trovärdig brandvägg:**
- **Fail-closed:** saknas en kompatibel provider → auditerad **403-blockering**,
  aldrig tyst moln-fallback (`residency_no_compliant_provider`).
- **Pins kan inte kringgå:** en klient som pinnar en modell blockeras ändå om den
  bryter en require-tagg.
- **Dry-run / explain:** `POST /router/decision` och MCP `route_explain` visar
  *vilken regel som föll ut och varför* innan ett enda provideranrop.
- **Bevis:** varje blockering/routning loggas i hash-kedjan → kontrollrapporten.
- **Hot-reload & rollback:** policy byts utan kod-deploy.

## 4. Hur sätter en CISO "vad som tar en intern modell"?

Tre steg, och det är avsiktligt "brandväggsenkelt":

1. **Tagga interna modeller `local`** (Providers-sidan) — on-prem/DGX = en provider
   med taggen `local` (även `on-prem`, `air-gapped`, `eu-resident`).
2. **Välj/anpassa regelverket:** `ROUTER_POLICY_PATH=builtin:nis2-baseline` ger en
   färdig NIS2-mall (personuppgift→lokal, secrets→lokal, security_review→air-gapped
   eller block, annars moln). CISO adapterar reglerna till sitt provider-set.
3. **Testa & bevisa:** dry-run en prompt → se vald modell + egress + vilken regel;
   audit-kedjan bevisar utfallet i efterhand.

## 5. Mall (levererad)

`builtin:nis2-baseline` (`internal/policy/builtin/nis2-baseline.yaml`) — inbäddad
i binären, väljs via `ROUTER_POLICY_PATH=builtin:nis2-baseline`. En startpunkt
(som en brandväggs default-ruleset) som CISO anpassar.

## 6. Gap / roadmap (ISSUE-093)

Motorn + mallen finns; det som gör det *CISO-självbetjänat* återstår:
- **Policy-adminsida** (ingen YAML): bygg regler som brandväggsregler
  (villkor → åtgärd), ordnad lista, **dry-run-ruta** ("klistra en prompt → se
  vilken regel/modell/egress"), aktivera/rulla tillbaka.
- **Rikare klassning** (avsnitt 2): financial/health/legal/security_classified +
  jurisdiktion + roll.
- **Provider-riskregister:** kurera leverantörstaggar (DPA, ISO, sub-processors)
  som del av leverantörskedje-kravet.
