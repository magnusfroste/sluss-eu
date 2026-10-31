# ISSUE-151 — "Mask and send": tredje utfall för en regel

`epic: EPIC-14` · `priority: P1` · `state: needs-info` (efter ISSUE-150)

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
