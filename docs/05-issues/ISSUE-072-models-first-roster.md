# ISSUE-072: Models-first roster (LiteLLM-style) — split models from providers

## Labels
- `epic: EPIC-11`
- `priority: P1`
- `type: enhancement`
- `sprint: v1.1`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Gör **Models** till den primära admin-ytan (rostern) och **Providers** till ett tunt kopplingslager — som LiteLLM ("Models + Endpoints" primärt, "LLM Credentials" separat). En modell är en routningsbar enhet med `tier` (vår USP); en provider är bara en återanvändbar koppling (endpoint + nyckel-env). Det gör om-tiering och modell-mix till en snabb UI-handling och realiserar "inget hårdkodat".

## Bakgrund

Nuvarande design (ISSUE-071, #67) är provider-centrerad: modeller ligger *inne i* `providercfg.Provider`. Men admins frekventa handling är att kurera **modell → tier** (rostern), inte att koppla upp providers (sällan). Referens: LiteLLM:s UI (Models + Endpoints primär; LLM Credentials som separat, återanvändbar entitet; Price Data Reload för pris-sync; Provider i Add Model är en *typ*, inte en förkonfigurerad entitet). Tier finns inte i LiteLLM — det är tokenizers klassificerings-routning och blir den avgörande kolumnen på Models-sidan.

**Viktigt (behåll USP:n):** routern gör `task → tier` automatiskt. Models-sidan sätter `tier per modell` (rostern), INTE modell per klassificering manuellt. Se [[backlog-user-curatable-roster]].

## Datamodell (split)

- **Provider (koppling)** — `providers.json`: `{ id, name, base_url, key_env }`. Inga modeller. (Refaktor av dagens `providercfg.Provider`; ta bort `Models`.)
- **Model (routningsbar)** — `models.json`: `{ id/name, provider_id, provider_model_id, tier, input_usd_per_mtok, output_usd_per_mtok, enabled }`. Refererar en provider via `provider_id`.
- Migrering: dagens `providers.json` (inbäddade modeller) → dela upp i `providers.json` (kopplingar) + `models.json` (modeller). Engångs vid start.

## Acceptanskriterier

- **🧩 Models-sida (primär)**: lista alla modeller (inbyggda seed + custom) med provider, slug, **tier**, pris, enabled, pris-källa (synkad/manuell). Add Model-formulär: välj **befintlig provider**, slug, **tier**, pris. Redigera/ta bort/**om-tiera** custom-modeller med ett klick. Inbyggda visas (read-only tills de blir seed-data, se nedan).
- **🔌 Providers-sida (koppling)**: bantad till CRUD för kopplingar (`base_url` + `key_env`), som LiteLLM:s "LLM Credentials". Ingen modell-inmatning här.
- **Inbyggda som seed-data**: de tre (`cheap-general`/`balanced-coder`/`premium-reasoning`) seedas till `models.json` vid första start (OpenRouter som provider-koppling) → inget hårdkodat, allt är data och om-tierbart. Alternativt: behåll som defaults men gör dem redigerbara via override.
- **main.go**: bygg registry-definitionen från providers + models (+ inbyggda seed); en `OpenAIAdapter` per provider vars nyckel finns i env. Modell oroutningsbar (`Enabled=false`) om dess providers nyckel saknas.
- **Pris**: sync scopad per provider (klar, #70); manuellt pris per modell är auktoritativt. "Price reload"-knapp (valfritt; kräver registry-hot-reload för utan-omstart).
- **Nav**: Models primär, Providers sekundär. Tester + `make smoke` grönt.

## Tekniska noter

- Bygg vidare på: `providercfg` (dela i providers + models), `pricing` (sync), Providers-sidan (banta), registry-merge i main.go.
- Håll fast-path orörd; bygget sker vid start. Ändringar tillämpas vid omstart (samma som nyckel-i-env).
- Live hot-reload (om-tiera utan omstart) = separat större steg (bygg om snapshot + adaptrar i drift).

## Härkomst

Användarens omvärdering + LiteLLM:s faktiska UI ("Models + Endpoints" primär, "LLM Credentials" separat). Slår ihop tre trådar: [[backlog-models-page]], provider-CRUD (#67) och "inget hårdkodat"/[[backlog-auto-pricing]] steg 2.
