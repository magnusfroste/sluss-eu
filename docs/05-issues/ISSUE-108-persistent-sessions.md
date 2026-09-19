# ISSUE-108 — Persistenta konsol-sessioner (överlever redeploy) + Secure-cookie bakom proxy

- **Epic:** EPIC-14 (nyckel-/åtkomstprovisionering)
- **Priority:** P1
- **State:** done (2026-07-19)

## Bakgrund (rapporterad live)

Efter varje redeploy blev admin utloggad. Rotorsak: `SessionStore` var en
ren in-memory `map[string]session`. Cookien fungerade (webbläsaren skickade
den), men serverns *minne* av sessionen nollställdes vid processomstart →
cookien pekade på ett sessions-ID som inte längre fanns → utloggad. Samma
flyktighet som allt annat som inte ligger i SQLite. Dålig signal för en
säkerhetsprodukt: "överlever inte ens sin egen omstart".

## Ändring

- **`sessions`-tabell i SQLite** (`internal/history/sessions.go`): `id`,
  `username`, `role`, `expires_at`. Ligger på `ROUTER_DATA_DIR`-volymen
  (`/data`), överlever redeploy.
- **`SessionStore` = write-through cache** (`adminauth.go`): in-memory på
  fast path (DB rörs aldrig på request-vägen, som API-nycklarna), men
  `create`/`delete` skriver igenom till SQLite och konstruktorn **laddar
  aktiva sessioner vid boot**. `SessionPersister`-interface → `nil` i tester =
  ren in-memory.
- **Server-side sessions behålls medvetet** (revocable: logout, break-glass,
  expiry) framför en stateless signerad cookie man inte kan döda — för en
  kontroll-/bevisprodukt är "vi kan kicka en session" en poäng.
- **Secure-cookie bakom proxy** (`requestIsHTTPS`): bakom Cloudflare Tunnel
  termineras TLS vid edgen och origin ser HTTP (`r.TLS == nil`), så
  `Secure`-flaggan sattes aldrig. Nu litar vi även på
  `X-Forwarded-Proto: https`.

## Tester

- `TestSessionSurvivesRestart` — session skapad mot en store resolvar efter
  att en *ny* store öppnats mot samma SQLite-fil (simulerar redeploy); och
  logout persisteras (raderad session återuppstår inte efter omstart).
- Befintliga login/admin/demolink-tester körs mot `NewSessionStore(nil)`
  (in-memory) — oförändrat beteende.

## Not

Sessioner rensas opportunistiskt (utgångna rader raderas vid boot-load).
12h TTL oförändrad.
