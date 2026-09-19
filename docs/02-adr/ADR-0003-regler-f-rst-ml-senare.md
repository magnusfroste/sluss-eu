# ADR-0003: Regler först, ML senare

## Status

Accepterad

## Kontext

Målet är låg latency och förklarbara beslut. En LLM-baserad classifier för varje prompt skulle öka både kostnad och latency.

## Beslut

MVP använder regler, snabb feature extraction och policy. Lightweight classifier kan läggas till för osäkra fall.

## Konsekvenser

MVP blir snabbare och mer förklarbar. Precisionen kan vara sämre i oklara fall, men konservativ routing minskar risk.

## Alternativ

Premium LLM-classifier för varje prompt; tränad routermodell från dag ett.
