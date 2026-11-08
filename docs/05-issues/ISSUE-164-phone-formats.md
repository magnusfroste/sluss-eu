# ISSUE-164 — Fler telefonformat, färre falsklarm

`epic: EPIC-14` · `priority: P2` · `state: done` · klar 2026-11-08

## Varför

Svarskontrollen (ISSUE-152) missade `(555) 123-4567` i ett live-svar på demon:
telefondetektorn är en blocklista av mönster, och formatet fanns inte. En sond
visade dessutom tre falsklarm som redan fanns: ett datum (`08-11-2024`), ett
ordernummer och ett fakturanummer räknades som telefonnummer.

## Levererat

- Nya mönster, alla validerade efteråt: `00`-prefix (10–17 siffror),
  riktnummer i parentes, `+1 (555) …`, nordamerikanska `555-123-4567` och
  `555.123.4567` (exakt tio siffror).
- Datum med avskiljare räknas inte. En sifferföljd direkt efter en etikett
  som *order*, *faktura*, *ärende*, *kundnr*, *org.nr*, *konto* m.fl. räknas
  inte (helt ord, så *David* är inte *id*).
- Tester för format och förväxlingar; korpusen 46 → 49 fall.

## Princip

Detektorerna är en blocklista av mönster med validering: precisa och
förklarbara. Den som vill ha allowlist-läge gör det på policynivå (default
lokalt, moln bara för namngivna frågetyper), och den lokala andra åsikten
täcker betydelse utan format.
