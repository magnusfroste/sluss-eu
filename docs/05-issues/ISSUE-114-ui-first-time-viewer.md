# ISSUE-114 — UI för förstagångsbetraktaren: evidens först, grupperad navigation, känslighet i loggen, agentregler i konsolen, provider-edit

- **Epic:** EPIC-15 (polish/brand)
- **Priority:** P1 (demo 2026-10-02)
- **State:** done (2026-09-29)

## Bakgrund

Demo med ny kontakt inom en vecka. Hälsokontrollen (ISSUE-113) var grön, så
nästa fråga var: förstår någon som ser konsolen för **första gången** vad
produkten gör? Genomgång med skärmdumpar av varje sida (landing, dashboard,
policy, modeller, providers, nycklar, logg, användare) som en cold viewer.
Fynd: dashboarden ledde med *besparing* (utvecklarens KPI) trots att köparen
är CISO/DPO (evidens); navigationen var en platt lista med elva poster;
loggen visade inte *varför* något blockerades eller vilken känslighetsklass
som avgjorde; konsolreglerna kunde inte uttrycka agent-kapabilitet
(ISSUE-111 fanns bara i YAML/packs); och en provider gick inte att ändra
utan att ta bort och lägga till igen (ägarönskemål: "jag vill kunna ändra
provider — edit").

## Ändring (allt användarriktat på engelska)

- **Dashboard** (`dashboard.go`): rubrik "Dashboard" + undertext som säger
  vad sidan bevisar. Ny **evidensrad** överst — *Blocked fail-closed*,
  *Personal data detected*, evidenslänkar (audit-export, incident-rapport,
  gap-rapport) — *före* besparingsheron. Skuggpolicy-korten visas bara när
  skuggdata finns; Learning/Shadow/Acceptance behåller ärliga tomlägen
  (produktprincip; testad i `TestDashboardSectionsHonestEmptyStates`).
- **Navigation** (`adminshell.go`): posterna grupperade i **Evidence**
  (Dashboard, Request log) · **Control** (Policy, Models, Providers, Keys,
  Users) · **Show** (Demo prompts, Live chat, Connect). Varumärke "Sluss".
- **Request log** (`logpage.go`): ny kolumn **Sensitivity** (pill), och
  blockkoden visas under uppgiftstypen på blockerade rader — loggen svarar
  nu på "varför" utan att öppna raden.
- **Policy-konsolen** (`policypage.go`, `policyrules.go`): konsolregler kan
  villkora på **agent capability** (`tool_risk` ∈ read/write/external/
  destructive) med kryssrutor, samma vokabulär som ISSUE-111; genererad
  YAML blir `tool_risk: { in: [...] }`; validering avvisar okända klasser;
  WhenSummary skriver "agent capability: …". Rubriker: "Compliance packs
  (modules)", "active", "N rules".
- **Providers** (`providers.go`): **Edit** per roster-provider — förifyller
  formuläret (id låst, namn, URL, nyckel-env, taggar), rubrik
  "Edit connection: <id>", Save changes/Cancel. Sparas via befintlig upsert;
  nyckeln är fortfarande ett env-var-*namn*, aldrig ett värde.
- **Keys / Users**: begripliga rubriker och etiketter ("Project (optional)",
  "Scopes (comma-separated, empty = all)", "Role", "active/disabled").
- **Landing** (`landing.go`): GitHub-länk i nav + sidfot
  "Open source · AGPL-3.0" — öppen kod är nu ett säljargument (ISSUE-112).

## Demo-konsekvens

Beat 9 (agenten) kan nu köras **utan** att byta till NIS2-packet: skapa en
konsolregel "agent capability: destructive → local" live inför betraktaren
(regelbytet auditeras), kör dry-run med tools-fältet, ta bort tools → moln.
Demo-manuset uppdaterat.

## Tester

- `TestConsoleRuleToolRiskRoundTrip` (validering, YAML, summary, formparsning)
- befintliga server-/dashboard-tester gröna; skärmdumpsgenomgång av alla
  sidor mot lokal instans (mock-provider + `builtin:nis2-baseline`).

## Acceptans

- [x] Dashboard leder med evidens; besparing kvar men underordnad
- [x] Sidomenyn grupperad Evidence / Control / Show
- [x] Logg visar känslighet + blockkod per rad
- [x] Konsolregel på `tool_risk` skapas, aktiveras och matchar i dry-run
- [x] Provider kan redigeras utan att tas bort
- [x] `go test ./...` grönt (utom sandbox-latency-flaken), secret-scan rent
