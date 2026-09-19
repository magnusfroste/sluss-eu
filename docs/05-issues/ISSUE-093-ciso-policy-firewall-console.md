# ISSUE-093 — CISO policy-konsol ("brandväggsregler") + rikare klassning

- `epic: EPIC-12`
- `priority: P1`
- `type: product`
- `sprint: backlog`
- `state: done` (klar 2026-07-12)

## Bakgrund

Analys: `00-product/09-nis2-routing-controls.md`. Policy-motorn ÄR redan
brandväggen (villkor → block/force/constraints/defaults, fail-closed) och en
NIS2-mall finns (`builtin:nis2-baseline`). Det som gör det *CISO-självbetjänat*
återstår.

## Scope

1. **Policy-adminsida (ingen YAML)** — bygg/visa regler som brandväggsregler:
   ordnad lista av `villkor → åtgärd`, aktiv policy-version, aktivera/rulla
   tillbaka. Läs först (visa den aktiva policyn läsbart), sen redigera.
2. **Dry-run-ruta** — klistra en prompt → visa vald modell, tier, egress, och
   *vilken regel som föll ut* (återanvänd `POST /router/decision` /
   `route_explain`). "Se innan du sätter."
3. **Rikare klassning** (feature-extraktorn, regelbaserat, p95 < 20 ms):
   sensitivity-klasser `financial`, `health`, `legal`, `security_classified`;
   ev. jurisdiktion + roll-dimension. Golden-cases + latency-guard.
4. **Provider-riskregister** — kurera leverantörstaggar (dpa-signed, iso27001,
   soc2, sub-processor) som del av leverantörskedje-kravet.

## Icke-mål

Juridiska compliance-påståenden (vi säljer kontroll + bevis). LLM i
routing-beslutet (fast path förblir deterministisk).

## Levererat i denna vända (2026-07-09)

- `builtin:nis2-baseline` mall + analysdoc (avsnitt 1–5). Dry-run finns redan via
  `/router/decision` och MCP `route_explain`; adminsidan (1–2) och rikare
  klassning (3) är detta issue.

## Levererat 2026-07-12 — rikare klassning (punkt 3)

- Sensitivity-klasser `financial`, `health`, `legal`, `security_classified` i
  feature-extraktorn: regelbaserat, EN + SV-termer (inkl. bestämda former för
  ASCII-svenska), precision före recall.
- Ranking: `secrets_possible` och `pii` behåller topplaceringen medvetet — en
  ny klass får aldrig tränga undan ett värde som redan driftsatta policies
  matchar (patientjournal med personnummer klassas fortsatt `pii`).
- Risk: `security_classified` → high; övriga → medium.
- Policy-vokabulären utökad; paketen uppdaterade (version `*_2026_07`):
  nis2-baseline håller alla fyra klasser lokalt, dora-baseline financial/legal/
  classified, gdpr-sovereign health → eu-resident. Gap-rapporten räknar de nya
  klasserna som känsliga (`sensitive_other`).
- Golden-cases (EN+SV + ranking-invarianter) + policy-pack-test; verifierat
  live i dry-run (patientjournal → health → fail-closed block med NIS2-paketet).

## Kvar (öppet i detta issue)

- Punkt 1: regelredigering utan YAML (adminsidan visar redan aktiv policy läsbart).
- Punkt 4: provider-riskregister (kurerade taggar dpa-signed/iso27001/soc2).

## Levererat 2026-07-12 — regelredigering utan YAML (punkt 1)

- "Rules — no YAML"-kort i policy-konsolen: villkor (sensitivity-klasser,
  task type, risknivå) → åtgärd (require provider tags / block fail-closed),
  ordnad lista, ta bort per rad.
- **Activate** hot-laddar regelverket (ingen omstart) som `pv_console_NNN`;
  **Roll back to baseline** återgår till env-konfigurerad policy
  (`ROUTER_POLICY_PATH`). Båda auditeras (`policy.console`, med inloggad aktör).
- Redigering av ett AKTIVT regelverk återaktiverar direkt (ny version) så
  tabellen aldrig ljuger. Regler genereras till samma YAML-dialekt som
  filbaserade policies → identisk validering; all input vitlistas (inga
  användarstyrda strängar i YAML-position).
- Regelverket lagras i SQLite och **återaktiveras vid boot** — CISO:ns regler
  överlever redeploy; vid fel faller bootn tillbaka till baseline (aldrig till
  ett ovaliderat regelverk).
- Verifierat live: add → activate → dry-run block (`console_rule_block`) →
  rollback → baseline; omstart → `console policy re-activated from db`.

## Levererat 2026-07-12 — provider-riskregister (punkt 4)

- "Risk register — supply chain"-kort på Providers-sidan: kurerad vokabulär
  (local, air-gapped, eu-resident, dpa-signed, iso27001, soc2, no-train,
  subprocessors-vetted) med beskrivningar — fakta OPERATÖREN intygar, aldrig
  något routern antar.
- Matris providers × taggar med kryssrutor (rostern = källa, redigerbar per
  rad, fritext för extra taggar); tagg-ändringar auditeras (`provider.tags`,
  med inloggad aktör) och appliceras på omstart.
- Kolumner som aktiv policy KRÄVER markeras "required"
  (`CompiledPolicy.RequiredProviderTags`); saknas en krävd tagg hos alla
  providers visas fail-closed-varningen — gapet som annars bara syns som
  blockerade requests.
- Verifierat live: NIS2-paket + otaggade providers → varning "requires
  air-gapped, local"; taggning av openrouter → varningen krymper till
  air-gapped, audit-post med aktör.

ISSUE-093 är därmed komplett: analys + mallar, policy-konsol med dry-run,
rikare klassning, regelredigering utan YAML och provider-riskregister.
