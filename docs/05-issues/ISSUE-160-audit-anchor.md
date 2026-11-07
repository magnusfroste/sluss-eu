# ISSUE-160 — Förankra auditkedjan utanför maskinen

`epic: EPIC-12` · `priority: P1` · `state: done` · klar 2026-11-07 (#70)

## Varför

"Vad hindrar en admin med diskåtkomst från att skriva om kedjan och räkna om
hasharna?" Inget, innan detta. En hashkedja bevisar att inget *inuti* filen
ändrats; den kan inte bevisa att hela filen inte skrevs om från en vald punkt.

## Levererat

- Loggraden `audit anchor` (seq + entry_hash för sista posten) vid start och
  varje timme (`ROUTER_AUDIT_ANCHOR_INTERVAL`; `off` stänger av). Skeppad med
  loggen till SIEM blir den beviset.
- `audit-verify -anchor <seq>:<hash> export.jsonl`: en omräknad kedja passerar
  den vanliga kontrollen men faller på ankaret; en trunkerad export likaså.
- `GET /router/audit/anchor` för ett skript som arkiverar ankaret på annat håll.

Ingen nyckel, ingen signatur, ingen ny tjänst: operatören skeppar redan loggar.

## Kvar (backlogg)

Förseglad rotation: signerad överlämning från arkiverad kedja till ny.
