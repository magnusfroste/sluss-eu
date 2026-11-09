# ISSUE-166 — Benchmark: 444 märkta syntetiska frågor, mätbart och jämförbart

`epic: EPIC-15` · `priority: P1` · `state: done` · klar 2026-11-09

## Varför

Ägarens fråga: *hur bevisar vi värdet av en LLM, så att vi kan mäta oss med
Zscaler?* Korpusen i `catch_corpus_test.go` (49 fall) räcker för att hålla
reglerna ärliga, men är för liten för att jämföra verktyg, och den saknade
ett mått på hur ofta ofarliga frågor flaggas. Ingen tillverkare publicerar
sådana siffror för sina DLP-motorer; vi kan inte kopiera dem ur en broschyr
och ska inte hitta på dem. Det vi kan göra är att publicera metoden, datan
och vårt eget resultat, och låta den som har Zscaler eller Netskope köra
samma frågor och räkna på samma sätt.

## Levererat

- **`benchmark/generate.py`**: deterministisk generator (fast frö) av 444
  unika syntetiska frågor på svenska och engelska. Personnummer, kortnummer
  och IBAN har giltiga kontrollsiffror; nycklar är bara format som inte
  liknar riktiga hemligheter. Varje fråga har etiketter (dataklasser) och
  ett slag: *identifier*, *term*, *meaning* (känsligt bara genom innehållet),
  *lookalike* (ser känsligt ut men är det inte) och *plain*.
- **`cmd/benchmark`**: `run` (in-process, endast regler, eller mot en
  instans via dry-run så att en andra åsikt mäts), `score` (andel fångade,
  andel ofarliga som flaggas, per slag, recall/precision per klass,
  latens, samma svar varje körning; Markdown) och `export` (CSV och textfiler
  plus en mall att fylla i för ett annat verktyg).
- **`benchmark/presidio.py`**: samma frågor genom Microsoft Presidio, som
  jämförelse med en öppen PII-detektor.
- **`make benchmark`** i CI: kontrollerar att korpusen är vad generatorn
  skriver och att varken andelen fångade eller andelen falsklarm blir sämre
  än golvet.
- **Manualsidan *Benchmark*** med metod, resultat och hur man mäter ett
  annat verktyg.

## Mätt med andra åsikt (demo, 2026-11-09)

Känsliga flaggade 71 % → 93 %; *meaning only* 4 → 43 av 58; ofarliga
flaggade 18 % → 23 %; samma svar båda körningarna för alla 444; median
273 ms inklusive nätverk. `run -url` klarar nu blockerade svar (403 utan
`job`): reglernas klasser räknas lokalt och andra åsiktens läses ur
`decision_reasons`.

## Fynd (rättas i ett eget ärende, inte mot samma korpus)

- Ordet *email*, *phone* eller *password* i en ofarlig fråga ger en klass.
- `db_password=…` fångas inte (understreck före ordet).
- En adress på *torget*/*gatan* utan känt gatuord fångas inte alltid.
- `#` följt av siffror (fakturanummer) räknas som kod.
- SQL på en rad utan staket räknas inte som kod.

Att rätta reglerna mot frågorna de mäts med vore att rigga mätningen. Nästa
ärende rättar dem mot nya, egna exempel och genererar sedan en ny korpus med
ett nytt frö.
