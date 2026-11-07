# ISSUE-162 — Positionering: "the AI gateway whose decisions you can read" och sidan What Sluss catches

`epic: EPIC-15` · `priority: P1` · `state: done` · klar 2026-11-07

## Varför

Vi sa *vad* (inga LLM i beslutet) men inte *varför* en CISO ska bry sig, och
inte *vad vi medvetet avstår från*. En erfaren CISO frågar "vad missar ni?" och
vi hade inget mätt svar.

## Levererat

- USP-text i README och manualens startsida: beslut man kan läsa, samma fråga
  samma beslut, inget lämnar maskinen för att klassas, ingen andra AI att
  styra. Priset sagt rakt ut: regler hittar identifierare, nycklar, markeringar,
  termer och kod, inte betydelse.
- Manualsidan **What Sluss catches, and what it does not** med mätta siffror
  från en fast korpus (46 fall, `internal/classifier/catch_corpus_test.go`) som
  körs i varje ändring; testet faller om sidan blir inaktuell. Dokumenterade
  missar och hur man sluter gapet (egna termer, default lokalt, inspektorn,
  lokal andra åsikt på roadmapen).
- Korpusen avslöjade luckor som täpptes: telefonnummer som folk skriver dem,
  svenska postadresser, sjukskriven/burnout/cellgift som hälsoord, kod utan
  staket.

## Mätt (v1.3)

36 av 46 fall klassade som en noggrann läsare skulle; 7 av 7 harmlösa lämnas
ifred. Missarna är alla "betydelse utan identifierare, markering eller term".
