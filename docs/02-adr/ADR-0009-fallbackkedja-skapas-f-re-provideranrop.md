# ADR-0009: Fallbackkedja skapas före provideranrop

## Status

Accepterad

## Kontext

När primär provider timear ut finns inte tid att göra ett nytt tungt routingbeslut.

## Beslut

Routingbeslutet innehåller primär modell, fallbackmodeller och timeout per attempt.

## Konsekvenser

Snabbare failover och bättre loggning. Kräver att fallback candidates filtreras innan.

## Alternativ

Beräkna fallback först vid fel; ingen fallback i MVP.
