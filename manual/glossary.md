# Words used in this manual

| Word | What it means here |
|---|---|
| **Sluss** | Your AI gateway. It sits between people and tools on one side and AI models on the other. |
| **Model** | An AI service that answers questions, in the cloud (for example GPT-4o) or on your own hardware. |
| **Local model** | A model on hardware the company controls. Data sent to it does not leave the company. |
| **Level (tier)** | How hard a question a model is trusted with: cheap, balanced or premium. You set it per model. A question goes to a model at or above the level it needs. See [How Sluss picks a model](/manual/choosing-models). |
| **Rule / policy** | What Sluss may do with a question, for example "personal data only to local models". |
| **Fail-closed** | When no allowed model exists, the question is refused. It is never sent somewhere else just to get an answer. |
| **Audit log** | The record of every decision and change. Each line is chained to the one before, so a changed line is detected. |
| **Identity provider (IdP)** | The system that checks who someone is when they sign in, such as Microsoft Entra ID, Okta or Google Workspace. |
| **OIDC** | The standard Sluss uses to ask the identity provider who someone is. You only need the three values the identity provider gives you. |
| **Group** | A list of people in the identity provider, such as "finance". Sluss uses groups to decide who is an admin and which department a question belongs to. |
| **MDM** | Mobile device management: the tool IT uses to manage company computers and phones, such as Intune or Jamf. |
| **DNS filter / web filter** | The service that decides which websites company devices may open, such as Defender, Cloudflare, Umbrella or Zscaler. |
| **Block page** | The page someone sees when the filter stops a site. Sluss provides one that points to the company chat. |
| **DNS-over-HTTPS** | A browser feature that can go around the company's filter. The browser policy from Rollout turns it off. |
| **Department key** | A key for one team. Requests made with it are logged under that team. |
| **Coverage check** | The small script that reports, once a day, whether a computer can still reach AI services directly. |
