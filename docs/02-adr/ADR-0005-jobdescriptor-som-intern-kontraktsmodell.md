# ADR-0005: JobDescriptor som intern kontraktsmodell

## Status

Accepterad

## Kontext

Routingen behöver fler signaler än rå prompttext: tasktyp, risk, metadata, capabilities och budget.

## Beslut

`JobDescriptor` införs som intern representation. Alla routingbeslut baseras på denna struktur.

## Konsekvenser

Bättre testbarhet och simulation. Kräver att feature extraction och SDK-signaler hålls konsekventa.

## Alternativ

Låt routing läsa direkt från request body; använd endast prompttext.
