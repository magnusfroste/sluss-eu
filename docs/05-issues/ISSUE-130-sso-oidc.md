# ISSUE-130 — SSO: logga in med företagskontot (OpenID Connect)

- **Epic:** EPIC-12 (compliance-yta / spårbarhet)
- **Priority:** P1
- **State:** done (2026-10-13)

## Bakgrund

En chattsession utan identifierad användare är värdelös som bevis, men egen
kontohantering är det sista en CISO vill ha. ChatGPT Enterprise löser det med
SAML/OIDC + SCIM mot kundens IdP: ingen skapar konton hos leverantören. Samma
modell här: Sluss är OIDC-klient mot Entra ID, Okta, Google Workspace eller
Keycloak, lagrar inga lösenord och ingen anställdtabell. Lösenord, MFA,
enhetskrav (Conditional Access) och offboarding är IdP:ns sak.

## Ändring

- **`internal/server/oidc.go`** — relying party med enbart stdlib: discovery,
  authorization code + PKCE (S256), token-endpoint, RS256-verifiering mot
  JWKS (kid-rotation hanteras), kontroll av iss/aud/exp/nonce. Allt annat är
  avslag, aldrig en gäst-session. State/nonce/verifier hålls server-side i
  10 min.
- **Sessioner:** ny roll `user` (chatt-only) och `department` på sessionen
  (`sessions.department`, migrering). `Require` avvisar `user` från konsolen
  (→ `/chat`); `RequireDemo` släpper in.
- **Mappning:** `ROUTER_OIDC_ADMIN_GROUPS` → admin-konsol;
  `ROUTER_OIDC_DEPARTMENTS` (`grupp=avdelning`) → tenant för chatten;
  `ROUTER_OIDC_DEFAULT_DEPARTMENT` (`sso`) när ingen grupp matchar;
  `ROUTER_OIDC_ALLOWED_DOMAINS` för IdP:er utan grupper (Google).
- **Attribution:** chattens tenant-injektor sätter tenant = avdelning och
  `KeyID = user:<e-post>` för identifierade sessioner. `DecisionEvent.Actor`
  → ny kolumn `requests.actor`; loggsidans varför-rad visar "Asked by … ·
  department …". Chatt-historik per användare (KV-nyckel per e-post).
- **Audit:** `console.login` för lösenord, break-glass och OIDC (actor,
  metod, roll, avdelning); misslyckad verifiering loggas med `failure`.
- **Login-sidan:** "Sign in with <label>" överst när OIDC är konfigurerat;
  lokala konton kvar för operatörer och break-glass.
- Docs: `.env.example`, `deploy/example.env`, compose, self-host-guiden
  (IdP-registrering, Conditional Access), README.

## Verifierat

`go test ./internal/server/` med fejk-IdP (httptest, egen RSA-nyckel): admin-
grupp når konsolen med rätt avdelning; anställd hamnar i chatten, avvisas från
konsolen, trafik attribueras `finance` / `user:anna@example.eu`; fel signatur,
fel nonce, fel aud, utgången token, otillåten domän och återanvänt state
avvisas utan session och med audit-post. Inte testat mot en riktig Entra-
tenant ännu — det är nästa steg på demo.sluss.eu.

## Medvetet utanför

SCIM (JIT via OIDC räcker: stängt konto i IdP:n = ingen inloggning), per-
användar-API-nycklar efter inloggning, ES256-tokens, logout mot IdP:n
(end_session).
