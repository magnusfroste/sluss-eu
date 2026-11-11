# ISSUE-150 — Konsolen 70/30: det väsentliga först, avancerat hopfällt

`epic: EPIC-14` · `priority: P1` · `state: done` · klar 2026-11-11

## Varför

Inför film för en CISO: Overview, chatten, Inspector, dry-run och rapporterna
är redo. Policy, Providers och Risk register visar maskinrummet (råa koder,
miljövariabler, öppna formulär, regler visade två gånger).

## Vad

- Ett mönster på alla sidor: det väsentliga synligt, en hopfälld
  **Advanced**-del längst ner som minns sitt läge.
- Klartext i stället för koder (`pii` → Personal data, `secrets_possible` →
  Secrets and keys).
- **Policy:** Your rules (en lista i klartext), Test a sentence (dry-run),
  Add a rule (dialog: när-data → Keep on own hardware / Block). Task type,
  risk, agentförmågor, råa taggar, compliance-paket, retention och danger
  zone under Advanced. Ingen dubbelvisning.
- **Providers:** kortare text, ny anslutning bakom en knapp.
- **Models:** lista och panel först; lägga till modell för hand, priser och
  thinking under Advanced.
- Risk register kvar som flik, tydligt för granskare.

Inget ändrar vad Sluss gör, bara vad som syns först.

## Levererat (2026-11-11)

- **Gemensamt:** `details.ds-adv[data-adv]` med ett skript som minns öppet
  läge per webbläsare (`advancedJS` i `design.go`); `dialog.ds-dialog`.
- **Policy:** *Your rules* är en lista i klartext (`plainRule`, `plainThen`),
  med *Remove* på regler tillagda i konsolen när de gäller. Utkast visas som
  *Not applied yet* med *Apply these rules*. *Add a rule* är en dialog (slag
  av data → Keep on own hardware / Mask and send / Block). *Test a sentence*
  svarar i klartext, skälen under *Details*. Under *Advanced*: vad som
  gäller och återgång, regel med fler villkor (uppgift, risk, agentens
  verktyg, egna taggar), regelpaket, lagringstid, radering.
- **Providers:** lista, *Test*, *Add a connection* och *Edit* i en dialog.
  Under *Advanced*: var varje nyckel läses ifrån, priser, tillstånd, och hur
  en egen modell läggs till.
- **Models:** lista med *Runs at* (own hardware / cloud), *Level*, *State*.
  Under *Advanced*: priser och leverantörens modellnamn, lägga till modell
  för hand.
- **Risk register:** samma tabell, texten omskriven för en granskare.
- **Fel rättat på vägen:** en maskeringsregel listades som *route by
  scoring*; `summarizeRoute` kände inte till `mask`. Sidan Models rullade i
  sidled på mobil (en skärmläsartext utan positionerad förälder).

## Mätt

Sidhöjd vid 1400 px bredd, före → efter: Policy 2010 → 900, Providers
1110 → 900, Models 900 → 900 (innehållet slutar vid 550). Synliga
formulärfält på Policy före: 19; efter: en textruta och två knappar.

## Verifierat

Enhetstester (ordning, klartext, inget visas två gånger, koder syns inte i
listan) och i webbläsare mot en lokal instans: testa en mening, lägga till
regel i dialogen, tillämpa, ta bort, Advanced minns sitt läge efter
omladdning, dialogerna för anslutning (ny och ändra), modellpanelen, ingen
sidledsrullning vid 390 px på någon av sidorna, inga skriptfel.

## Inte ändrat

Vad Sluss gör. Adresser och formulärfält är desamma, så skript mot konsolen
fungerar som förut. Quick start, Overview, Activity och Access är orörda.
