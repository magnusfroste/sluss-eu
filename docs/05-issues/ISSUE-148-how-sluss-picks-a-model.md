# ISSUE-148 — Manualsida: How Sluss picks a model

`epic: EPIC-14` · `priority: P2` · `state: done`

## Varför

Hur tier-golv, regler och kostnad avgör vart en fråga går fanns inte
beskrivet. Ägaren: "keep it stupid simple — så det går att förklara och
hantera". Underlag: en körning av motorn med lokal modell som balanced
respektive premium (en gratis lokal modell vinner nästan allt; som premium tar
den allt).

## Vad

- `manual/choosing-models.md`: tre steg (ta bort det som inte är tillåtet —
  regler, nivå, nere; välj bästa för priset; annars stopp), nivåtabellen
  cheap/balanced/premium med vilka frågor som kräver vad, "nivån är ditt
  omdöme, inte ett faktum om modellen", konsekvenstabellen, de två
  inställningarna (nivå per modell, regler), hur man ser varför (Inspector →
  Route, dry-run) och en enkel startpunkt. Säger rakt ut att "skicka en viss
  sorts fråga till en viss modell" inte finns i konsolen ännu.
- Länkad från manualens start, Quick start, ordlistan (Level/tier) och
  Models-sidan.
- Manualens Markdown stöder nu `*kursiv*` (utanför `code`), vilket flera
  sidor redan använde.

## Senare

När vad väljs (uppmätt kvalitet per uppgift, styra en frågetyp till en modell,
regeln "datahemvist går före tier-golvet") tas upp separat.
