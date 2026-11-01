# ISSUE-157 — Klassificeraren ser nyckelvärden och svenska säkerhetsskyddsmarkeringar

`epic: EPIC-14` · `priority: P0` · `state: done`

## Fel (hittat vid CISO-test på demon 2026-11-01)

| Fråga | Före | Efter |
|---|---|---|
| AWS-nyckel `AKIA…` | ingen klass → moln | secrets_possible → lokalt |
| `aws_secret_access_key = …` | ingen → moln | secrets_possible |
| `sk-…`, `ghp_…`, PEM private key, Slack-token | (ingen om ordet "key" saknades) | secrets_possible |
| "HEMLIG enligt säkerhetsskyddslagen" | ingen → moln | security_classified |
| "BEGRÄNSAT HEMLIG" | ingen → moln | security_classified |

Orsak: detektorerna för nyckelvärden fanns (`internal/secrets`, ISSUE-042) men
användes bara för att maskera loggar. Klassificeraren reagerade bara på ord som
"api key" och "password".

## Rättelse

- Klassificeraren kör `secrets.Detect` på varje meddelande: innehåller det en
  nyckel/token/privat nyckel → `secrets_possible` (samma som när man skriver om
  nycklar).
- Nya mönster: `aws_secret_access_key`, PEM private key, Slack-token. Varje
  mönster har en billig förkontroll (delsträng) innan regexen — 0,3 ms för en
  stor prompt (45 kB) i stället för 4 ms.
- Svenska säkerhetsskyddsmarkeringar: säkerhetsskyddslagen,
  säkerhetsskyddsklass, begränsat hemlig, hemlig enligt, konfidentiell enligt,
  samt NATO/EU CONFIDENTIAL/SECRET.

## Medvetet inte ändrat

- **En dataklass per fråga, hemligheter rankas högst.** "classified SECRET"
  blir secrets_possible (stannar lokalt) och träffar därför inte en regel som
  blockerar security_classified. Säkert, men en regel på "säkerhetsklassat →
  blockera" kan överraskas. Kandidat: matcha regler mot alla dataklasser i
  frågan, inte bara den högst rankade.
- "CONFIDENTIAL – internal only" och opublicerade finansiella siffror
  klassas inte. Företagsinterna markeringar är vanliga och skulle med en
  blockeringsregel stoppa mycket — ett policybeslut, inte en detektorfråga.
