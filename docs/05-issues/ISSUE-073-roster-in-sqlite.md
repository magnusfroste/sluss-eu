# ISSUE-073: Roster in SQLite — the DB controls tier → model (with live reload)

## Labels
- `epic: EPIC-11`
- `priority: P1`
- `type: enhancement`
- `sprint: v1.1`
- `category: enhancement`
- `state: ready-for-agent`

## Mål

Gör **SQLite till källan för rostern** (vilken modell fyller vilken tier) och för provider-kopplingarna — istället för `models.json` / `providers.json`. Databasen styr tier→modell. Fas 2 i samma issue: **live hot-reload** så att ändringar på Models-sidan gäller *utan omstart* för tier→modell-ändringar.

Bygger vidare på ISSUE-072 (#72), som la rostern i JSON-filer på volymen. Detta flyttar samma data till `history.db` (den befintliga SQLite:n under `ROUTER_DATA_DIR`) så det finns EN källa för sanning.

## Bakgrund

Efter #72: `providercfg` läser/skriver `providers.json` (kopplingar) + `models.json` (modeller: tier, provider, slug, pris). `internal/history` äger `history.db` (requests, kv). Användaren vill att DB:n styr rostern — enklare mental modell, en lagringsmekanism, och grund för live-redigering. Nycklar ligger kvar i env (aldrig i DB) — CISO-hållningen består.

## Acceptanskriterier

**Fas 1 — DB som källa (tillämpas vid omstart):**
- Nya tabeller i `history.db`: `roster_providers` (id, name, base_url, key_env) och `roster_models` (id, provider_id, provider_model_id, tier, input_micros_per_mtok, output_micros_per_mtok, enabled). Metoder på `history.Store`: Load/Upsert/Delete för båda.
- **main.go**: öppna roster-storet *före* registry-bygget (kräver omordning — history.db öppnas i dag efter registry-bygget). Bygg registry-definitionen från DB-rostern (+ seed vid tom DB). Utan `ROUTER_DATA_DIR`: falla tillbaka på in-memory seed (som i dag).
- **Migrering (engångs)**: om `models.json`/`providers.json` finns och DB-rostern är tom → importera dem till tabellerna, döp om filerna `.imported` (jfr spend.json-migreringen). Om #72 redan seedat filer täcker detta.
- **Models- + Providers-sidorna**: CRUD skriver till SQLite istället för JSON-filer. Samma UI.
- Pris-sync scopad per provider (klar, #70) och seed av de tre inbyggda (klar, #72) bevaras — nu seedade till DB.

**Fas 2 — live hot-reload (tier→modell utan omstart):**
- När Models-sidan ändrar rostern: bygg om registry-snapshoten och swappa in den via `registry.Store` (det finns hot-reload-stöd; jfr policy-hotreload). Tier→modell-ändringar behöver bara ny snapshot — befintliga providers adaptrar täcker nya slugs, så ingen adapter-ombyggnad krävs.
- **Nya providers** (ny endpoint/nyckel) kräver fortfarande omstart (nyckeln sätts i env → redeploy ändå) — dokumentera skillnaden i UI:t ("tier/modell-ändringar gäller direkt; nya providers vid omstart").
- Fas 2 får delas till eget PR om Fas 1 blir stor.

## Tekniska noter

- Håll allt under `ROUTER_DATA_DIR`; en DB (`history.db`). Ingen ny fil-DB.
- Fast-path orörd; rostern läses vid start (Fas 1) resp. vid reload (Fas 2), aldrig i request-vägen.
- Nycklar: bara env, refererade per `key_env`. Ingen hemlighet i DB.
- Löser även [[backlog-roster-override-deprecation]]: när DB äger rostern kan `ROUTER_MODEL_*`-env-override:n antingen skriva till DB vid start eller fasas ut (den skapar annars mismatch mellan UI och routning).
- Tester: store-metoder (round-trip, migrering, seed-vid-tom), CRUD-handlers, `make smoke` grönt, statiskt CGO_ENABLED=0-bygge intakt.

## Klar när

- DB:n styr tier→modell; Models/Providers-CRUD persisterar till SQLite; migrering från JSON funkar; (Fas 2) tier→modell-ändring gäller utan omstart. Full svit grön.
- `01-architecture/14-deployment-topology.md` uppdaterad: rostern i SQLite för v1.0 (Postgres = Enterprise 2.0).

## Härkomst

Användarbeslut efter models-först (#72): "vi fixar så databasen styr vilken modell för vilken tier." Konsoliderar config till samma SQLite som historik. Se DECISION_LOG 2026-07-04 (lagringstrappa) och [[backlog-models-page]].
