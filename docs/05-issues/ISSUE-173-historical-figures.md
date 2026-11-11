# ISSUE-173 — Historiska personer är inte personuppgifter

`epic: EPIC-14` · `priority: P3` · `state: done` · klar 2026-11-11

## Varför

Namnregeln (ISSUE-168) kan inte skilja en kund från en författare. En
skoluppsats om Astrid Lindgren hölls lokalt som personuppgift, och frågor om
kända personer stod för de flesta kvarvarande falsklarmen med enbart regler.

## Vägval

En lista över kända personer blir aldrig komplett, och levande offentliga
personer är fortfarande levande personer med dataskydd. Listan innehåller
därför bara personer som inte längre lever (cirka 250: författare, forskare,
konstnärer, statsmän). Den ska inte växa till att täcka levande personer; de
flaggas även i fortsättningen, och det är en avsiktlig kostnad.

## Levererat

- `internal/pii/historical.go`: listan och `isHistoricalFigure` (även
  genitiv: *Lindgrens*, *Darwin's*). `findNames` hoppar över en träff på
  listan; ett personnummer, en adress eller ett hälsoord i samma text räknas
  som förut.
- Förnamnet *Vilhelm* saknades i namnlistan (hittat av ett nytt test).
- Korpusen fick fyra namn till: två levande offentliga personer (flaggas,
  avsiktligt) och två historiska som listan inte har (flaggas, ärlig miss).
- Andra åsikten inspelad på nytt för den ändrade korpusen.

## Mätt

Endast regler: ofarliga flaggade 17 → 16 av 212; rätt klass exakt 86 % →
87 %. Med inspelad andra åsikt: 97 % av känsliga, 9 % av ofarliga. På
korpusen är ändringen alltså en fråga; värdet ligger utanför den, i varje
fråga om en bok, en teori eller en epok.

## Kvar

- *Karl Johans gate* läses som ett namn (gatunamn efter en person).
