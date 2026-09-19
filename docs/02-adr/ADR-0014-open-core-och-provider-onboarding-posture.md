# ADR-0014: Open core, två-repo-modell och provider-onboarding som medvetet opt-in

## Status

Accepterad (2026-07-25). **Delvis ersatt 2026-09-19** (DECISION_LOG, ISSUE-112): beslut 3 (två repos, allowlist-export) utgår — repot är EN publik utvecklarrepo med ny historik; beslut 1 (AGPL-3.0-kärna) kvarstår, `ee/`-splitten (beslut 2) skjuts upp tills en kommersiell modell beslutats. Beslut 4 (provider-opt-in) kvarstår oförändrat.

## Kontext

Sluss är en säkerhetsprodukt — den säljer *kontroll + bevis* över vem som får
hantera organisationens data. Två återkommande frågor behövde ett formellt,
verifierbart svar innan produkten möter köpare och ev. open-source-community:

1. **Öppen källkod?** Förtroende är produkten. Påståenden som "ingen LLM i
   beslutet, fail-closed, hash-kedjad audit" är starkast när en teknisk
   granskare kan *läsa koden* (DECISION_LOG 2026-07-10). En stängd
   säkerhetsprodukt som säger "lita på oss" är svagare än en öppen som säger
   "läs själv". Samtidigt lever ett bolag på något — n8n/Supabase/Grafana/
   LiteLLM visar open core: öppen kärna + betald unlock för admin-bekvämlighet.
2. **Hur läggs providers (t.ex. OpenAI) till?** Om en amerikansk molnprovider
   är påslagen *by default* motsäger det hela sovereignty-pitchen och
   fail-closed-by-default. (DECISION_LOG 2026-07-19, "nycklarna till
   lägenheten".)

Repot var dessutom kortvarigt publikt med hela GTM-korpusen (deck, strategi,
advisor-identitet) — disciplin räckte inte som skydd (DECISION_LOG 2026-07-19,
två-repo-beslutet).

## Beslut

### 1. Open core, AGPL-3.0-only på kärnan

Kärnan (gateway, klassificerare, policy-motor med fail-closed, audit-kedja +
verifier, monitor mode + gap-rapport, compliance-packs) är **öppen källkod
under AGPL-3.0-only**. AGPL hindrar att någon SaaS:ar kärnan utan att bidra
tillbaka. **Säkerheten och beviset paywallas aldrig** — att paywalla det vi
säljer ("betala för att vara trygg") rasar pitchen. Måttet: hela *Measure*-
och *Enforce*-stegen är gratis.

Premium (`ee/`, kommersiell licens) = admin-bekvämlighet och organisationslager:
SSO/SCIM, multi-tenant i skala, schemalagda kontroll-/incidentrapporter,
signerade regime-packs som prenumeration (den underhållna NIS2/DORA/AI-Act-
korpusen = moaten, inte proxyn). Prisstegen justeras: *enforce* är gratis, det
är organisations-/bekvämlighetslagret som kostar.

### 2. `ee/`-gränsen är additiv — kärnan bygger fristående

Kärnan importerar **aldrig** `ee/`. `ee/` importerar kärnan och registrerar
sig via plugin/build-tag, så en publik build kompilerar utan `ee/`. Detta gör
open/closed-splitten mekanisk, inte konventionsbaserad.

### 3. Två repos: privat = sanningskälla, publikt = genererad allowlist-export

Se DECISION_LOG 2026-07-19. Privat repo (detta) är enda utvecklingsstället
(produkt + internminne + `ee/`). Publikt repo byggs av ett **allowlist**-
export-skript (fail-closed: lista vad som GÅR ut, inte vad som exkluderas) med
CI-guard och ren ny historik. Implementeras i ISSUE-109.

### 4. Provider-onboarding: medvetet opt-in, aldrig default-on

Varje egress-provider (inkl. OpenAI/ledande providers) läggs till av admin som
en **taggad, tierad connection** (som DGX/zai) — aldrig påslagen i env av
misstag. Arkitekturen stödjer redan detta (ISSUE-072: Provider = connection med
compliance-taggar; secret bara via env-var-*namn*; policy refererar
tiers/profiler per ADR-0011, aldrig råa vendornamn). OpenRouter kvarstår som
enkel on-ramp **för publikt, okänsligt data** (aggregatorn är dessutom en extra
subprocessor; för känsligt data är en direkt taggad connection mer försvarbar).
Kurering-som-kontroll (liten taggad, policy-gate:ad katalog — inte en
modell-firehose) är en USP.

## Konsekvenser

- Positivt: "läs koden själv" blir en pitch-poäng mot både LiteLLM-DIY och
  SaaS-konkurrenter; AGPL skyddar mot gratis-SaaS-kopiering; fail-closed-by-
  default gäller även vår egen provider-policy; splitten är mekanisk (allowlist
  + `ee/`-gräns), inte disciplinberoende.
- Negativt/kostnad: `ee/`-gränsen kräver disciplin i importriktningen (CI kan
  vakta att kärnan inte importerar `ee/`); två repos innebär ett explicit
  export-steg vid publicering; open core kräver community-hygien (SECURITY.md,
  CONTRIBUTING, licens-headers) innan publikt.
- AGPL kan avskräcka vissa proprietära integratörer — accepterat; kärnköparen
  self-hostar och AGPL-copyleft är sällan ett hinder för intern drift.

## Alternativ

- **Helt stängd källa** — förkastat: undergräver förtroende-pitchen; en
  säkerhetsprodukt vinner på granskbarhet.
- **Permissiv licens (Apache/MIT)** — förkastat: låter en molnleverantör
  SaaS:a kärnan utan återbidrag; AGPL:s network-copyleft är exakt vårt skydd.
- **Fair-code / source-available (BSL/SSPL)** — övervägt: starkare skydd men
  förlorar rätten att kalla det "open source", och det ordet bär
  förtroendevärdet. AGPL valdes.
- **Paywalla audit/SSO som LiteLLM** — förkastat: för Sluss ÄR evidens och
  fail-closed produkten; att paywalla dem säljer otrygghet.
- **OpenAI som default-on provider i env** — förkastat: motsäger sovereignty
  och fail-closed-by-default; medvetet opt-in är kontrollberättelsen.
