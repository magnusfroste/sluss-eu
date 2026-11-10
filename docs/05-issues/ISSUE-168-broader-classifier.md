# ISSUE-168 — Bredare klassificerare: namn, fler identifierare, större ordlistor

`epic: EPIC-14` · `priority: P1` · `state: done` · klar 2026-11-10

## Varför

Ägaren: *"vi behöver fånga mycket för att inte uppfattas för smala för en
verksamhet"*, med Zscalers DLP-ordlistor som förebild. Benchmarken (ISSUE-166)
visade dessutom att den största gruppen missar med enbart regler var
namngivna personer.

## Levererat

Allt är regler: listorna står i koden, samma text ger samma svar.

- **Namn i fritext** (`internal/pii/names.go`): förnamn ur en fast lista
  (svenska, de vanligaste i språk som talas här, engelska) följt av ett
  efternamn; partiklar (*von*, *af*, *al*), dubbelnamn och bindestreck. Senare
  omnämnanden av samma för- eller efternamn räknas, så att *mask and send*
  döljer dem (`[NAME_1]`). Ensamt förnamn räknas inte. Kan inte skilja en kund
  från en författare. `ROUTER_NAME_DETECTION=off` stänger av. Svarskontrollen
  flaggar inte namn.
- **Fler identitetsnummer** (`internal/pii/more.go`): finskt och norskt
  (kontrollsiffror), US SSN och UK NINO (formatregler), danskt CPR bredvid
  ordet; passnummer, registreringsnummer och födelsedatum bredvid ordet.
- **Fler nyckelformat** (`internal/secrets/mask.go`): Stripe, GitLab, npm,
  Hugging Face, SendGrid, Twilio, DigitalOcean, Shopify, Azure, webhook-adresser.
- **Större ordlistor** (`internal/classifier/features.go`): hälsa (tillstånd,
  vård, intyg), ekonomi (affärer, kurspåverkande, kredit, skatt, upphandling),
  juridik (domstol, parter, dokument, konkurs, intrång), personregister,
  HR-uppgifter och GDPR:s särskilda kategorier, samhällsviktig verksamhet och
  totalförsvar. Cirka 350 nya termer.
- **Allmän fråga är inte data:** en kort fråga om vad något är eller hur det
  fungerar får ingen klass av ett ord ensamt. Gäller inte om frågan pekar på
  eget material, nämner en person eller innehåller ett värde, en märkning
  eller kod. Säkerhetsmärkningar undantas aldrig.

## Mätt (endast regler)

Korpusen gjordes svårare samtidigt: nya känsliga frågor för det nya, och
ofarliga motexempel skrivna för att fälla de nya reglerna.

| | 1.8.1 | 1.9 |
|---|---|---|
| Känsliga flaggade | 74 % | 81 % |
| Ofarliga flaggade | 10 % | 14 % |
| Bara innebörd | 0 av 58 | 14 av 58 |
| Rätt klass exakt | 81 % | 82 % |

Presidio på samma korpus: 69 % och 24 %. På 1.8.1-korpusen ger de nya
reglerna 80 % och 4 %; den siffran smickrar, eftersom regeln om allmänna
frågor skrevs med de motexemplen kända.

## Kända kostnader

- Frågor om kända personer flaggas (11 i korpusen).
- En affisch om diabetesveckan flaggas: inte en fråga, och ordet är ett hälsoord.
- Orden i listorna matchar hela ord; svenska böjningar utan å/ä/ö måste stå
  med för sig (*fackmedlemmar*, *utvecklingssamtalet*).
- Klassificeringen av mycket stora frågor tar omkring en fjärdedel längre tid.

## Kvar

- Andra åsikten mätt om på den nya korpusen (efter utrullning).
- Regeltexten i konsolen (Inspector) visar typen `name` men förklarar den inte.
