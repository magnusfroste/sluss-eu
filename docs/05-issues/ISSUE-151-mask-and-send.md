# ISSUE-151 — "Mask and send": tredje utfall för en regel

`epic: EPIC-14` · `priority: P1` · `state: done` · klar 2026-11-09

## Idé

En regel har idag två utfall: behåll på egen hårdvara, eller blockera. Lägg
till ett tredje: ersätt personuppgifter med platshållare (`[PERSON_1]`,
`[ID_1]`) innan frågan går till molnet, och fyll i dem igen i svaret innan
användaren ser det. Molnkvalitet, men personuppgifterna lämnar aldrig huset.

Skillnad mot grindar (Zscaler m.fl.) som blockerar eller maskerar men inte
levererar ett svar, och mot proxys (LiteLLM m.fl.) som inte läser innehållet.

## KISS

Ingen ny inställning: ett tredje alternativ i den enkla regeldialogen —
*Keep on own hardware · Mask and send · Block*.

## Risk och gränser

- Missad maskning = läcka. Börja konservativt: bara det klassificeraren hittar
  säkert (personnummer, e-post, telefon, IBAN, hemligheter). Namn i fri text
  först när det kan göras pålitligt.
- Inspector (Context) och auditkedjan visar exakt vilka typer som maskades och
  hur många — aldrig värdena.
- Platshållarnas värden lever bara i minnet för den förfrågan.
- Bygger på befintlig maskning av hemligheter (ISSUE-042).

## Levererat (2026-11-09)

- Policy: `route: { mask: true }`; ett tredje alternativ i konsolens regeldialog
  (*keep on own hardware · mask and send · block*). Slås ihop med andra regler
  utan att lätta på något; tillämpas bara när svaret kommer från en modell som
  inte är lokal.
- `internal/masking`: platshållare per värde (samma värde, samma platshållare),
  hela konversationen (alla roller, textdelar i arrayform, verktygsanrop),
  hela texten (ingen 64 KiB-gräns). Återställning i vanliga svar och i
  strömmade, där en platshållare kan delas mellan två bitar.
- Detektorerna fick positioner (`pii.Find`, `secrets.MaskWith`) som delar
  mönster med klassningen. Grupperade IBAN upptäcks nu.
- Syns i dry-run, `X-Router-Masked`, inspektorn och auditkedjan
  (`request.masked`, typer och antal); mätvärde
  `router_masked_requests_total`.
- Verifierat end-to-end mot en molnleverantör som loggar vad den får: inga
  värden, bara platshållare; användaren fick värdena tillbaka i båda vägarna.

Kvar: namn i fritext (kräver pålitlig namnigenkänning; den lokala andra
åsikten kan bli vägen dit).
