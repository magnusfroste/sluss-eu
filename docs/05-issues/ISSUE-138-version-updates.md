# ISSUE-138 — "Ny version finns" och uppgraderingsväg

- **Epic:** EPIC-14 (drift)
- **Priority:** P2
- **State:** done (2026-10-29)

## Bakgrund

Ägaren: inspireras av AgentHotel, som visar att en ny version finns och kan
uppgradera sig själv. Self-hosters kör Docker, Compose, EasyPanel eller
AgentHotel.

## Beslut

Samma information som AgentHotel, men **ingen självuppgradering**: AgentHotel
har Docker-socketen och kör ett skript på värden; en gateway som skyddar
organisationens data ska inte ha makt över sin egen värd. Sluss visar i
stället exakt uppgraderingssteget för just den installationen.

## Ändring

- `updates.go`: bakgrundskontroll var 6:e timme mot GitHubs publika API
  (`compare/<denna build>...main`): hur många ändringar efter, och de 15
  senaste commit-rubrikerna. Skickar inget om instansen. Utvecklingsbyggen
  utan commit-stämpel jämförs inte. `ROUTER_UPDATE_CHECK=off` för
  air-gappade installationer.
- **Version-sidan** `/router/system`: denna build (commit, byggtid), senaste
  på main ("N changes behind" / "Up to date" / "Check is off"), **What is
  new** med länkar, och **How to upgrade** per sätt — EasyPanel, Docker,
  Compose, AgentHotel — med `ROUTER_DEPLOYMENT` först. "Check now"-knapp.
- **Menyns fot:** versionen länkar till sidan och får en gul prick när en
  uppdatering väntar.
- **Overview:** en blå informationsrad under "Needs you" när en ny version
  finns.
- Env-exempel, compose (`ROUTER_DEPLOYMENT=compose`), manualen.

## Verifierat

Tester mot fejkat GitHub-API: hittar nyare version och ändringar i rätt
ordning, up to date, dev-build, avstängd, nil-checker; sidan visar rätt
steg först; prick i menyn; informationsrad på Overview.
