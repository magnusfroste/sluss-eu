# ISSUE-127 — Sluss som app i AgentHotel

- **Epic:** EPIC-14 (drift/distribution)
- **Priority:** P2
- **State:** done (2026-10-14) — PR i `magnusfroste/agenthotel`

## Bakgrund

AgentHotel (magnusfroste/agenthotel) kör agenter som isolerade containrar
med egen domän, TLS och volym, och injicerar providernycklar. Sluss passar
som mall av typen `git-app` (som Lobby och Reel-studio): hotellet bygger
vår Dockerfile, monterar `/data` (imagens default sedan ISSUE-126), och
Caddy sköter domän och TLS.

## Ändring (i agenthotel)

- `templates/sluss/meta.yaml`: git-app mot `sluss-eu@main`, port 8080,
  `/healthz`, `LOCAL_API_KEY` (hex) och admin-lösenord genererade per
  deploy, `ROUTER_PUBLIC_URL=https://${DOMAIN}`, `INJECT_PROVIDER_ENV=true`
  så hotellets OpenRouter-nyckel gör molnmodellerna routbara direkt.
  Formulär: compliance-pack (default nis2-baseline) och valfria OIDC-fält.
  OIDC-fälten ligger medvetet utanför envFile: en ofylld `${PLACEHOLDER}`
  hade nått Sluss som bokstavlig issuer.
- git-app skickar nu `GIT_SHA` (full sha) och `BUILD_TIME` som build-args,
  så sidfoten i Sluss visar rätt commit även när hotellet bygger.
- Instruktionerna beskriver poängen: lägg Sluss som provider på hotellet
  (`https://<domain>/v1`, genererad nyckel, modell `auto`) så att varje
  Hermes/OpenClaw-agent går genom gatewayen — klassning, routing och audit
  före varje modellanrop.

## Verifierat

Hotellets backend-tester (213/213, inkl. nytt mall-test), `node --check`,
frontend-bygge. Ej deployat på ett riktigt hotell ännu.
