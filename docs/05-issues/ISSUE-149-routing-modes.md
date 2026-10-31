# ISSUE-149 — Modellval: lägen (Save / Balanced / Best answer) och uppmätt kvalitet

`epic: EPIC-14` · `priority: P3` · `state: needs-info` · **användardriven**

Byggs när en användare ber om det. Fram till dess gäller "keep it stupid
simple": två inställningar (nivå per modell, regler) förklarade i manualen
(ISSUE-148).

## Idé

Ett enda val för hela instansen, som körlägen i en bil:

| Läge | Betydelse |
|---|---|
| Save | Billigaste tillåtna modell, lokala först |
| Balanced (standard = dagens beteende) | Bra svar till rimligt pris |
| Best answer | Bästa tillåtna modell även om den kostar |

**Princip:** läget påverkar bara steg 2 (valet bland det tillåtna), aldrig
steg 1 (regler, nivågolv, block). "Reglerna bestämmer vad som är tillåtet;
läget bestämmer hur Sluss väljer bland det tillåtna." Ett läge kan aldrig
släppa ut data.

## Förutsättningar och risker

- Sluss har redan lägen internt (`router_mode` cheap/premium per förfrågan,
  conservative mode) — städa upp och visa, inte bygga nytt.
- **Uppmätt kvalitet per uppgift** behövs för att lägena ska göra skillnad.
  Med en gratis lokal premiummodell gör "Best answer" annars nästan ingenting,
  eftersom två modeller på samma nivå antas vara lika bra.
- Namn: "läge" (instansens val) får inte förväxlas med "nivå" (modellens).
- Läge per person i chatten väntar tills vidare.

## Relaterat, också användardrivet

- Styra en viss sorts fråga till en viss modell (t.ex. publik research →
  en specifik molnmodell) från konsolen.
- Valbar regel "datahemvist går före tier-golvet" per policyregel, av som
  standard, synlig i Inspector och audit.
- Jämför två modeller sida vid sida i chatten.
- Konkurrentgenomgång (grindar som Zscaler, konfigurerad routing som
  LiteLLM/Portkey/Intric, inlärd routing som NotDiamond).

## När det blir aktuellt

Skissa konsolvyn först, sedan bygg.
