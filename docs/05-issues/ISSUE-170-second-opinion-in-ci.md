# ISSUE-170 — Andra åsikten mätt i CI

`epic: EPIC-15` · `priority: P2` · `state: done` · klar 2026-11-10

## Varför

Andra åsikten mättes bara när någon körde benchmarken för hand mot en
instans. En regeländring som gjorde kombinationen sämre, eller en ändrad
instruktion till modellen, kunde gå obemärkt.

## Vägval

En modell kan inte frågas från ett bygge utan nätverk och nyckel, och CI ska
vara hermetiskt. Därför spelas modellens svar in en gång och spelas upp.

## Levererat

- `benchmark record -url … -key …` frågar en instans (som måste ha andra
  åsikten påslagen) och skriver `benchmark/second-opinion.jsonl`: per fråga
  de klasser modellen gav, plus modellnamn, datum, commit och ett
  fingeravtryck av instruktionen (`secondopinion.PromptHash`).
- `benchmark run -second-opinion <fil>` lägger inspelningens klasser till
  dagens regler (bara skärpning, som i routern), utan modell och nätverk.
- `make benchmark` kör det i CI med golv på andel fångade och falsklarm.
- Inspelningen vägras om frågorna (hash per fråga) eller instruktionen har
  ändrats; felmeddelandet säger hur man spelar in på nytt.

## Vad det visar och inte visar

Visar: vad dagens regler och den inspelade modellen ger tillsammans; fångar
regeländringar som försämrar paret. Visar inte: hur modellen skulle svara på
en ny fråga, eller hur en annan modell svarar.

## Småsaker i samma ändring

- Chattens inspector skriver de nya typerna i ord.
- ISSUE-151 stod kvar som öppen på en rad i indexet; borttagen (levererad).
