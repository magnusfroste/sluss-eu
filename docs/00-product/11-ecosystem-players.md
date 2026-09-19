# 11 — Ecosystem & players: AI gateways, GenAI security, and the shadow-AI problem

> Commissioned 2026-07-14 after an advisor meeting (a CISO with a long
> regulated-finance background) that validated the per-prompt hybrid model and
> raised the bypass/shadow-AI objection. Purpose: substrate for the investor
> pitch and first-customer conversations. Research performed 2026-07-17 by a
> web-research agent; claims multi-sourced unless tagged `[single source]` or
> `[rumor]`. Companion docs: `08-positioning-ciso.md`, `10-market-nis2-eu-global.md`,
> DECISION_LOG 2026-07-14.

---

## 1. The emerging ecosystem map (as of mid-2026)

### 1a. LLM gateways / routers

| Player | Positioning | Deployment | Pricing | Funding / traction |
|---|---|---|---|---|
| **LiteLLM (BerriAI, YC W23)** | De-facto default OSS gateway: 100+ providers behind an OpenAI-format proxy with cost tracking, guardrails, load balancing ([GitHub](https://github.com/BerriAI/litellm)) | OSS self-hosted + enterprise tier (SSO, RBAC, audit logs) | OSS free; Enterprise ~$250/mo basic to ~$30k/yr premium ([TrueFoundry analysis](https://www.truefoundry.com/blog/litellm-pricing-guide)) | ~53.8k GitHub stars; "1B+ requests served, Fortune 500 + federal agencies" ([litellm.ai](https://www.litellm.ai/ai-gateway)); only ~$2.1M confirmed funding per [Tracxn](https://tracxn.com/d/companies/litellm/__mfXZyWlSZ7SOXPT24aIT1nlmqGNQ68euBk2-ovrz95Y) — adoption far ahead of monetization |
| **Portkey** | "Unified control plane for production AI": gateway + observability + guardrails + governance | SaaS + OSS gateway (12.5k stars) + hybrid/on-prem | Free tier; enterprise governance ~$2–5k/mo | **$15M Series A, Feb 2026** (Elevation, Lightspeed); claims 500B tokens/125M requests/day across 24k orgs, $180M annualized LLM spend managed ([Portkey blog](https://portkey.ai/blog/series-a-funding/), [GlobeNewswire](https://www.globenewswire.com/news-release/2026/02/19/3241385/0/en/Portkey-Raises-15M-Series-A-to-Scale-the-Unified-Control-Plane-for-Production-AI.html)) |
| **OpenRouter** | Developer **marketplace** for 400+ models — aggregation + billing, not enterprise governance | SaaS only | Pass-through + ~5% fee | $40M seed+A (a16z, Menlo) June 2025; **$113M Series B at $1.3B, May 2026** (CapitalG) ([TechCrunch](https://techcrunch.com/2026/05/26/openrouter-more-than-doubles-valuation-to-1-3b-in-a-year/)); >$100M annualized inference spend by May 2025 ([Sacra](https://sacra.com/c/openrouter/)) |
| **Kong AI Gateway** | AI plugin layer on the incumbent API gateway (token rate limiting, PII redaction, semantic routing) ([Kong](https://konghq.com/products/kong-ai-gateway)) | OSS self-host + Konnect SaaS + Enterprise | Konnect tiers ~$500–2,500/mo | Product line of late-stage Kong Inc.; strongest "already-run-Kong" play |
| **Cloudflare AI Gateway** | Free edge control plane: caching, analytics, unified billing, **spend limits (June 2026)** ([changelog](https://developers.cloudflare.com/changelog/post/2026-06-05-spend-limits/)) | SaaS only, no self-host | Core free; 5% fee on unified billing ([pricing](https://developers.cloudflare.com/ai-gateway/reference/pricing/)) | Public-company product; commoditizes the thin-proxy layer |
| **Martian** | Interpretability-based LLM routing (cost −20–97%) | SaaS | Custom | $9M (NEA, Nov 2023) + Accenture Ventures 2024 ([TechCrunch](https://techcrunch.com/2023/11/15/martians-tool-automatically-switches-between-llms-to-reduce-costs/)); low 2026 commercial visibility — pivoted toward interpretability research; a claimed $1.3B valuation is `[unreliable single source]`, likely confusion with OpenRouter |
| **TrueFoundry** | Kubernetes-native AI platform + gateway **in the customer's own VPC/on-prem** — strongest on-prem posture among funded vendors | Customer VPC/on-prem + SaaS | Custom | **$19M Series A, Feb 2025** (Intel Capital, Peak XV) ([Businesswire](https://www.businesswire.com/news/home/20250206649881/en/)) |
| **Helicone (YC W23)** | OSS observability + Rust AI gateway | SaaS + full OSS self-host | Free/usage-based | ~5.8k stars; no priced round found beyond YC |
| **Others** | **Vercel AI Gateway** (SaaS, zero markup, GA Aug 2025); **Bifrost/Maxim** (Go OSS, "50x faster than LiteLLM", 6.6k stars); **Arch/Plano (Katanemo)** (Envoy-based, routes via its own 1.5B model — [Arch-Router paper](https://arxiv.org/html/2506.16655v1)); **Apache APISIX AI Gateway** (OSS, Apr 2025); **Solo.io agentgateway** (now Linux Foundation); **Requesty** (London, $3M seed Sept 2025, markets GDPR/AI-Act-ready routing); **Unify** ($8M 2024 → pivoted away from routing — cautionary tale) | | | |

**European gateways:** **nexos.ai** (Vilnius, Nord Security founders) is the flagship: AI Workspace + AI Gateway, ~200 models, explicitly pitched at the shadow-AI problem — **€30M Series A at €300M valuation, Oct 2025** (Index, Evantic) ([TechCrunch](https://techcrunch.com/2025/10/20/european-ai-rising-star-nexos-ai-raises-30m-to-unlock-enterprise-ai-adoption/), [EU-Startups](https://www.eu-startups.com/2025/10/with-e30-million-in-funding-vilnius-based-nexos-ai-sets-its-sights-on-ai-adoption-and-the-shadow-ai-challenge/)). Note: **SaaS**, not self-hosted.

### 1b. GenAI security / AI firewalls — a consolidation wave, not a market

Seven of the category's named startups were absorbed by platform incumbents in ~13 months (Aug 2024–Oct 2025):

| Target → Acquirer | Price | Date |
|---|---|---|
| Robust Intelligence → **Cisco** (now "AI Defense") | ~$400M `[press figure]` ([SecurityWeek](https://www.securityweek.com/cisco-to-acquire-ai-security-firm-robust-intelligence/)) | closed Oct 2024 |
| Protect AI → **Palo Alto Networks** (Prisma AIRS) | $700M announced / $634.5M final ([PANW](https://www.paloaltonetworks.com/company/press/2025/palo-alto-networks-completes-acquisition-of-protect-ai)) | closed Jul 2025 |
| Apex Security → **Tenable** | ~$105M `[press]` ([Tenable](https://www.tenable.com/press-releases/tenable-announces-intent-to-acquire-apex-security-to-expand-exposure-management-across-the-ai-attack-surface)) | Jun 2025 |
| Prompt Security → **SentinelOne** | ~$250M ([Yahoo](https://finance.yahoo.com/news/sentinelone-acquire-prompt-security-250m-182354008.html)) | closed Sep 2025 |
| Aim Security → **Cato Networks** (now "Cato AISEC") | undisclosed, ~$350M `[rumor]` ([CyberScoop](https://cyberscoop.com/cato-networks-acquires-ai-security-startup-aim-security/)) | Sep 2025 |
| Lakera → **Check Point** | ~$201.8M per [20-F](https://www.sec.gov/Archives/edgar/data/0001015922/000117891326001932/zk2634942.htm) (~$300M per [Calcalist](https://www.calcalistech.com/ctechnews/article/rj5bc1vige)) | closed Oct 2025 |
| CalypsoAI → **F5** ("AI Guardrails" + "AI Red Team") | $180M ([GeekWire](https://www.geekwire.com/2025/f5-paying-180m-to-acquire-calypsoai-to-boost-ai-enterprise-security-offerings/)) | Sep 2025 |

**Still independent:** **WitnessAI** ($58M Jan 2026, Sound Ventures; claims 500% ARR growth — [PR](https://www.prnewswire.com/news-releases/witnessai-raises-58-million-for-global-expansion-and-announces-new-ways-to-secure-ai-agents-302659319.html)); **HiddenLayer** ($50M Series A 2023, no B found); **Noma Security** ($100M Series B Jul 2025); **Zenity** (agent governance, $38M B); **Cyberhaven** (shadow-AI data lineage, **$100M Series D at $1.0B, Apr 2025** — [PR](https://www.prnewswire.com/news-releases/cyberhaven-raises-100-million-series-d-at-1-billion-valuation-302418497.html)); plus early-stage Pillar, Harmonic, Knostic, Lasso, Nightfall, Arthur; governance-layer Credo AI and Holistic AI (explicit EU AI Act packs).

**Two structural facts:** (1) the inline "AI firewall" proved to be an absorbable *feature*, priced at $100–700M capability tuck-ins, not revenue multiples (Robust Intelligence had ~$10M revenue at a $400M exit — [Calcalist](https://www.calcalistech.com/ctechnews/article/rjgsb5npa)); (2) inline detection is done with **purpose-built small ML classifiers, not frontier-LLM judges**, because of latency budgets — the "no LLM in the hot path" principle is already industry practice at the detection layer, but nobody markets *deterministic, explainable* classification as the product.

### 1c. Incumbent security vendors — the shadow-AI-discovery land grab

Every major SSE/SASE/firewall vendor now ships (a) shadow-AI discovery, (b) prompt-level DLP, (c) block/coach policies, (d) increasingly MCP/agent visibility:

- **Zscaler**: AI Access Security; "AI Protect" suite launched Jan 2026; June 2026 added MCP gateway + endpoint AI visibility ([press](https://www.zscaler.com/press/zscaler-unveils-new-innovations-secure-enterprise-ai-adoption)).
- **Netskope**: SkopeAI / One AI Security — 370+ GenAI apps classified, instance-aware (corporate vs personal account), real-time user coaching with redirect-to-sanctioned-app ([product](https://www.netskope.com/solutions/netskope-one-ai-security), [coaching docs](https://docs.netskope.com/en/real-time-user-coaching.html)). Its telemetry is the market's evidence base (below).
- **Palo Alto Networks**: AI Access Security (employee usage) + Prisma AIRS 1.0→3.0 (Apr 2025 → Mar 2026), Protect AI folded in; $25B CyberArk deal for agent identity ([press](https://www.paloaltonetworks.com/company/press/2026/palo-alto-networks-secures-agentic-ai-with-prisma-airs-3-0)).
- **Microsoft**: Purview **DSPM for AI** (prompts monitored across Copilot, Azure AI, third-party apps), Defender for Cloud Apps shadow-AI discovery, Edge for Business inline prompt DLP + Purview Chrome extension ([Learn](https://learn.microsoft.com/en-us/purview/dspm-for-ai)). Mostly E5-gated.
- **Cato Networks**: GenAI CASB controls (Apr 2025) → Aim acquisition → **Cato AISEC** in SASE Cloud from early 2026 ([Cato](https://www.catonetworks.com/platform/ai-security-aisec/)).
- **Cisco** (AI Defense, Jan 2025), **Check Point** (GenAI Protect + Lakera), **Fortinet** (FortiOS 8.0 GenAI app control + FortiAnalyzer 8.0 **Shadow-AI Report**, Mar 2026 — [press](https://www.fortinet.com/corporate/about-us/newsroom/press-releases/2026/fortinet-introduces-fortios-8-expand-secure-networking-with-secure-ai-controls-fabric-based-ai-agents-flexible-sase-and-simplified-sdwan)), **Cloudflare** (Firewall for AI, open beta 2025).

**The prevalence numbers this land grab is chasing** (also Tokenizer's pitch ammunition):

- **72% of enterprise genAI use was shadow IT** (personal accounts) in early 2025; by the Jan 2026 report still **47% of genAI users** use personal, unmanaged accounts ([Netskope 2025](https://www.netskope.com/resources/cloud-and-threat-reports/cloud-and-threat-report-generative-ai-2025), [2026](https://www.netskope.com/resources/cloud-and-threat-reports/cloud-and-threat-report-2026)).
- GenAI **data-policy violations more than doubled in 2025**; average org 223 incidents/month ([Netskope press](https://www.netskope.com/press-releases/netskope-threat-labs-shadow-ai-risks-proliferate-as-genai-platforms-and-ai-agents-see-rapid-adoption)).
- **39.7% of data movements into AI tools involve sensitive data**; personal-account share: ChatGPT 32%, **Claude 58%, Perplexity 61%** ([Cyberhaven 2026](https://www.cyberhaven.com/blog/sensitive-data-flowing-into-ai-tools)).
- **IBM Cost of a Data Breach 2025**: 20% of orgs had a shadow-AI-linked breach, average cost **$4.63M (+$670k premium)**; 97% of AI-related incidents lacked AI access controls; 63% had no AI governance policy ([IBM](https://www.ibm.com/reports/data-breach), [newsroom](https://newsroom.ibm.com/2025-07-30-ibm-report-13-of-organizations-reported-breaches-of-ai-models-or-applications,-97-of-which-reported-lacking-proper-ai-access-controls)).

### 1d. EU / sovereignty players

- **Mistral AI**: Europe's AI champion — **€1.7B Series C at €11.7B post, Sept 2025, ASML lead (€1.3B, 11%)** ([CNBC](https://www.cnbc.com/2025/09/09/ai-firm-mistral-valued-at-14-billion-as-asml-takes-major-stake.html), [Mistral](https://mistral.ai/news/mistral-ai-raises-1-7-b-to-accelerate-technological-progress-with-ai/)). Sells deployable models — not a gateway/governance layer.
- **Aleph Alpha → Cohere merger** (announced Apr 2026, ~$20B combined valuation per Handelsblatt `[single source]`): a government-blessed "sovereign AI" champion expected to run on **STACKIT** (Schwarz Digits' sovereign cloud) ([TechCrunch](https://techcrunch.com/2026/04/25/why-cohere-is-merging-with-aleph-alpha/)). Aleph Alpha itself had generated little revenue.
- **EU-hosted inference**: **OVHcloud AI Endpoints** (40+ open-weight models, France-hosted, GDPR-positioned — [OVHcloud](https://www.ovhcloud.com/en/public-cloud/ai-endpoints/)); **IONOS AI Model Hub** (Germany, ISO 27001, explicit EU-AI-Act docs — [IONOS](https://docs.ionos.com/cloud/ai/ai-model-hub)); Scaleway Generative APIs; T-Systems/Open Telekom Cloud. These are *endpoints Tokenizer can route to*, not competitors.
- **European governance/gateway startups**: nexos.ai (see 1a — closest EU competitor, but SaaS); Langdock (Germany, ChatGPT-for-companies); niche EU-AI-Act audit tooling (e.g. hash-chained audit logs: [systima aiact-audit-log](https://github.com/systima-ai/aiact-audit-log)) — micro-entrants, no funded player.
- **Regulatory clock** (verified dates):
  - **EU AI Act**: in force Aug 2024; GPAI obligations Aug 2025; **Digital Omnibus agreed May–Jun 2026** — high-risk (Annex III) obligations deferred to **2 Dec 2027**, embedded-product high-risk to Aug 2028; transparency/watermarking from Dec 2026 ([Consilium](https://www.consilium.europa.eu/en/press/press-releases/2026/05/07/artificial-intelligence-council-and-parliament-agree-to-simplify-and-streamline-rules/), [Gibson Dunn](https://www.gibsondunn.com/eu-ai-act-omnibus-agreement-postponed-high-risk-deadlines-and-other-key-changes/)). Interpretation: enforcement pressure eased ~16 months, but obligations (incl. Article 12 record-keeping) are now date-certain.
  - **NIS2**: Sweden's transposition, **Cybersäkerhetslagen (SFS 2025:1506), in force 15 Jan 2026**, registration with MCF from Feb 2026 ([Advisense](https://advisense.com/2026/01/19/the-swedish-nis2-implementation-cybersakerhetslagen/)) — NIS2 is *now live* in Tokenizer's home market.
  - **DORA**: applies since **17 Jan 2025** to financial entities + critical ICT providers; in Sweden DORA prevails over NIS2 where they overlap ([RISMA](https://www.rismasystems.com/en/resources/articles/how-nis2-dora-gdpr-are-implemented-in-scandinavia)).

---

## 2. Where Tokenizer fits — positioning gap analysis

**The gap, property by property:**

| Property | State of the market |
|---|---|
| Self-hosted/on-prem gateway | Covered: LiteLLM, TrueFoundry, Kong, Portkey hybrid. Table stakes, not a moat alone. |
| **Deterministic, no-LLM/no-ML content classification for routing** | **Essentially nobody.** Kong routes via embedding similarity (ML); Arch and Martian route via small *models*; LiteLLM/Portkey/Bifrost route deterministically only on *metadata* (cost, latency, weights) — not prompt content. Content-aware + deterministic + explainable + <100ms is open. |
| **Fail-closed egress policy (cloud vs on-prem per prompt class)** | Incumbent SSE does block/allow *per app*; gateways do routing *per model*. **Nobody does policy-controlled egress per prompt classification** ("payment data never leaves; trivial tasks go to cheap EU-hosted models"). This cloud/on-prem hybrid per request is exactly what the advisor validated. |
| **Tamper-evident audit** | Gateways offer audit logs; **none advertise cryptographic tamper-evidence**. Only micro-entrants (systima, Certainity, Senthex) target AI-Act Article 12 logging. An open [LiteLLM feature request for EU AI Act compliance](https://github.com/BerriAI/litellm/issues/24836) signals unmet demand. |
| **EU regime packs (NIS2/DORA/GDPR/AI-Act mappings)** | Governance platforms (Credo, Holistic) do frameworks but sit outside the request path. No gateway ships regime packs. Requesty markets "EU-compliant routing" but is SaaS and seed-stage. |
| **Cost/CO2-aware routing** | Cost: broadly covered. **CO2: no commercial gateway ships it** — academic only ([GAR, arXiv](https://arxiv.org/abs/2605.11603)). Differentiating garnish for EU ESG buyers, not the wedge. |

**Who is closest, and what they'd have to do to copy:**

1. **nexos.ai** — same problem statement (safe AI adoption, shadow AI), EU, funded (€30M). To copy: ship a genuinely self-hosted product (its value prop is a managed workspace — self-hosting inverts its ops model) and rebuild classification as deterministic/explainable. Closest competitor; also the best evidence the thesis fundraises.
2. **LiteLLM** — owns the self-host default. To copy: build a content-classification engine, policy compiler, and tamper-evident log — none trivial, and its enterprise focus is spend/access management, not compliance. More likely: Tokenizer competes *against* "we'll just run LiteLLM" in every deal — the answer is regime packs, fail-closed egress semantics, and audit evidence, not proxying.
3. **TrueFoundry / Portkey** — enterprise VPC deployment exists. To copy: EU regime packs and deterministic classification; both are US/India-centric and sell to platform teams, not risk/compliance officers.
4. **Netskope/Zscaler/Cato** — own enforcement, but their unit of control is *the app*, not *the prompt-to-model route*, and they can't host the gateway inside the customer's boundary. They are complements (steering traffic *to* Tokenizer) and acquirers more than competitors.
5. **Check Point/PANW/Cisco post-acquisition stacks** — have prompt inspection, but ML-classifier-based, SaaS-attached, and sold as platform add-ons; deterministic auditability ("show the exact rule that fired, reproducibly, to a regulator") is architecturally against their grain.

**Tokenizer's defensible combination:** *self-hosted + deterministic explainable classification + fail-closed per-class egress + tamper-evident audit + EU regime packs*. Each element exists somewhere; the combination exists nowhere, and the combination is precisely what a NIS2/DORA-regulated EU buyer can defend to an auditor.

---

## 3. The shadow-AI / bypass problem (the advisor's objection)

**The honest baseline:** ~50% of employees use unapproved AI; **46% say they'd continue even if banned** ([Software AG, Oct 2024, n=6,000](https://newscenter.softwareag.com/en/news-stories/press-releases/2024/1022-half-of-all-employees-use-shadow-ai.html)); **78% of AI users bring their own AI to work — 80% at SMBs** ([Microsoft/LinkedIn Work Trend Index 2024, n=31,000](https://news.microsoft.com/source/2024/05/08/microsoft-and-linkedin-release-the-2024-work-trend-index-on-the-state-of-ai-at-work/)). No mechanism seals this; the realistic goal is moving the *bulk* of usage — especially sensitive usage — through the sanctioned path.

**(a) Network/DNS/SSE enforcement** — works where it exists, with holes: SSE vendors classify hundreds of GenAI apps and block/coach inline. But TLS inspection breaks on certificate-pinned apps ([Cato best practices](https://support.catonetworks.com/hc/en-us/articles/360007713437-Best-Practices-for-TLS-Inspection)); BYOD devices won't trust the corporate CA; personal phones/hotspots bypass everything. Without MDM, mid-market realistically gets **DNS-level filtering only** (e.g. Cloudflare Gateway AI category — [docs](https://developers.cloudflare.com/cloudflare-one/traffic-policies/dns-policies/common-policies/)): good for discovery and steering, not sealing.

**(b) SSO/identity** — the strongest cheap control: bind ChatGPT Enterprise/Claude/Copilot to the IdP (SAML+SCIM), apply conditional access, offboard centrally. The hole: personal logins ("Continue with Google" on chatgpt.com) never touch the corporate IdP ([Microsoft Q&A](https://learn.microsoft.com/en-us/answers/questions/5793514/how-to-prevent-users-from-logging-into-personal-ch)). Blocking personal accounts requires an inline inspection point injecting workspace-ID headers ([dope.security](https://dope.security/post/chatgpt-workspace-id-what-it-is-and-how-to-use-it-for-security)) — i.e., identity alone can't do it without *some* proxy.

**(c) The carrot — where the evidence actually is.** This is the direct answer to the Dropbox objection, and the Dropbox story itself resolves it: IT never beat Dropbox by blocking; it won by *sanctioning an equivalent* (Dropbox for Business, 2011 — Dropbox's enterprise pitch was literally "your users already run our product, now pay to control it" — [InfoWorld](https://www.infoworld.com/article/2238712/dropbox-wants-shadow-it-to-drive-enterprise-adoption-3.html)). The GenAI replays:

- **Samsung**: banned GenAI May 2023 after three source-code leaks in 20 days ([TechCrunch](https://techcrunch.com/2023/05/02/samsung-bans-use-of-generative-ai-tools-like-chatgpt-after-april-internal-data-leak/)) → built internal **Samsung Gauss** by Nov 2023 ([TechCrunch](https://techcrunch.com/2023/11/08/samsung-unveils-chatgpt-alternative-samsung-gauss-that-can-generate-text-code-and-images/)).
- **JPMorgan**: restricted ChatGPT Feb 2023 → **LLM Suite** ("ChatGPT in a JPMorgan-approved wrapper") to 200k+ employees from Aug 2024, ~250k by late 2025 ([CNBC 2024](https://www.cnbc.com/2024/08/09/jpmorgan-chase-ai-artificial-intelligence-assistant-chatgpt-openai.html), [CNBC 2025](https://www.cnbc.com/2025/09/30/jpmorgan-chase-fully-ai-connected-megabank.html)). Goldman, BofA, Citi, Deutsche Bank followed the same ban→license arc.
- **Trend evidence carrot works**: personal-account share of workplace ChatGPT fell from **~74% (2024)** to **~32% (2026)** as orgs provisioned sanctioned accounts ([Cyberhaven 2024](https://www.prnewswire.com/news-releases/cyberhaven-report-surge-in-shadow-ai-accounts-poses-fresh-risks-to-corporate-data-302151221.html) → [2026](https://www.cyberhaven.com/resources/report/ai-adoption-risk-report-2026)) — while unsanctioned tools (Claude 58%, Perplexity 61% personal) stayed shadow. Sanctioning demonstrably converts usage; not sanctioning demonstrably doesn't.
- Even **Gartner's 2026 predictions tell CIOs to improve enterprise AI tool UX specifically to reduce shadow AI** ([Gartner, May 2026](https://www.gartner.com/en/newsroom/press-releases/2026-05-13-gartner-predicts-by-2027-50-percent-of-enterprises-without-a-people-centric-ai-strategy-will-lose-their-top-ai-talent)), and Slack/Salesforce research shows employees' favorite AI tools "lack business context" ([Salesforce](https://www.salesforce.com/news/stories/ai-tools-lack-job-context/)) — the sanctioned tool can win on *quality*, not just policy: company context, premium models paid for, no data anxiety.

**(d) Browser extensions / enterprise browsers** — the no-MDM enforcement layer: Island and Palo Alto Prisma Browser enforce GenAI DLP **on unmanaged/BYOD devices without MDM** ([Palo Alto](https://www.paloaltonetworks.com/sase/prisma-browser)); extension platforms (LayerX, Harmonic — which *nudges and redirects to the sanctioned alternative* — [Harmonic](https://www.harmonic.security/solutions/browser-based-genai-security)) do the same lighter-weight. Caveat: extension supply-chain risk (Cyberhaven's own extension was compromised Dec 2024 — [LayerX write-up](https://layerxsecurity.com/blog/attack-campaign-targeting-browser-extensions/)).

**(e) Discovery/monitor-first** — the canonical maturity ladder, per Netskope's productized pattern and Gartner guidance: **discover → coach/redirect → DLP on sanctioned app → hard-block only highest-risk** ([Netskope coaching](https://docs.netskope.com/en/real-time-user-coaching.html), [Gartner: monitor and dialogue rather than deny](https://mytechdecisions.com/compliance/chatgpt-generative-ai-policies/)). Gartner's **AI TRiSM** frame: orgs operationalizing AI trust/transparency/security see ~50% better AI adoption/business outcomes by 2026 `[secondary sources — paywalled original]` ([Gartner TRiSM](https://www.gartner.com/en/articles/ai-governance-trism)); 40% of AI data breaches from cross-border GenAI misuse by 2027 ([Gartner](https://www.gartner.com/en/newsroom/press-releases/2025-02-17-gartner-predicts-forty-percent-of-ai-data-breaches-will-arise-from-cross-border-genai-misuse-by-2027)).

**Answer to the advisor, in one paragraph:** Bypass is real and permanent at the margin — but the Dropbox lesson cuts *for* Tokenizer, not against it: convenience won, so the winning strategy is to *be the convenience*. Tokenizer's enforcement story for the mid-market is carrot-first (company-paid premium models + company context, through the gateway, smoother than a personal account), identity-second (SSO-bound access, sanctioned keys), cheap-network-third (DNS steer + discovery), with tamper-evident audit turning "most usage goes through us" into a defensible compliance narrative — which is what NIS2/DORA actually requires (demonstrable risk management, not perfection).

---

## 4. Segment analysis: Large → Mid → Small

**Governance maturity (EU, survey-grade):** AI use in 2025: **55% of large enterprises vs 30% of medium vs 17% of small** ([Eurostat](https://ec.europa.eu/eurostat/statistics-explained/index.php?title=Use_of_artificial_intelligence_in_enterprises)). Only **~36% of EU enterprises have documented ICT-security policies** (heavily size-skewed; Sweden highest at 66% in 2022 — [Eurostat](https://ec.europa.eu/eurostat/web/products-eurostat-news/-/ddn-20221208-2)). Market-research figures suggest ~65% of SMBs have no formal device management `[single source, low confidence]`; no reliable SSE-by-size survey exists — but directionally: SSE/proxy is large-enterprise plumbing.

| Segment | Reality | Enforcement story that works | Fit for Tokenizer |
|---|---|---|---|
| **Large (2,500+, esp. regulated)** | Has SSE, MDM, CISO, DORA/NIS2 programs; already buying Netskope/Zscaler AI controls and building JPMorgan-style wrappers | Full stack: SSE steering + SSO + gateway | Long sales cycles; incumbents entrenched; but **regulated Nordic mid-caps (banks, insurers, energy under DORA/NIS2)** are reachable via design partners — the advisor's own network |
| **Mid-market (100–2,500)** | The structural gap: NIS2 *applies* (essential/important entities down to 50 employees/€10M), but no SSE, partial MDM, 1–5 person IT/security. 80% BYOAI. Can't buy E5+Purview complexity; won't run raw LiteLLM | **Carrot + identity + DNS**: company-paid premium AI through the gateway (`/connect` onboarding), SSO-bound, DNS discovery/steer, monitor→coach ladder; tamper-evident audit as the NIS2/DORA evidence artifact | **Primary target.** The segment where "self-hosted but easy" is a category-defining promise — nobody serves compliance-grade AI governance at mid-market ops capacity |
| **Small (<100)** | No governance capacity; shadow AI ~universal; unreachable economically for a self-hosted product | ChatGPT Team + prayer | Not a target (except via MSPs later) |

**Recommended beachhead:** NIS2/DORA-covered **Nordic/DACH mid-market and lower-large regulated organisations** (regional banks, insurers, fintechs, energy/utilities, healthcare, municipal-owned utilities) — they have (1) a *legal* forcing function now in force (Cybersäkerhetslagen since 15 Jan 2026, DORA since Jan 2025), (2) real sensitivity (the fail-closed on-prem story lands), (3) too little IT to run DIY LiteLLM+SSE stacks, and (4) budget authority concentrated in one CISO/CIO you can actually reach. The enforcement story sold to them: **"You cannot stop shadow AI with a firewall you don't have. You can make the sanctioned path the best tool your employees have ever used — and prove to your regulator exactly what left your boundary, and what never did."**

---

## 4b. Honest gaps — what Zscaler/SSE does that Tokenizer does not

Asked before closing scope (2026-07-17). Four gaps, one strategically important:

1. **They see ALL traffic; we only see traffic pointed at us.** An SSE is a
   forward proxy with an agent on every device: it can *discover* shadow AI in
   the wild (chatgpt.com, claude.ai, 800+ catalogued GenAI apps, corporate vs
   personal account). Tokenizer's gap report answers "what would policy have
   done with the sanctioned traffic?" — SSE answers "what are my employees
   using at all?". Different questions; the CISO asks both.
2. **Network enforcement.** Block/redirect unsanctioned AI apps, inline user
   coaching, browser isolation. Tokenizer has no stick outside its own
   endpoint — by design (carrot strategy), but say so honestly.
3. **Full DLP engine.** File uploads, OCR, exact-data-match, document
   fingerprinting across all apps. Tokenizer classifies prompt content
   rule-based; attachments are not deep-inspected (yet).
4. **Platform apparatus.** SOC2/ISO/FedRAMP, SIEM integrations, threat-intel
   feeds, per-app risk scores, global operations, MCP/agent-identity gateways
   (2026). Large-enterprise plumbing we neither have nor should build now.

**Why the scope still closes:** the inverse is also true — SSE cannot route a
prompt to the customer's own on-prem model (an SSE is itself a US cloud the
traffic transits, exactly what a sovereignty buyer distrusts), cannot pick a
model per prompt class, and offers no customer-owned tamper-evident audit.
**We are the destination; they are the road network.** Their coaching-redirect
can literally point at our `/connect`.

**The one gap that bites in our segment** (mid-market without SSE): no
discovery at all — they cannot see the problem we solve. Two cheap ways to
narrow it, parked in backlog:
- *DNS recipe as a runbook* (docs, not code): free DNS filter (e.g. Cloudflare
  Gateway free tier) + AI-domain list → discovery numbers + steering toward
  the gateway. We sell the recipe; the customer owns enforcement.
- *Future feature*: ingest the customer's DNS/proxy logs → a "shadow-AI
  discovery report" in the same evidence language as the gap report — without
  becoming an SSE.

**Pitch honesty rule:** never say "we stop shadow AI." Say: *"SSE tools see
the traffic; we make the sanctioned path the best one and prove what happened
there. If you run Zscaler, steer it here; if you don't, we are your cheapest
place to start."*

---

## 5. Implications for the investor pitch

1. **Timing — the money agrees the layer matters:** OpenRouter $1.3B (May 2026), Portkey $15M A (Feb 2026), nexos.ai €30M at €300M (Oct 2025), WitnessAI $58M (Jan 2026), Cyberhaven $1B — while incumbents paid **~$2B combined in 13 months** for seven GenAI-security startups. The gateway/governance layer is validated; the EU self-hosted flavor is unclaimed.
2. **Timing — regulation is now, not 2027:** DORA live since Jan 2025; Swedish NIS2 law in force **15 Jan 2026** with registration deadlines already passed; AI Act record-keeping/transparency lands Dec 2026–Dec 2027 (Omnibus dates now fixed). Tokenizer sells compliance *evidence* for laws in force today — the AI-Act delay is upside runway, not a demand-killer.
3. **The wedge:** deterministic prompt classification + fail-closed egress + tamper-evident audit, self-hosted, with EU regime packs — a combination no gateway (US or EU) ships; nexos.ai proves the demand and the fundability but is SaaS, which is precisely the property regulated EU buyers distrust.
4. **The moat (honest version):** not the proxy (commoditized by LiteLLM/Cloudflare) but (a) the compiled policy/regime-pack corpus mapped to NIS2/DORA/AI-Act articles, (b) audit-chain architecture regulators accept, (c) trust/distribution in the Nordic regulated mid-market — advisors with regulated-finance CISO backgrounds are the moat's seed.
5. **Shadow-AI objection, answered with data:** bans fail (46% would defy them — Software AG); carrots demonstrably convert (workplace ChatGPT personal-account share ~74%→~32% 2024→2026 as sanctioning spread; JPMorgan 250k users). Tokenizer's carrot: premium multi-model AI, company-paid, with company context — *through* the gateway because that's where the good tool lives. Even Gartner now instructs CIOs to fight shadow AI with better sanctioned UX.
6. **Exit landscape is proven:** seven comparable acquisitions at $100–700M by SASE/security platforms that all lack an EU-sovereign self-hosted story; European acquirers (Check Point EU ops, Cato, or an EU sovereignty buyer like Schwarz Digits) are plausible.
7. **Honest risk #1 — absorption:** the 2025 M&A wave shows inline AI controls become platform features. Mitigation: the self-hosted + regulatory-evidence combination is the part platforms structurally can't bundle (their control planes are SaaS), and mid-market is the segment they don't sell to.
8. **Honest risks #2–3:** LiteLLM-DIY is the real competitor in every technical evaluation (win on regime packs, fail-closed semantics, audit evidence — not on proxying); and enforcement is probabilistic — never pitch "we stop shadow AI," pitch "we make sanctioned AI win and give you the audit trail NIS2/DORA demand." Mid-market compliance budgets are small; pricing must land in the €1–4k/month band the segment can sign without procurement.

---

**Key uncertainties flagged:** Martian's status/valuation (unreliable); LiteLLM cumulative funding (conflicting $2.1M vs ~$15M); Aim Security price (`rumor`, ~$350M); Lakera price ($201.8M SEC vs $300M press); Gartner TRiSM "50%" wording (secondary sources only); SSE/MDM penetration by segment (no survey-grade data — directional only); Cohere–Aleph Alpha $20B valuation (single source, Handelsblatt).
