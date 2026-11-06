# Where each setting lives

**What you get:** you know which settings are changed in the console and which are changed by whoever runs the server, and why a new API key needs a restart while a new model does not.

**What you need:** nothing. This page explains what already happens.

**Time:** 5 minutes to read.

## Two places, two words

Every setting of Sluss lives in one of two places. Use the two words, and there is never any doubt about who changes what.

| | **The environment** | **The database** |
|---|---|---|
| What lives there | Secrets and infrastructure | Everything the console edits |
| Read | Once, when Sluss starts | When Sluss starts, and again on every change in the console |
| Changed by | Whoever runs the server (EasyPanel, Docker, the host) | An administrator, in the console |
| Survives a restart | Yes, as long as the variable is still set | Yes, it is on the data volume |

"Runtime" is not wrong, but it does not separate the two: everything applies while Sluss runs. The difference is *where* a setting is kept and *when* it is read.

## The environment

The environment is the list of variables the server is started with. In EasyPanel it is the service's *Environment* tab; with Docker it is the `-e` flags or the `.env` file. Sluss reads it once, at start.

What belongs there:

- **API keys.** `OPENROUTER_API_KEY` for cloud models, and one variable per local server that takes a key, with a name you choose, for example `GPU_BOX_KEY`. The database only ever holds the *name* of the variable, never its value. That is deliberate: a copy of the database gives nobody a key.
- **`LOCAL_API_KEY`**, the first key clients use before you create department keys in the console.
- **`ROUTER_DATA_DIR`**, where the database and the audit log live. In a container it is `/data`.
- **`ROUTER_POLICY_PATH`**, the compliance pack Sluss starts from. The rules you add in the console sit on top of it and live in the database.
- **`ROUTER_DASHBOARD_PASSWORD`**, the break-glass login, for an operator who is locked out.
- **`ROUTER_ADMIN_USER` and `ROUTER_ADMIN_PASSWORD`**, read only once: when the user table is empty, they create the first account. After that they do nothing. Change the password on [Your profile]({{BASE}}/me), and feel free to remove the two variables.
- **Sign-in with the company account** (`ROUTER_OIDC_*`), the public address, web search, the version check and the price list.

The full list, explained, is in [`deploy/example.env`](https://github.com/magnusfroste/sluss-eu/blob/main/deploy/example.env).

## The database

The database is one file, `history.db`, under the data directory. Everything an administrator changes in the console is written there:

- **Providers** ([Providers]({{BASE}}/router/providers)): each connection's name, address, the *name* of its key variable, and its tags, for example *local* and *on-prem*.
- **Models** ([Models]({{BASE}}/router/models)): each model, which provider serves it, its name at the provider, its level (cheap, balanced or premium), its price, and whether it is enabled.
- **Console users** and **department keys**, the **rules** added in the console, the **request log** (what kind of question went where, never its text), each person's **chat history**, and **demo links**.

The audit log is *not* in the database. It is its own file, `audit-chain.jsonl`, next to it, so that an auditor can verify it on its own.

## What happens at start

1. Sluss reads the environment.
2. If the database is empty, it writes a starting roster into it: the OpenRouter connection and three models, one per level. Otherwise it reads the providers and models that are there.
3. For each provider, it looks up the key variable *by name* in the environment. If the value is there, the provider's models can be chosen. If it is missing, the provider is listed but its models are not routable, and the Providers page says *key missing*. A local server that takes no key has no key variable, and needs none.
4. From then on, a change on Models or Providers applies at once. The one exception is a **new key variable**: the environment is read only at start, so adding a key to a new provider means restarting Sluss. The Providers page says *awaiting restart* until you do.

## An example

A company runs Sluss with three providers. In the environment: `OPENROUTER_API_KEY`, `RTX_API_KEY` and `DGX_API_KEY`. In the database: OpenRouter (cloud), *GPU North* at `https://north.example.internal` with key variable `RTX_API_KEY`, and *GPU South* at `https://south.example.internal/v1` with key variable `DGX_API_KEY`, both tagged *local*. The models behind them, with their levels, are in the database too.

To rotate the key for GPU North, the operator changes `RTX_API_KEY` in EasyPanel and restarts. Nothing in the console changes. To raise GPU North's model to *premium*, an administrator changes the level on Models, and it applies at once. Nothing in the environment changes.

## If you are unsure

The boot banner at the start of the log says where the data directory is and which providers answered. The [Providers]({{BASE}}/router/providers) page says, for each connection, whether its key is set.
