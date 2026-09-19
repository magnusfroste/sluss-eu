# ISSUE-101 — Landing: köparpositionering (risk, USP:ar, CTA-fix)

- category: enhancement
- state: done
- epic: EPIC-15
- priority: P1
- type: product|frontend
- sprint: 09
- klar: 2026-07-12

## Bakgrund

Analys 2026-07-12: sidan ledde med mekanism, inte risk; USP:ar listades som
features utan "unlike"-sats; self-hosted (största förtroendepunkten) saknades;
CTA:erna pekade in i login-väggen. Beslut: ingen lead-capture här — CRM ägs av
ett externt CRM; portalen ÄR produkten och sidan ska FÖRSTÄRKA budskapet för CISO:n
som kommer via en delad demolänk.

## Levererat

- Hero: "Your teams already use AI. Decide where the data goes — and prove
  it." — utfall före mekanism; deterministisk-raden kvar som meta.
- Ny sektion **"The risk you remove — Say yes to AI, on your rules"**: Shadow
  AI (gap-rapporten som svar), evidensgapet ("Where does our AI data go?"),
  och motrisken ("Blocking AI creates shadow AI") — enablement-framing.
- Ny sektion **"Why Tokenizer"** med unlike-satsen som rubrik ("Generic AI
  gateways optimise traffic. Tokenizer decides where your data is allowed to
  go — and proves it.") + sex USP-kort: deterministisk/förklarbar, fail-closed,
  **self-hosted i din infrastruktur**, evidens, monitor-först, ROI.
- CTA-fix: inga länkar in i login-väggen (`/chat` borta som CTA, dashboard
  endast i nav); primär CTA ankar till risksektionen, `/connect` kvar. Ingen
  lead-capture (ett externt CRM).
- FAQ +2: self-hosted-frågan + incidentrapporterings-frågan. `llms.txt` bär
  samma positionering (AEO).

## Verifiering

Nya asserts i landing_test (budskap, CTA-regler, llms.txt-positionering);
livekörning verifierade H1/H2-sekvensen och Positioning-avsnittet.
