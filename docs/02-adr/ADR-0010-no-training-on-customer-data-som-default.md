# ADR-0010: No training on customer data som default

## Status

Accepterad

## Kontext

Användare behöver tillit. Routingdata är värdefull men promptinnehåll kan vara känsligt.

## Beslut

Kunddata används inte för modellträning utan explicit opt-in. Outcome-aggregat kan användas anonymiserat enligt policy.

## Konsekvenser

Starkare privacy-position. Mindre data för snabb ML-utveckling.

## Alternativ

Opt-out istället för opt-in; träna endast på allt internt.
