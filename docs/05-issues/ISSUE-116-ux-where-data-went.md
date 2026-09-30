# ISSUE-116 — UX: "Where your data went" först, egress i loggen, policyn i klartext

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P0/P1 (demo 2026-10-02)
- **State:** done (2026-09-30)

## Bakgrund

UX-genomgång som förstagångsbesökare (skärmdumpar av varje sida). Konsolen
såg proffsig ut men svarade inte tydligt på köparens fråga *vart tog datan
vägen?* — och hade en bugg som syns på scen.

## Ändring

1. **Bugg: "Personal data detected" visade 0** trots att personnummer
   skickats. Kortet räknade bara PII i *blockerade* audit-poster, så PII som
   routats lokalt syntes inte (introducerad i ISSUE-114). Ersatt av
   **"Personal data (PII)"** från historiken: totalt, "N kept in the house ·
   M blocked", och rött "N went to the cloud" om policyn tillät det.
2. **Evidenslänkarna** på dashboarden var webbläsarblå med understrykning på
   mörk botten — nu stylade som länklista.
3. **Egress sparas per request** (`requests.egress`, migrering): beslutet
   skriver primärens egress, det *svarande* försöket skriver om den (en
   fallback kan byta). En regel för allt — `egressFromTags` — delas av
   `X-Router-Egress`, loggen och dashboarden. Rader från före kolumnen
   klassas från modellens nuvarande taggar.
4. **Request log:** ny kolumn **Egress** (local/cloud/blocked-pill),
   blockerade rader tonade, provider under modellen, tokens i en kolumn —
   tabellen får plats i 1280 px (kostnaden klipptes tidigare).
5. **Dashboard leder med "Where your data went":** kort *Stayed in the
   house / Blocked fail-closed / Personal data / Evidence*, en staplad
   local/cloud/blocked-stapel och en tabell per känslighetsklass där en
   känslig klass i moln-kolumnen flaggas "left the house — allowed by your
   policy". Besparingen kommer därefter. Totalen säger "retained history"
   i stället för felaktiga "since last restart" när SQLite används.
6. **Policysidan:** aktiva regler i klartext (`CompiledPolicy.Summaries`:
   "data is pii → only models tagged local", block i rött), **dry-run direkt
   efter**, regeleditorn, paketen, och **"Data, retention & danger zone"**
   (reset/radering) sist med röd ram. JS-felmeddelande "fel" → "error".

## Tester

- `TestEgressStoredAndCorrectedByAnsweringAttempt` (history)
- `TestSummariesReadLikeFirewallRules` (policy)
- `TestDataFlowFromHistory` — legacy-rader, PII-kortet, dashboard- och
  logg-HTML
- `TestPolicyPageShowsActiveRulesAndOrder` — klartextregler + sektionsordning
- Skärmdumpar av dashboard, logg, policy och dry-run mot lokal instans.

## Kvar till ISSUE-117 (efter demo)

Klick på loggrad → beslutsförklaring + filter; local/cloud-bricka och Edit per
rad på Models; fäll ihop tomma avancerade dashboard-sektioner; läsbarare
belopp.
