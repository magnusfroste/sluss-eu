# ISSUE-159 — Konsolhärdning: inloggningsbroms, säkerhetsheaders, /metrics bakom nyckel

`epic: EPIC-14` · `priority: P1` · `state: done` · klar 2026-11-07 (#69)

## Varför

Sluss säljs som säkerhetsprodukt och granskas som en. Tre saker en CISO ser på
fem minuter: ett lösenordsformulär utan broms, en konsol utan
Content-Security-Policy, och `/metrics` öppet för internet.

## Levererat

- **Inloggningsbroms** på `/router/login`: fem fel per konto (tjugo per adress)
  inom femton minuter → väntetid som dubblas, upp till femton minuter. Varje fel
  och varje broms auditeras (`console.login_failed`, `console.login_throttled`)
  med adress. I minne per instans.
- **Säkerhetsheaders** på varje svar: CSP (bara egen origin, inline tillåtet
  eftersom konsolen renderar sina egna sidor), X-Frame-Options DENY, nosniff,
  Referrer-Policy, Permissions-Policy; HSTS bakom TLS-proxy
  (`X-Forwarded-Proto: https`). Verifierat med headless-webbläsare över alla
  konsolsidor och chatten: inga CSP-fel.
- **`/metrics`** bakom konsolgaten eller `ROUTER_METRICS_TOKEN` (egen token för
  skrapern). Ny publik `GET /version` (release, commit, byggtid) för
  deploy-kontroll och drift.
- Bonus: mocken strömmar, så chatten visar det inbyggda testsvaret.

## Icke-mål

Lösenordspolicy utöver åtta tecken, MFA för lokala konton (SSO-vägen ger MFA
via IdP:n).
