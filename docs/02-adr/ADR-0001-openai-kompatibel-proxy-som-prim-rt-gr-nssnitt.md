# ADR-0001: OpenAI-kompatibel proxy som primärt gränssnitt

## Status

Accepterad

## Kontext

Klienter och agenter har redan stöd för OpenAI-liknande API:er. För att sänka adoptionströskeln ska användaren kunna byta base URL utan att skriva om sin klient.

## Beslut

Tjänsten exponerar `/v1/chat/completions` i MVP och använder `model: auto` som standard för routing. Routermetadata skickas via headers eller `metadata`-fält.

## Konsekvenser

Adoption blir enklare. Nackdelen är att OpenAI-formatet inte uttrycker alla routingbehov, vilket kräver extra metadata och headers.

## Alternativ

Eget API från start; SDK-only; direkt integration med varje agent.
