# ADR-0008: Asynkron loggning för icke-kritiska events

## Status

Accepterad

## Kontext

Synkron logging kan förstöra latencybudgeten.

## Beslut

Decision/attempt events enqueueas asynkront. Kritiska audit-events kan skrivas synkront.

## Konsekvenser

Lägre latency. Kräver kö, retry och hantering av backlog.

## Alternativ

Skriv allt synkront; skriv inget per request.
