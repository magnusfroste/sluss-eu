# ISSUE-144 — Modern chatt

`epic: EPIC-14` · `priority: P2` · `state: done`

## Varför

Chatten såg ut som en demo: sessionslistan till höger, ett enradigt fält,
svar i bubblor som ren text (ingen formatering av kod, listor eller tabeller),
ingen stopp-knapp. Användare jämför med de stora AI-assistenterna.

## Vad

- **Historik till vänster**, grupperad Today / Yesterday / Previous 7 days /
  Previous 30 days / Older, med sökning, New chat och radera. Kan fällas in
  (minns valet); på smal skärm en låda med bakgrund.
- **Tom chatt:** hälsning och inmatningsrutan centrerad, förslagen (admin-
  hanterade quick prompts) under. Chatten öppnar alltid på en ny konversation.
- **Inmatning:** växande textruta i en rundad ruta; Enter skickar,
  Shift+Enter ny rad; Attach och Search som knappar i rutan; filer kan dras in.
  Skicka-knappen blir Stopp medan svaret strömmar (även Escape).
- **Svar** utan bubbla, med avatar, renderade som Markdown: rubriker, listor
  (nästlade), kodblock med språk och kopieringsknapp, tabeller, citat, länkar.
  Renderaren escapar all modelltext innan taggar läggs till och tillåter bara
  http(s)-länkar — modellsvar kan aldrig injicera HTML eller skript.
- **Under varje svar:** Copy, och Regenerate på det senaste (ersätter svaret,
  routern väljer på nytt).
- **Kvar från förut:** routing-etiketter (uppgift, modell, pinned, fallback,
  webbsök), notisen när personuppgifter styr routningen, policyblock som kort,
  källor (nu som numrerade länkar), modellväljaren för admin, gästläget utan
  meny, anställdläget med namn och avdelning.
- Följer svaret bara när läsaren är längst ner; annars en knapp "till senaste".

## Klart när

- Befintliga tester gröna (modellväljaren, gästläget).
- Markdown-renderaren provad med rubrik, listor, kod, tabell, citat, oavslutat
  kodblock och injektionsförsök (`<script>`, `<img onerror>`, `javascript:`).
- Webbläsarkontroll desktop och mobil: tom chatt, strömmat svar, Regenerate
  ersätter, historiklåda på mobil, riktigt anrop genom routern med PII-notis.

## Rättelse 2026-10-31

Att fälla in historiken fick panelen att glida åt vänster över adminmenyn
(negativ marginal), och mellan 760 och 900 px blandades skrivbords- och
mobilläget. Nu krymper panelen på bredden inom chattytan, döljs för
tangentbordsfokus när den är stängd, och under 900 px är den alltid en låda
(Escape eller klick utanför stänger). Fokus flyttas med panelen.
