# ISSUE-167 — Fem brister som benchmarken hittade

`epic: EPIC-14` · `priority: P1` · `state: done` · klar 2026-11-10

## Varför

Första körningen av benchmarken (ISSUE-166, v1.8.0) visade fem fel i
reglerna: 18 % av de ofarliga frågorna flaggades och 10 % av frågorna med en
identifierare eller nyckel missades.

## Levererat

1. **Nyckeletikett bakom understreck.** `db_password=…`, `smtp_auth_token: …`
   (`internal/secrets/mask.go`: `\b` ersatt med `(?:\b|_)`).
2. **Gata i två ord.** *Lilla torget 47*, *Stora gatan 12* tillsammans med
   postnummer (`internal/pii/pii.go`). *plan 3* är fortfarande ingen gata.
3. **En SQL-sats utan staket** räknas som kod (`sqlStatementPattern`);
   *please select a date from the list* gör det inte.
4. **Orden *email*, *phone*, *address*** ger inte längre klassen pii; värdena
   hittas av detektorerna.
5. **Ämnesord om kod** (*invoice*, *checkout*, *security*) ger klassen
   source_code bara när uppgiften är en koduppgift. Risknivån påverkas som
   förut.

Lämnat som det är: orden *password*, *token*, *api key* flaggar fortfarande
frågan som möjlig hemlighet. En miss där är ett läckt lösenord.

## Hur mätningen hålls ärlig

Varje rättning testas på nya exempel som inte finns i korpusen
(`internal/classifier/benchmark_fixes_test.go`). Korpusen är sedan
omgenererad med nytt frö; mallarna är desamma, så siffrorna visar
rättningarna. Manualsidan redovisar före (1.8.0) och efter (1.8.1).

## Syntetiskt, och ingen verklig persons

Generatorn tog förut slumpade personnummer med giltig kontrollsiffra, som
kan tillhöra en verklig person. Nu: Skatteverkets testpersonnummer
(`benchmark/test_personnummer.txt`), PTS-serien för fiktion
(070-174 06 05–99) och kortprefix 4000 00.

## Resultat (endast regler)

| | 1.8.0 | 1.8.1 |
|---|---|---|
| Känsliga flaggade | 71 % | 74 % |
| Ofarliga flaggade | 18 % | 10 % |
| Identifierare/nyckel | 90 % | 100 % |
| Rätt klass exakt | 75 % | 81 % |

## Beteendeändring

Ett brev som nämner en faktura eller ett mejl, utan värden, klassades förut
som source_code respektive pii och hölls lokalt av sådana regler. Det kan nu
gå till molnet. Står i CHANGELOG.

## Andra åsikten på nya korpusen (demo, 2026-11-10)

Känsliga flaggade 74 % → 92 %; *meaning only* 0 → 39 av 58; ofarliga
flaggade 10 % → 13 %; samma svar båda körningarna; median 296 ms med nätverk.
`benchmark run -url` gör nu omförsök vid nätverksfel och 5xx.

## Kvar

- Fixturen *architecture prose* i `catch_corpus_test.go` flyttad till
  ofarliga; tabellen i *What Sluss catches* uppdaterad (31 av 40).
