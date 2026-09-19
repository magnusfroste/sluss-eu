# ISSUE-089 — Demo-leads + namngivna demo-länkar (vem har provat?)

- `epic: EPIC-15`
- `priority: P2`
- `type: product`
- `sprint: backlog`
- `state: ready-for-agent`

## Idé (ägarens, 2026-07-08)

1. **Lead-capture på landningssidan**: besökaren anger e-post + uttryckligt
   samtycke → får demo-länken (MVP: visas direkt; senare tillval: mejlas, vilket
   kräver SMTP-infra). GDPR-hanteringen ska vara *exemplarisk* — den ÄR demon
   för ett compliance-bolag: tydligt syfte, kort retention, raderbar, loggad i
   vår egen tamper-evidenta audit-kedja ("så här hanterar vi din enda uppgift").
2. **Namngivna/per-mottagare demo-länkar**: dagens delade token (ISSUE-087) kan
   inte skilja mottagare åt. Ge varje länk en etikett (företag/namn/e-post) →
   `token → {etikett, skapad, #öppningar, senast öppnad}`.
3. **Admin-vy "Demo-länkar"**: tabell etikett · skapad · #öppningar · senast
   öppnad + rotera/stäng av per länk. Det är "vem har provat"-tavlan =
   bekräftelsen. Audit-aktör = etiketten (inte generiska `demo-guest`).
4. Senare tillval: riktig notis (mejl/Slack/webhook) vid första öppning.

## Beslut som väntar

- Notis: räcker admin-vyn eller ska pling byggas (och vart)?
- Mejlutskick av länken (kräver SMTP-provider + SPF/DKIM) eller visa-direkt.

## Beroenden

Bygger på ISSUE-087 (demo-session/roll) och audit-kedjan (ISSUE-075).
Lead-lagring i SQLite (KV eller egen tabell) + retention-sweep (ISSUE-045-mönstret).
