# ADR-0012: Verifiering används selektivt

## Status

Accepterad

## Kontext

Verifiering kan höja kvalitet men ökar kostnad och latency.

## Beslut

Verifiering aktiveras via policy för high-risk tasks och via cascade för oklara fall. Låg-risk tasks verifieras inte i MVP.

## Konsekvenser

Bättre riskhantering. Kräver tydliga triggers och kostnadsmätning.

## Alternativ

Verifiera allt; verifiera inget.
