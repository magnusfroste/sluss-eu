# ADR-0007: Eventlogg för beslut, attempts och outcomes

## Status

Accepterad

## Kontext

För att förbättra routern krävs mätdata om beslut, kostnad, latency och outcome.

## Beslut

Varje request loggar decision event, provider attempts och eventuellt outcome.

## Konsekvenser

Det möjliggör dashboard, evals och förbättring. Privacy och retention måste hanteras noggrant.

## Alternativ

Endast aggregerade metrics; ingen requestnivå-loggning.
