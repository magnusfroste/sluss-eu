# ADR-0011: Registry använder interna modellprofiler

## Status

Accepterad

## Kontext

Modellnamn och providerutbud ändras. Policy ska inte behöva skrivas om varje gång.

## Beslut

Policy refererar till interna profiler som `cheap`, `balanced-coder`, `premium-reasoning` och registry mappar dessa till faktiska modeller.

## Konsekvenser

Bättre stabilitet och enklare providerbyte. Kräver tydlig registryversionering.

## Alternativ

Policy refererar direkt till provider/model-id.
