# ISSUE-126 — Self-host med ren Docker

- **Epic:** EPIC-14 (drift)
- **Priority:** P2
- **State:** done (2026-10-12)

## Bakgrund

Demo-instansen körs i EasyPanel, men en kund som self-hostar vill köra
`docker run` eller compose direkt. Tre luckor:

1. Imagen satte inte `ROUTER_DATA_DIR`. En ren `docker run -v …:/data` utan
   den variabeln körde helt i minnet — nycklar, roster, konsolregler och
   audit-kedjan försvann vid omstart. Tyst.
2. `deploy/docker-compose.yml` pekade på ett gammalt image-namn
   (`ghcr.io/magnusfroste/sluss`) och hette "tokenizer".
3. Ingen guide för den som inte kör en PaaS.

## Ändring

- **Dockerfile:** `ENV ROUTER_DATA_DIR=/data` (katalogen fanns redan, ägd av
  `app`). Compose och EasyPanel satte den redan — ingen beteendeändring där.
- **Compose:** rätt image (`ghcr.io/magnusfroste/sluss-eu`), råd att pinna
  `sha-`/`v`-tagg, plus `ROUTER_ADMIN_USER/PASSWORD`, `ROUTER_POLICY_PATH`,
  `ROUTER_PUBLIC_URL`. Volymnamnet `tokenizer-data` behålls så befintliga
  installationer inte tappar data.
- **`deploy/example.env`:** samma nya nycklar.
- **Guide** `docs/07-operations/self-host-docker.md` (engelska — kundvänd):
  docker run, quick start, TLS (Caddy), uppgradering och buildverifiering,
  backup/restore av volymen, air-gap (`docker save/load`), checklista.
- README länkar guiden.
