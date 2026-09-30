# CISO-demo: manus med personas (verifierat 2026-07-14)

Åtta "testskott" spelade mot en exakt spegel av prod-setupen (NIS2-paketet +
local-taggad DGX). Varje prompt är copy-paste-klar; utfallen nedan är **körda,
inte gissade**. Dramaturgin: *billigt moln när datan är ofarlig → huset när den
inte är det → fail-closed när inget duger → beviset på allt.*

## FÖRE MÖTET — checklista (ordningen spelar roll)

*(Verifierat 2026-07-14: live kör redan senaste builden — nya landingen,
konsolen, Test-knappen och incident-rapporten är på plats. Det som återstår
är konfiguration, inte deploy.)*

1. **Fixa glm-slugen**: Models-sidan → `glm-premium` → slug `zai/glm-5.2` →
   **`glm-5.2`** → Save → redeploy. Verifiera med **Test-knappen** på raden
   (grön ✅). Den är din enda premium-modell — trasig slug = kraschad demo.
2. **Re-tiera qwen36-35b → `balanced`** och sätt pris **in 1.0 / ut 4.0**
   $/Mtok (idag: cheap + pris 0 — då vinner den ALLT inkl. trivialt och
   moln-beatet försvinner; som `cheap` blockeras hälsa/juridik i stället).
   Save → redeploy.
3. **Byt policy**: `ROUTER_POLICY_PATH=builtin:nis2-baseline` (idag kör live
   `pii-local`, 2 regler — beats 3/4/7 kräver NIS2-paketets regel för de nya
   klasserna så att det är REGELN som driver lokalt, inte prisslumpen).
4. Verifiera att **dgx**-providern bär taggarna `local, on-prem, eu-resident`
   (riskregistret på Providers-sidan — kolumnen "local" ska vara ✓ och
   "required"-flaggad).
5. **Reset demo data** (Policy-sidan) så dashboarden startar ren — reseten
   hamnar själv i audit-kedjan, vilket är en poäng i sig.
6. **Kör alla åtta prompts nedan i dry-run** (Policy-sidan) och bocka av att
   utfallen matchar. Tar 3 minuter.
7. Skapa en **namngiven demolänk** för CISO:n (Demo prompts-sidan) — då ser du
   efteråt om hen gått in igen.

## Manuset — 8 beats à ~1 min

> Öppna med landningssidan (30 sek): *"Say yes to AI — on your rules."*
> Sedan chatten (admins kontrollpanel) och kör beats 1–5 där; 6–8 i konsolen.

### Beat 1 — Anna, marknad (det ofarliga → billigaste molnet)
```
write a concise git commit message for a bugfix
```
**Utfall (live):** `glm-premium` · moln (scoring valde bästa värdet — glm är
prissatt lägre än gpt-4o).
**Säg:** "Ofarlig data → molnet, bästa värde per krona. Det är CO2- och
kostnadsmotorn — mätt mot en allt-på-premium-baseline på dashboarden."

### Beat 2 — Johan, kundtjänst (signaturögonblicket: PII → huset)
```
Summarise the case for customer 811218-9876 who complained about an invoice
```
**Utfall:** `qwen36-35b` (balanced) · **LOKAL** · sens `pii` · grön banner
*"the data never left the house"*.
**Säg:** "Personnumret upptäcktes i innehållet — regelbaserat, under en
millisekund, ingen LLM i beslutet. Prompten gick till er egen GPU."

### Beat 3 — Sara, HR (hälsodata — GDPR art. 9)
```
Summarise the patient journal and the sick leave certificate before the insurance meeting
```
**Utfall:** `qwen36-35b` · **LOKAL** · sens `health`.
**Säg:** "Ingen personuppgift i texten — men klassen *hälsodata* räcker.
Specialkategorierna har egna brandväggsregler."

### Beat 4 — Peter, CFO (finansdata, på svenska)
```
Sammanfatta bokslutet och lönelistan inför styrelsemötet
```
**Utfall:** `qwen36-35b` · **LOKAL** · sens `financial`.
**Säg:** "Klassificeraren läser svenska. Bokslut och lönelistor lämnar aldrig
huset."

### Beat 5 — Elin, utvecklare (svår uppgift — fortfarande hemma)
```
debug this race condition deadlock in my concurrent Go code, goroutines hang on a mutex
```
**Utfall (live):** `glm-premium` · **moln** · sens `source_code` — för att
DINA regler tillåter det (källkod är inte med i regel 1).
**Säg:** "Svår uppgift → starkaste modellen policyn tillåter." **Interaktivt
moment:** om CISO:n reagerar — "ska koden verkligen ut?" — lägg till
`source_code` i regel 1 i konsolen INFÖR hen, aktivera, kör om prompten →
lokal. Regeländring på 20 sekunder, utan YAML, utan omstart. Starkaste beatet
i hela demon om det uppstår.

### Beat 6 — Marcus, säkerhet (fail-closed — ingen tyst fallback)
```
security review our SSO login flow for auth bypass and secret leakage
```
**Utfall:** **BLOCKED (fail-closed)** · `residency_no_compliant_provider` ·
task `security_review`.
**Säg:** "NIS2-paketet kräver air-gapped för säkerhetsgranskningar — ni har
ingen sådan modell, alltså blockeras det. **Aldrig** en tyst fallback till
molnet. Och blocken är i sig audit-evidens. Vill ni ändra regeln gör ni det
själva i konsolen — utan YAML, utan omstart." *(Visa Rules-kortet + dry-run,
men aktivera inget mitt i demon.)*

### Beat 7 — Lisa, juridik (avtal → huset)
```
Summarise this NDA and the settlement agreement and flag the risks
```
**Utfall:** `qwen36-35b` · **LOKAL** · sens `legal`.

### Beat 8 — "men om någon försöker runda?" (pin ≠ kryphål)
I chatten: välj **cheap-general** i modellväljaren (pinna molnmodellen), kör
beat 2-prompten igen.
**Utfall:** **403 — auditerad block.** Policyn står över användarens val.
**Säg:** "Även den som uttryckligen väljer en molnmodell kommer inte förbi
brandväggen."

### Beat 9 — Agenten (nya kapitlet: kapability, inte bara innehåll)

> **Förutsätter:** deploy med ISSUE-114 (sep 2026). Kör beatet i policy-
> konsolen utan att byta pack: skapa regeln live INFÖR betraktaren —
> "When agent capability: destructive → Route local", aktivera (bytet
> auditeras — poäng i sig). Alternativ: aktivera `builtin:nis2-baseline`
> (regel 4d gör samma sak) och växla tillbaka efteråt.

**Säg först:** "Allt ni sett hittills styr på *innehåll*. Men era agenter har
snart verktyg — och då är frågan inte bara vad prompten säger, utan vad agenten
*kan göra*. Titta här: prompten är helt ofarlig."

Dry-run (Policy-sidan eller curl mot `/router/decision`) — ofarlig prompt, men
agenten deklarerar ett delete-verktyg:

```json
{"messages":[{"role":"user","content":"clean up stale test accounts from the CRM"}],
 "tools":[
   {"type":"function","function":{"name":"search_accounts","description":"Find accounts"}},
   {"type":"function","function":{"name":"delete_account","description":"Permanently remove an account"}}]}
```

**Utfall:** tvingas **LOKAL** (regel `agent_high_capability_stays_local`) —
`tool_risk=destructive`, trots `sensitivity=none`. Kör sedan samma prompt
**utan** tools-fältet → molnet, som vanligt.
**Säg:** "Sluss klassar verktygen agenten deklarerar — läsa, skriva, skicka ut,
förstöra — deterministiskt, innan någon modell ser prompten. En agent som KAN
radera kör bara mot er egen infrastruktur, eller stoppas. Namnen på verktygen
hamnar i audit-loggen — aldrig argumenten, de kan innehålla kunddata. Och ja:
mina egna agenter — Hermes, Odysseus — går genom exakt samma sluss."

*(Fallback om något strular: kör `route_explain` via MCP med samma payload —
samma motor, inga providers, ingen statistik.)*

## Finalen — beviset (5 min)

1. **Dashboard**: börja med **"Where your data went"** — local/cloud/blocked per datakategori ("personnummer: 2 kept in the house"), sedan besparing + CO2 vs all-premium-baseline.
2. **Policy-konsolen**: aktiva regler läsbart, dry-run, riskregistret
   (leverantörsmatrisen med "required"-kolumner).
3. **`/router/incident-report?window=24h`**: "om ni måste rapportera en
   incident har ni LLM-evidensen för 24/72-timmarsfönstret på en URL — antal
   och klassningar, aldrig promptinnehåll."
4. **`/router/audit/export`**: hash-kedjad, verifierbar offline — "ni kan
   radera data, men aldrig faktumet att ni raderade den."
5. Landa i **monitor mode**: "Kör paketet som skuggpolicy i två veckor mot er
   riktiga trafik — inget ändras för användarna, och gap-rapporten ger er
   siffran: hur många prompts med persondata som gick till molnet."

## Om något går fel live

- **DGX nere** → beats 2–5,7 blir fail-closed-blocks i stället. Berätta-story:
  "det är exakt vad fail-closed betyder — hellre stopp än läcka." Verifiera
  före mötet med Test-knappen på qwen-raden.
- **Premium-modellen svarar inte** → beat 1/5 faller vidare i fallback-kedjan;
  synligt i request-loggen. MCP `recent_errors` + `provider_probe` felsöker på
  under en minut.
- Reservväg: kör ALLA beats i **dry-run** i stället för chatten — samma
  klassning/routing, inga provideranrop alls.

## LIVE-FACIT

**2026-09-29 kväll, slutgenrep inför demo (gäller):** roster = cheap-general
(gpt-4o-mini, moln) · balanced-coder (gpt-4o, moln) · **autoversio** (Qwen 27B
på 2×RTX 5090, lokal, balanced 1.0/4.0) · glm-local (DGX Spark, lokal reserv,
2.0/8.0). glm-premium/Z.ai borttagen. Lokalpris över gpt-4o-mini är avsiktligt:
annars vinner lokalt även ofarligt och molnbeatet försvinner.

| Beat | Utfall | Tid |
|---|---|---|
| B1 commit-meddelande | moln · cheap-general | 1.6 s |
| B2 personnummer | **lokal** · autoversio · `pii:personnummer` | 2.6 s |
| B3 hälsodata | **lokal** · autoversio | 3.9 s |
| B4 bokslut (sv) | **lokal** · autoversio | 4.7 s |
| B5 Go-deadlock | **lokal** · autoversio (ändrat, se nedan) | 12 s |
| B6 säkerhetsgranskning | **403** fail-closed | 0.6 s |
| B7 NDA | **lokal** · autoversio | 2.2 s |
| B8 pin + personnummer | **403** (pin rundar inte) | 0.9 s |
| B9 agent m. delete-verktyg | moln tills regeln skapas live → lokal | 1.2 s |

**B5 nytt utfall:** källkod går nu *lokalt* — autoversio är billigare än
gpt-4o för balanced-uppgifter. Ny replik: "Svår kod → starkaste modellen
policyn tillåter, och den är er egen: 12 sekunder, inget lämnade huset."
Hoppa över det interaktiva "lägg till source_code i regel 1"-momentet.

**2026-09-29, genrep mot sluss.eu (ISSUE-113/114 live, ny DGX-endpoint):**
lokal modell är nu **glm-local** (glm-5.3-flash på DGX Spark, balanced, 1.0/4.0).
B1 moln · B2 pii lokal (`pii:personnummer`) · B3 health lokal · B4 financial
lokal · B5 source_code moln · B6 403 · B7 legal lokal · B8 403 (pin rundar
inte) = **8/8**. B9 verifierat lokalt på main-builden: konsolregeln
"agent capability: destructive → local" matchar med delete-verktyget, utan
tools → moln; skapas live i demon.

**Risker inför demo:**
- **Z.ai ger 429 på varje anrop** (kvot). ISSUE-113-fallbacken räddar B1/B5
  (svarar via OpenRouter), men dry-run visar glm-premium medan svaret kommer
  från gpt-4o-mini/gpt-4o. Fixa kvoten eller avaktivera glm-premium.
- **Lokal latens 20–60 s** (glm-5.3-flash ≈ 22 tok/s, långa svar). B4 tog
  59,6 s mot 60 s timeout; B5 fastnade 60 s på glm-local innan gpt-4o svarade.
  Kör lokala beats i **chatten (streaming)** så tokens syns direkt, och visa
  dry-run (instant) som beviset. Svaren säger korrekt "jag ser inget
  underlag" — poängen är *vart* prompten gick, inte svaret.


**2026-08-15, genrep mot sluss.eu (redeploy m. ISSUE-108/110/111):** beats 1–8
= 8/8 enligt facit, med rollbyte: **laguna-s-21** (ny DGX-modell) har tagit
över de lokala beatsen (2/3/4/7) från qwen36-35b — samma egress-utfall, bättre
svar. B1/B5 glm moln, B6/B8 403. Beat 9 kräver redeploy med NIS2-regel 4d
(#141, mergad efter deployen) + `builtin:nis2-baseline` aktiv — verifieras
efter nästa redeploy.

**2026-07-19, verifierat mot sluss.eu** (efter DNS-flytt, ny env, tier-fix
gpt-4o→balanced, ISSUE-107-baseline och demo-data-reset): alla 8 beats enligt
tabellen nedan — B1 glm moln · B2 pii lokal · B3 health lokal · B4 financial
lokal (svenska) · B5 source_code moln · B6 403 fail-closed · B7 legal lokal ·
B8 pinnad moln → 403. Konsolreglerna (console_rule_1) aktiva efter redeploy.

### Ursprungligt facit 2026-07-14 (verifierat mot tokenizer.froste.eu)

Aktiv policy: **`pv_console_001`** — Magnus konsolregler (Väg B): regel 1 =
sex känslighetsklasser → require `local`; regel 2 = `security_review` → block.
Baseline `pii-local` kvar i env som rollback-mål. Roster: qwen36-35b (dgx,
cheap, 1/4), glm-premium (zai, slug `glm-4.6` — FIXAD), cheap-general,
balanced-coder.

| Beat | Live-utfall |
|---|---|
| 1 trivial | `glm-premium` · moln |
| 2 PII | `qwen36-35b` · **LOKAL** |
| 3 health | `qwen36-35b` · **LOKAL** |
| 4 financial (SV) | `qwen36-35b` · **LOKAL** |
| 5 debug | `glm-premium` · moln → *interaktiva regel-momentet* |
| 6 security review | **BLOCKED fail-closed** |
| 7 legal/NDA | `qwen36-35b` · **LOKAL** |

Kända kuggar (från repetitionerna): modell med pris 0 vinner all scoring;
"Review this NDA…" klassas unknown_high_risk → använd "Summarise…"-
formuleringarna; konsol-regelverket ERSÄTTER baseline-paketet (pii måste vara
ibockad i regel 1).
