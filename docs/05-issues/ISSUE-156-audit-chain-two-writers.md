# ISSUE-156 — Auditkedjan förgrenas när två processer skriver samtidigt

`epic: EPIC-14` · `priority: P0` · `state: done`

## Fel

`audit-verify` på demons export: *CHAIN BROKEN after 451 valid records*.
Undersökning: tre brott (2026-10-01 två gånger, 2026-10-31 01:02), alla
precis efter en uppstart. EasyPanel startar den nya containern innan den gamla
stängs; båda skrev till samma `audit-chain.jsonl` och var och en fortsatte från
sin egen minnesbild av sista raden — kedjan förgrenades. Inga rader ändrade,
inga borttagna: alla 678 sekvensnummer finns, tre är dubbletter.

## Rättelse

- **Skrivaren:** varje append tar ett exklusivt `flock` på filen och läser den
  verkliga sista raden från disk innan seq/prev_hash sätts. Två processer kan
  inte förgrena kedjan längre. (Ingen lås-plattform → oförändrat beteende.)
- **Verifieraren:** `audit-verify` säger fortfarande BROKEN (exit 1) men
  förklarar hela filen: ändrade rader (hash stämmer inte), borttagna (länk till
  okänd rad, saknade sekvensnummer) och förgreningar (länk till en tidigare
  rad). Är allt förgreningar med intakta rader och inget saknas står det
  uttryckligen.
- Manualen (Show that it works) nämner förklaringen.

## Befintlig kedja

Historiken kan inte lagas — det är poängen med en hashkedja. Demons fil
fortsätter verifiera som BROKEN vid de tre punkterna, nu med förklaring. Vill
man ha en ren kedja för en demo: flytta filen åt sidan (behåll den som bevis)
så startar en ny vid nästa uppstart.

## Test

- `TestChainTwoWritersOnOneFileStayLinked` — två sinks på samma fil; föll
  utan rättningen ("sequence 2 does not follow 2"), passerar med.
- `TestDiagnoseChain` — förgrening, ändrad rad och borttagen rad skiljs åt.
