# ISSUE-125 — Autodeploy: merge till main → demo-instansen kör den nya builden

- **Epic:** EPIC-14 (drift)
- **Priority:** P2
- **State:** done (2026-10-11)

## Bakgrund

Varje merge krävde en manuell Deploy i EasyPanel, och ungefär varannan deploy
landade inte (gamla containern behölls utan fel). ISSUE-121 gjorde det
synligt via `sluss_build_info`; nu används samma signal för att verifiera.

## Ändring

Nytt jobb `deploy` i `.github/workflows/publish-image.yml`, efter
`build-push`, bara på `main`:

1. POST till EasyPanels deploy-webhook (secret `EASYPANEL_DEPLOY_WEBHOOK`).
   URL:en innehåller en token — den loggas aldrig.
2. Pollar `${SLUSS_DEPLOY_URL}/metrics` (repo-variabel) var 15:e sekund i upp
   till 12 minuter tills `sluss_build_info` visar `github.sha`.
3. Landar den inte: triggar en gång till och väntar igen; därefter rött jobb
   med "Stop, then Deploy"-hänvisning.

Utan secret hoppas jobbet över med en notis (forks förblir gröna). Utan
variabel triggas deployen men verifieras inte. `concurrency: deploy-main`
avbryter en äldre deploy när en nyare merge kommer.

## Uppsättning (ägaren)

1. EasyPanel → tjänsten → Deployments → kopiera **Deploy Webhook**-URL:en.
2. GitHub → Settings → Secrets and variables → Actions:
   - Secret `EASYPANEL_DEPLOY_WEBHOOK` = webhook-URL:en
   - Variable `SLUSS_DEPLOY_URL` = `https://demo.sluss.eu`
3. Stäng av EasyPanels egen "Auto Deploy" om den är på, annars byggs två gånger.
