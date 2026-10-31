# ISSUE-155 — Profilsida för den inloggade

`epic: EPIC-14` · `priority: P2` · `state: done`

## Varför

Sluss är nu fleranvändar (chatthistorik per person, ISSUE-154). Den
inloggade behöver se vem hen är, kunna byta sitt lösenord och styra sin egen
chatthistorik.

## Vad

- `/me` för alla inloggade: namn, roll, avdelning och hur man loggade in
  (lokalt konto, företagskonto/SSO, nödlösenord, API-nyckel, demolänk).
- **Byt lösenord** för lokala konton: nuvarande lösenord, nytt två gånger,
  minst 8 tecken. Auditloggas (`admin_user.password_changed`). Rollen ändras
  inte. Nödinloggning och SSO förklarar var lösenordet hör hemma i stället.
- **Chatthistorik:** ladda ner som JSON, radera allt (auditloggas
  `chat.history_deleted`). Request-loggen behålls — den innehåller aldrig
  texten.
- Admins i konsolramen; anställda och demogäster fristående med "Back to the
  chat". Nås från namnet i menyns fot och från identitetsraden i chatten.
- Två felmeddelanden på svenska i Console users ("spara:") nu på engelska.

## Ingår inte (användardrivet)

Roller som val (Admin/Employee) och avdelning för lokala konton, lista över
SSO-användare, inbjudningar, CSV-import, återställning via mejl.
