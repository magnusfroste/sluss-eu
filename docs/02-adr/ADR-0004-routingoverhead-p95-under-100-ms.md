# ADR-0004: Routingoverhead p95 under 100 ms

## Status

Accepterad

## Kontext

Om routern känns långsam kommer användare att stänga av den. Modellanrop är redan långsamma; extra overhead måste hållas låg.

## Beslut

Fast path får inte göra DB-slag, externa prisuppslag eller LLM-klassificering. Policy och registry hålls i minne.

## Konsekvenser

Arkitekturen måste prioritera cache, async logging och mätning. Vissa features får skjutas till slow path.

## Alternativ

Acceptera högre latency för bättre klassificering; routea endast per session istället för per prompt.
