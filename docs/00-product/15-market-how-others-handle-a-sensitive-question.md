# 15 — Marknadsresearch: hur andra hanterar en känslig fråga (2026-11-15)

> Beställt av ägaren efter v1.11.1: *hur gör andra på marknaden som försöker
> adressera det vi adresserar?* Kompletterar `11-ecosystem-players.md`
> (aktörer, finansiering, juli 2026) med en annan vinkel: **vad som händer
> med en känslig fråga** hos var och en, och var de sitter.
>
> **Underlag och ärlighet:** en sökrunda 2026-11-15, till största delen
> leverantörernas egna sidor och dokumentation. Påståendena är deras och
> inte oberoende verifierade. Inga priser, inga oberoende mätningar av hur
> mycket någon fångar. Det som inte gick att få fram står sist.

## De fyra grupperna

| Grupp | Exempel | Var de sitter | Vad de gör med en känslig fråga |
|---|---|---|---|
| Nätverksgrindar | Zscaler, Netskope, Microsoft Purview, Cloudflare | I nätverket, webbläsaren eller på datorn | Blockerar, varnar eller maskerar |
| AI-gateways för utvecklare | LiteLLM, Kong, Portkey | Som adress för appar och verktyg | Maskerar eller blockerar via tillägg |
| AI-säkerhetsspecialister | Lakera, Prompt Security, Nightfall, Harmonic | Proxy eller API framför modellen | Maskerar, blockerar eller coachar |
| Europeiska suveräna plattformar | Intric, Langdock, Mistral | Egen chatt och egen drift | Håller allt i EU eller på egen hårdvara |

## Hur var och en gör

### Nätverksgrindar

- **Zscaler** dekrypterar AI-trafiken och låter DLP-motorn läsa prompten.
  Beslutet väger användare, målapp och datatyp; utfallet är tillåt, maskera
  eller blockera. De beskriver ordlistor och "intent-based guardrails", och
  kontrollerar även svaret (promptinjektion, skadliga länkar).
- **Netskope** sitter i trafikvägen och lägger till coachning: en ruta som
  påminner om policyn och som användaren måste kvittera.
- **Microsoft Purview** styr främst via Endpoint DLP och ett webbläsartillägg
  som varnar eller stoppar inklistring i ChatGPT och liknande. Starkast i
  Copilot, delvis för andra AI-tjänster.
- **Cloudflare** har PII-detektering i sin brandvägg för AI, byggd på
  Presidio enligt deras egen blogg, och DLP-profiler för prompter till
  ChatGPT, Gemini, Claude och Perplexity.

### AI-gateways

- **LiteLLM** maskerar personuppgifter genom Presidio, som man driftar själv
  som två separata tjänster. Maskera eller blockera per typ; kan stängas av
  per nyckel.
- **Kong** har en PII-tvättare som bara finns i Enterprise och anropar en
  extern tjänst, kombinerad med en semantisk spärr som jämför prompter mot
  listor via inbäddningar.
- **Portkey** heter numera Prisma AIRS AI Gateway. Maskeringen är
  enkelriktad: värdena sätts inte tillbaka i svaret.

### AI-säkerhetsspecialister

- **Lakera**: starkast på promptinjektion; ingår nu i Check Point.
- **Prompt Security**: transparent proxy; ingår i SentinelOne.
- **Nightfall**: moln-DLP som maskerar i prompter.
- **Harmonic**: coachar användaren i stället för att blockera.
- Sökningen gav inget tydligt svar på om de bygger på modeller eller regler.

### Europeiska plattformar

- **Intric** (svenskt): EU-moln, egen drift eller helt isolerad installation;
  skriver att policyer avgör vilka modeller som körs var. Sidan förklarar
  inte hur styrningen går till. Närmast Sluss i både marknad och idé.
- **Langdock**: flera modeller på EU-infrastruktur.
- **Mistral**: drift i Europa eller hos kunden.

## Vad det betyder för Sluss

- **Utfallen är smalare hos de andra.** Nästan alla blockerar eller maskerar.
  Att automatiskt skicka frågan till en modell på egen hårdvara fanns bara
  antytt hos Intric.
- **Återställande maskering är ovanlig.** Portkey säger uttryckligen att
  deras är oåterkallelig. Sluss sätter tillbaka värdena i svaret.
- **Presidio är marknadens standardmotor för personuppgifter.** Cloudflare
  och LiteLLM bygger på den. Jämförelsen i `manual/benchmark.md` är alltså en
  jämförelse mot det flera konkurrenter faktiskt kör.
- **Ingen talar om samma svar varje gång eller om att beslutet går att
  läsa.** Zscalers "intent-based" pekar åt motsatt håll. Det är Sluss
  tydligaste egna position. Inga belägg hittades för att kunder efterfrågar
  just det.
- **Konsolidering pågår.** Lakera, Prompt Security och Portkey har köpts upp
  av stora säkerhetsbolag. En fristående, öppen och självdriftad produkt
  blir ovanligare.

## Där de andra är starkare

- **Täckning utan att användaren gör något.** Nätverksgrindarna fångar all
  trafik. Sluss kräver att verktygen pekas om och att övriga dörrar stängs
  med Entra, Intune och DNS-filter (`manual/one-address.md`).
- **Coachning.** Netskope och Harmonic förklarar för användaren i stunden.
- **Promptinjektion och skadligt innehåll i svar.** Kärnan hos Lakera och
  Zscaler. Sluss flaggar bara värden i svaret.
- **Filer och bilder.** Zscaler och Netskope läser uppladdade filer. Inte
  undersökt närmare.

## Positioneringskartan

`assets/positioning-map.svg` (och `.png`) placerar aktörerna ovan på två
axlar, valda för att visa Sluss position:

- **Vågrätt: vad som händer med en känslig fråga.** Från *stoppas* via
  *maskeras* till *besvaras ändå, på egen hårdvara*.
- **Lodrätt: vems kontroll.** Från *leverantörens moln, sluten motor* till
  *egna servrar, regler som går att läsa*.

Placeringarna är vår läsning av varje leverantörs offentliga dokumentation
och visar angreppssätt, inte kvalitet. Axlarna är valda till Sluss fördel;
på en axel för täckning utan omkonfigurering, eller för skydd mot
promptinjektion, hade Sluss hamnat lågt. Den som använder bilden utåt ska
kunna stå för varje placering: jämförande reklam ska vara sann och gå att
belägga.

## Vad som inte gick att få fram

- Oberoende mätningar av hur mycket någon fångar.
- Priser.
- Nordiska konkurrenter utöver Intric.
- Om specialisterna bygger på modeller eller regler.

## Källor

- Zscaler: <https://www.zscaler.com/blogs/product-insights/secure-generative-ai-usage-in-enterprise>, <https://www.zscaler.com/products-and-solutions/ai-guard-for-apps>
- Netskope: <https://www.netskope.com/de/blog/safely-enable-chatgpt-generative-ai-in-one-move>, <https://community.netskope.com/generative-ai-60/how-to-craft-effective-generative-ai-policies-a-walkthrough-7107>, <https://www.techrepublic.com/article/netskope-chatgpt-app/>
- Microsoft Purview: <https://learn.microsoft.com/purview/ai-other-apps>, <https://www.proarch.com/blog/microsoft-purview-chatgpt-ai-data-protection>
- Cloudflare: <https://developers.cloudflare.com/waf/detections/firewall-for-ai/>, <https://developers.cloudflare.com/changelog/post/2025-08-25-ai-prompt-protection/>, <https://blog.cloudflare.com/ai-prompt-protection>
- LiteLLM: <https://docs.litellm.ai/docs/proxy/guardrails/pii_masking_v2>
- Portkey: <https://portkey.ai/docs/product/guardrails/pii-redaction>, <https://www.requesty.ai/blog/llm-guardrails-pii-redaction-gateways>
- Kong: <https://developer.konghq.com/plugins/ai-sanitizer/>, <https://developer.konghq.com/cookbooks/multi-layer-ai-guardrails/>, <https://developer.konghq.com/ai-gateway/policies/ai-semantic-prompt-guard/>
- Specialisterna: <https://www.lakera.ai/risk/ai-data-leakage>, <https://www.superblocks.com/blog/generative-ai-security-tools>, <https://nexos.ai/blog/llm-security-tools/>
- Europeiska plattformar: <https://intric.ai/en/governance>, <https://europeanstack.pages.dev/software/langdock>, <https://www.reply.com/en/artificial-intelligence/sovereign-ai-for-european-organisations-foundation-models-infrastructure-and-compliance>
