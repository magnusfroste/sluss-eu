# ISSUE-158 — Regler matchar alla dataklasser i en fråga

`epic: EPIC-14` · `priority: P1` · `state: done`

## Varför

En fråga fick bara en dataklass (den högst rankade: secrets > pii >
security_classified > health > financial > legal > source_code). På demon
blockerade regissörens regel "säkerhetsklassat → BLOCK" därför inte en
säkerhetsklassad fråga som också nämnde "secret" eller innehöll en nyckel — den
blev "secrets" och gick lokalt. Säkert, men inte vad regeln sa.

## Vad

- `JobDescriptor.Sensitivities`: alla dataklasser i frågan, starkast först.
  `Sensitivity` är oförändrad (den starkaste) — loggar, rapporter och klienter
  som läser den påverkas inte.
- `policy.EvaluationInput.Sensitivities`: en regels dataklass-villkor matchar
  när *någon* av frågans klasser finns med. Utan listan beter sig allt som
  förut.
- Dry-run visar `sensitivities`; svarshuvudet `X-Router-Data-Classes` listar
  dem; Inspector (Route → Data) skriver "also found: … — rules for these apply
  too".
- Manualen (How Sluss picks a model): flera klasser → alla matchande regler
  gäller, strängast vinner.

## Effekt

Strängare, aldrig svagare: fler regler kan matcha en fråga, och begränsningar
ackumuleras fail-closed. En fråga som tidigare gick lokalt kan nu blockeras om
en blockeringsregel gäller en av dess andra klasser — det är avsikten.

## Test

- `TestJobKeepsEveryDataClass` — säkerhetsklassat + nyckel + personnummer ger
  [secrets_possible, pii, security_classified].
- `TestBlockRuleOnClassifiedFiresWhenSecretsRankHigher` — blockeringsregeln på
  security_classified fiar även när secrets rankas högst; en ren
  nyckel-fråga träffas inte.
- Hela sviten, eval, policy golden och regression gröna.

## Rättelse samma kväll

Live visade ett kundklagomål med personnummer [pii, source_code]: ordet
"invoice" ger signalen "payment" (avsedd för betalkod), som räknades som
källkod. I listan kommer source_code nu bara från riktig kod; orden auth,
payment och security påverkar bara den enskilda primära klassen som förut.
Test: `TestDataClassesNoCodeFromTopicWords`.
