# ISSUE-146 — Manualen i konsolramen

`epic: EPIC-14` · `priority: P3` · `state: done`

## Varför

Klickade en admin på Manual i menyn öppnades en fristående sida utan
konsolens vänstermeny — svårt att komma tillbaka.

## Vad

- Inloggad admin läser manualen i konsolramen: vänstermenyn kvar (Manual
  markerad), manualens sidor som innehållslista bredvid texten (på mobil en
  rad som scrollar).
- Utloggad besökare får den publika manualen som förut — den är publik med
  avsikt, en admin behöver den innan inloggning.
- En anställd eller demogäst får den publika manualen med "← Back to the chat".
- `AdminAuth.Optional`: lägger in sessionens användare och roll om det finns
  en, annars serveras sidan ändå.
- Manualens gamla `nav`-stil är avgränsad så den inte slår igenom på
  konsolmenyn.

## Klart när

- `manual_console_test.go`: admin får konsolram, utloggad och anställd den
  publika sidan.
- Webbläsare desktop och mobil: menyn kvar, tillbaka till Models med ett klick.
