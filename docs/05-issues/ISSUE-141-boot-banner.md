# ISSUE-141 — Startbanner i loggen

`epic: EPIC-14` · `priority: P3` · `state: done`

## Varför

LiteLLM och AnythingLLM hälsar operatören i `docker logs` med version och var
konsolen finns. Sluss loggade bara en JSON-rad `router starting` sist, svår att
hitta bland init-raderna.

## Vad

- Första loggraden vid varje start: `sluss starting` med version, commit, build
  — syns även om något under init fallerar.
- När servern är redo: en kort banner i klartext på stderr (Version, Runtime,
  Listening, Console, Providers, Data, Deployment, Update check, Manual) och en
  JSON-rad `sluss ready` med samma fakta för loggpipelines.
- `ROUTER_LOG_BANNER=off` tar bort klartexten för JSON-only-miljöer.
- Ingen hemlighet skrivs ut (inga nycklar, inga lösenord, ingen admin-användare).

## Klart när

- `cmd/router/banner_test.go` täcker release- och main-bygge, konsol-URL från
  `ROUTER_PUBLIC_URL` respektive localhost:port.
- Lokal uppstart visar bannern och `sluss ready`.
