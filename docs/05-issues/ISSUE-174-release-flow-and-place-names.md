# ISSUE-174 — Releaseflödet, demons versionsnummer och platser med personnamn

`epic: EPIC-15` · `priority: P3` · `state: done` · klar 2026-11-11

## Varför

Vid v1.11.0 hände två saker:

1. Release-flödet startades direkt efter mergen och kördes på commiten före,
   där changeloggen saknade avsnittet. Det vägrade, och fick startas om.
2. Demon visade inget versionsnummer efteråt. Orsaken var äldre än
   omstarten: utrullningen sker efter bygget från main, vars image saknar
   versionsstämpel. Den stämplade imagen byggs för taggen, strax efter, och
   skrivs också till `latest`, men ingenting rullade ut den. Att demon förr
   visade rätt version berodde på att de två byggena råkade bli klara i rätt
   ordning.

## Levererat

- `release.yml`: steget *Use the newest main* hämtar main, väntar upp till
  två minuter på att changeloggen har avsnittet, och taggar den commiten.
- `publish-image.yml`: jobbet `deploy` körs även för en tagg och väntar då på
  att `/version` rapporterar taggen, inte bara commiten. Samtidighetsgruppen
  är skild per slag av ref, så att taggens utrullning inte avbryter mains.
- Namnregeln: ett namn följt av ett platsord (*gate*, *torg*, *gymnasium*,
  *road* …) är en plats (`placeFollows` i `internal/pii/names.go`).
- *Remove* på en gällande regel på Policy-sidan klickad i webbläsare: regeln
  försvann ur listan och slutade gälla.

## Inte verifierat

Ändringarna i flödena går bara att pröva genom en release. Nästa release
visar om demon rapporterar versionen av sig själv.
