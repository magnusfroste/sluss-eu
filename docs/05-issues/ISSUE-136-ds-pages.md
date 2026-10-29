# ISSUE-136 — Models, Access och Policy i designsystemet

- **Epic:** EPIC-15 (produktyta)
- **Priority:** P1
- **State:** done (2026-10-29)

## Ändring

- **Gemensamt sidhuvud** `dsHead(active, title, sub)`: rubrik, en mening,
  och — när sidan hör till en menygrupp med flera sidor — gruppens sidor som
  **flikar** (Models · Providers · Risk register; Department keys · Console
  users). Flikarna ersätter undermenyn i sidomenyn: en väg, inte två.
- **Samma ram på alla sidor**: innehållskolumn max 1200 px, samma marginaler,
  kortradie och tabellrytm via `designFrameCSS` — utan att varje mall skrivs
  om. Policy-sidan var 900 px bred, Models 1100, Keys utan gräns; nu lika.
- Menyn har samma bredd på alla sidor (box-sizing i skalet).
- Rubriker och beskrivningar i klartext utan registry-version: "Department
  keys", "Console users", "Which models Sluss may use, where they run, and
  what they cost."
- Sidor: Models, Providers, Risk register, Department keys, Console users,
  Policy.

## Kvar till ISSUE-137

Activity (Request log, rapporter), Rollout-gruppen (Quick start, Managed
devices, Connect, Demo link), regeltexterna på Policy i klartext,
sidopanel för modellrader, mobilchatten.
