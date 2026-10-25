# ADR-0015: Sluss Egress — AI-API-proxy som MDM pekar ut, opt-in TLS-terminering

## Status

Föreslagen (2026-10-25). Underlag för ISSUE-132. Ägaren beslutar innan bygge.

## Kontext

Kunden vill att *all* AI-trafik från hanterade enheter passerar deras Sluss,
inte bara det som någon har kommit ihåg att konfigurera. Läget efter
ISSUE-128a/131:

| Trafik | Idag |
|---|---|
| Verktyg med base-URL (SDK:er, Cursor, Claude Code, agenter) | MDM sätter `OPENAI_BASE_URL`/`ANTHROPIC_BASE_URL` → Sluss ✅ |
| Webbchattar (chatgpt.com, claude.ai) | kundens filter blockerar → `/blocked` → Sluss-chatten ✅ |
| Appar som anropar `api.openai.com`/`api.anthropic.com` men **inte** går att konfigurera | passerar förbi, eller blockeras helt av filtret ❌ |

Den sista raden är luckan: en desktop-app, ett plugin eller ett skript med
hårdkodad endpoint och egen (ofta privat) API-nyckel. Blockera den och
användaren tappar verktyget, släpp den och policyn gäller inte.

Två fakta styr designen:

1. **Webbchattarna kan aldrig routas genom Sluss.** chatgpt.com talar
   ChatGPT:s interna webbprotokoll, inte API:t. Även dekrypterad kan trafiken
   inte skickas till en annan modell. För dem är block + erbjud rätt svar.
2. **API-trafiken kan.** Den talar exakt de format Sluss redan förstår
   (`/v1/chat/completions`, `/v1/messages`). Att ta emot den på nätverket i
   stället för via base-URL är en transportfråga, inte en ny produkt.

## Beslut

### 1. Sluss blir en uttalad forward-proxy för AI-API-värdar — inget annat

En ny lyssnare (`ROUTER_EGRESS_ADDR`, t.ex. `:3128`) tar emot HTTP `CONNECT`.
MDM:en rullar ut en **PAC-fil** (genereras på Rollout-sidan) som skickar
*endast* AI-leverantörernas API-värdar till Sluss och allt annat `DIRECT`.

- Tillåtna värdar är en kort, kod-ägd lista (`api.openai.com`,
  `api.anthropic.com`, `generativelanguage.googleapis.com`, `api.mistral.ai`,
  …) plus `ROUTER_EGRESS_EXTRA_HOSTS`.
- `CONNECT` till **alla andra värdar avvisas** (403 + audit). Sluss blir
  aldrig en allmän webbproxy, och webbchattar går aldrig in i Sluss.

### 2. TLS termineras med kundens egen CA — avstängt tills kunden slår på det

För tillåtna värdar terminerar Sluss TLS med leaf-certifikat som utfärdas
i minnet av en **mellan-CA som kunden lämnar** (`ROUTER_EGRESS_CA_CERT`,
`ROUTER_EGRESS_CA_KEY`), och som MDM:en redan har gjort betrodd på
enheterna (de flesta Zscaler/Defender-kunder har en sådan).

- Utan CA-variabler startar inte egress, och det finns inget inbyggt
  fallback-cert. Det är fail-closed.
- Leaf-cert: kort livstid (24 h), cachade per värd, nyckeln lämnar aldrig
  processen.
- Uppstart och varje konfigurationsändring auditeras (`egress.enabled`).

### 3. Dekrypterade anrop går genom exakt samma pipeline som `/v1`

- `POST /v1/chat/completions` och `POST /v1/messages` mappas till befintliga
  hanterare: klassning, policy, motor, fallback, audit, streaming oförändrat.
- **Klientens egen API-nyckel vidarebefordras aldrig.** Sluss använder sina
  egna providernycklar. Nyckeln loggas som ett kort fingeravtryck (hash-prefix)
  så att "privat nyckel använd på företagsenhet" blir ett bevis, utan att
  nyckeln sparas.
- Övriga sökvägar på samma värdar (filer, fine-tuning, assistants, okänt)
  → **403 fail-closed** med JSON-fel i leverantörens format + audit.
  Allow-listan utökas medvetet, aldrig implicit.

### 4. Attribution: nätverk först, enhet sedan

PAC kan inte bära nycklar. v1 attribuerar med **källnät**
(`ROUTER_EGRESS_NETWORKS=10.1.0.0/16=finance,…`) och annars tenant `egress`.
Fas 3 lägger till **enhetscertifikat** (mTLS mot proxyn med MDM-utfärdat
SCEP-cert). Det är samma identitet som ISSUE-128b skisserade, utan egen agent.

### 5. Det som uttryckligen inte görs

- Ingen interception av webbchattar eller annan trafik än listade API-värdar.
- Ingen transparent interception (iptables/route-kapning). Kunden pekar
  aktivt ut proxyn.
- Ingen lagring av promptinnehåll utöver det `ROUTER_PROMPT_LOGGING` redan
  styr.

## Konsekvenser

**Positiva**
- "Allt AI-API från hanterade enheter passerar Sluss" blir sant, inte bara
  "allt som är konfigurerat". Det lyfter Sluss från gateway till AI-brandvägg
  i samma kategori som Netskope/Zscaler AI, fast self-hosted och med
  hash-kedjad audit.
- Shadow-nycklar blir synliga: privata API-nycklar på företagsenheter syns
  som fingeravtryck i loggen och på dashboarden.
- Ingen ny klient på enheten. PAC och CA rullas ut med MDM:en som redan finns.

**Negativa / risker**
- **Det är TLS-inspektion.** Kunden behöver DPIA och, i Sverige, MBL §11-
  förhandling (Betriebsrat i Tyskland). Dokumentationen säger det rakt ut.
  Positioneringen är "kontroll + bevis", aldrig "övervakning".
- **Cert-pinning:** appar som pinnar leverantörens cert slutar fungera. Det
  blir i praktiken blockering, vilket är samma läge som idag med filtret. Det
  dokumenteras och syns som TLS-fel i egress-loggen.
- **Ny attackyta:** en proxy-port och en CA-nyckel i processen. CA:n ska vara
  en dedikerad mellan-CA med namnbegränsning (`nameConstraints` på de
  listade värdarna), så att en läckt nyckel inte kan utfärda cert för annat.
- **Latens:** en extra TLS-handskakning per anslutning. Routingbudgeten
  (p95 < 100 ms före provideranrop) mäts separat för egress.
- Webbläsarbaserade API-anrop (CORS från webbsidor) omfattas om webbläsaren
  följer PAC. Det dokumenteras men byggs inte särskilt.

## Alternativ

| Alternativ | Varför inte (nu) |
|---|---|
| Bara base-URL via MDM (dagens läge) | Täcker inte okonfigurerbara appar, som är luckan |
| Bara blockera API-värdarna i filtret | Tar bort verktyget i stället för att styra det och ger noll bevis om användningen |
| DNS-omskrivning av `api.openai.com` → Sluss-IP (samma cert-krav, ingen PAC) | Fångar även appar som ignorerar proxyinställningar, men kräver att Sluss nås på 443 under leverantörens namn för *alla* klienter i nätet. Kvar som fas 4-tillval för kunder som hellre styr DNS än PAC |
| Transparent interception / Sluss Bridge med WireGuard (ISSUE-128b) | Kräver nätverksposition eller egen agent. Analysen 2026-10-13 visade att MDM + PAC ger samma effekt utan en binär på desktop |
| Full secure web gateway | Inte vår produkt. Kunden har redan en, och Sluss ska passa in, inte ersätta |
| Cloudflare Gateway / Zscaler som proxy och Sluss bakom | Möjligt för kunder som redan har det, men de terminerar TLS och skickar vidare. Det ger två inspektionspunkter och Sluss ser inte enheten. Dokumenteras som integrationsväg, inte som grundläget |

## Faser (förslag till ISSUE-132)

1. **E1 — proxy utan dekryptering:** `CONNECT`-lyssnare, värdlista, avvisa
   allt annat, audit per anslutning (värd, källnät, utfall). PAC-generator på
   Rollout. Bevis: "vilka enheter når AI-API:er, och hur ofta". Ingen CA krävs.
2. **E2 — TLS-terminering och API-mappning:** kundens CA, leaf-cert-cache,
   mappning till `/v1`-pipelinen, nyckelfingeravtryck, 403 på okända
   sökvägar, streaming.
3. **E3 — enhetsidentitet:** mTLS mot proxyn med MDM-cert, enhet + användare
   i audit-kedjan.
4. **E4 (tillval) — DNS-läge** för kunder utan PAC.

E1 ger värde direkt: det är ren observation utan inspektion, och förhandlingsbart
med facket på en dag.
