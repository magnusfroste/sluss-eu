# ADR-0002: Providerabstraktion med intern normaliserad request

## Status

Accepterad

## Kontext

Olika providers har olika format för messages, tool calls, streaming, token usage och fel.

## Beslut

Alla inkommande requests normaliseras till `NormalizedModelRequest`. Provideradaptrar mappar sedan till provider-specifikt format.

## Konsekvenser

Det blir enklare att lägga till providers och fallback. Tool-calling och streaming kräver dock noggranna contract tests.

## Alternativ

Använda endast en hosted gateway; hårdkoda varje provider i routingen.
