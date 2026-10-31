# ISSUE-154 — Chatthistorik per person

`epic: EPIC-14` · `priority: P1` · `state: done`

## Fel

Bara anställda via SSO hade egen chatthistorik. Alla andra — admin-konton,
nödinloggningen, API-nyckeln, lokala anställdkonton och gäster via
demolänken — delade en gemensam historik på servern. En prospekt med
demolänk kunde se adminkontonas chattar, och admins varandras. Webbläsarens
lokala kopia delades dessutom mellan alla som loggade in på samma dator.

## Rättelse

- En historik per identitet på servern (`demo_sessions:<user>`). Gäster via
  samma demolänk delar den länkens historik (`demo:<label>`), aldrig någon
  annans.
- Webbläsarens kopia är per person (`tok_demo_sessions_v1:<user>`).
- "Reset demo data" rensar allas historik (`KVDeletePrefix`).
- Den gamla gemensamma historiken visas inte längre för någon; den rensas vid
  nästa "Reset demo data".

## Test

`chathistory_test.go`: två admins, demolänk, demogäst och nödinloggning ser
aldrig varandras historik; reset rensar allt.

## Nästa

Users med roller/avdelning och profilsidan `/me` (ladda ner / radera sin
historik) — förslaget från 2026-10-31, väntar på "kör".
