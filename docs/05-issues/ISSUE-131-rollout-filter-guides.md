# ISSUE-131 — Filterguider på Rollout-sidan

- **Epic:** EPIC-12 (compliance-yta)
- **Priority:** P1
- **State:** done (2026-10-25)

## Bakgrund

Rollout-kitet (ISSUE-128a) sa "lägg in denylistan i ert filter och peka
blocksidan mot Sluss" men inte hur. En IT-admin ska klara steget på 15
minuter utan att ringa någon, och veta i förväg om filtret kan skicka
användaren vidare till Sluss eller bara blockera.

## Ändring

- `rolloutguides.go`: steg-för-steg per produkt, med instansens egen
  `/blocked`-URL ifylld: **Microsoft Defender** (Defender for Cloud Apps
  *Unsanctioned* + Enforce app access, eller indikator-import; Network
  Protection i block-läge), **Cloudflare Zero Trust** (lista, DNS/HTTP-
  policy, block page → redirect), **Cisco Umbrella** (destination list,
  Block Page Appearance → Redirect to URL, app-kategori Generative AI),
  **Zscaler ZIA** (custom URL-kategori + AI & ML Applications, custom
  notification med redirect), **övriga DNS-filter** (sinkhole, ärligt om
  att HTTPS-sidan inte kan visas utan cert).
- Varje guide märks **redirects to Sluss** eller **block only**. Defender
  är block only: guiden säger det och lutar sig på web clip + bokmärke.
- Varje guide slutar med samma kontroll: öppna chatgpt.com på en hanterad
  enhet, kör coverage-skriptet, enheten ska rapportera *closed*.
- Ny fil `genai-defender-indicators.csv`: denylistan i Defender for
  Endpoints importformat (DomainName, Block).

Klickvägar ändras mellan releaser; guiderna namnger skärmen, inte pixlar.

## Verifierat

Tester: guiderna bär instansens URL:er och märkningar, Defender är block
only med not, indikatorfilen har rätt header och 12 fält per rad.
