# ISSUE-169 — Skydd av samhällsviktig verksamhet utan märkning, och preciserad andra åsikt

`epic: EPIC-14` · `priority: P1` · `state: done` · klar 2026-11-10

## Varför

Efter v1.9.0 missade även regler + lokal modell 11 av 232 känsliga frågor;
sju gällde hur en samhällsviktig anläggning skyddas, utan märkning. Sex av
modellens nio extra falsklarm var git-commit-meddelanden som lästes som kod.

## Levererat

- **Regel:** anläggning (vattenverk, ställverk, kontrollrum, ledningscentral,
  bränsledepå …) och skyddsord (skalskydd, svagheter, vaktschema, reservplats,
  sabotage …) i samma text ger `security_classified`. Var för sig ger de
  ingenting (`criticalFacilities`, `protectionWords` i
  `internal/classifier/features.go`).
- **Andra åsiktens instruktion:** kod måste finnas i texten för att vara
  källkod; en allmän fråga utan person eller dokument innehåller inga data;
  skydd av samhällsviktig verksamhet är säkerhetsklassat.
- **Finskt identitetsnummer** rapporteras inte längre också som telefonnummer.

## Ärlighet i mätningen

Regeln skrevs med de sju missade frågorna kända, så de fångas per
konstruktion. Korpusen fick två nya känsliga frågor som regeln inte fångar
och två sorters ofarliga motexempel. Att andelen flaggade ofarliga föll från
14 % till 8 % beror på korpusens sammansättning, inte på reglerna; det står i
manualen.

| Endast regler | 1.9.0 | 1.9.1 |
|---|---|---|
| Känsliga flaggade | 81 % | 84 % |
| Säkerhetsklassat, recall | 0,67 | 0,92 |
| Rätt klass exakt | 82 % | 86 % |

## Kvar

- Andra åsikten mätt om efter utrullning (instruktionen är ändrad, så både
  träffar och falsklarm kan ha rört sig).
