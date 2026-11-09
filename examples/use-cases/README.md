# Use cases

Fifteen people and agents ask Sluss real questions. One policy, three outcomes a rule can have, and what Sluss does with each question. Everything is synthetic: no real personal data.

Run them yourself, no key and no network needed:

```bash
make use-cases          # one line per case; fails if an outcome differs
MARKDOWN=1 make use-cases   # the table below
```

`make use-cases` starts a fresh Sluss with two test providers: *anthropic* plays a model on your own hardware, *openai* a cloud model. It loads [`policy.yaml`](policy.yaml) and sends every question in [`cases.yaml`](cases.yaml) to the dry-run (`POST /router/decision`), which calls no model. For a *mask* case it also sends the question through `/v1/chat/completions`, to check that a real request is masked. CI runs it on every change.

## The policy

| When the question holds | The rule says |
|---|---|
| security-classified material | **block** |
| an agent tool that can delete or change things | **keep on own hardware** |
| health, legal or financial data, or a key | **keep on own hardware** |
| personal data (identity numbers, e-mail, phone, IBAN, card, address) | **mask and send** |
| anything else | the best model for the price |

When several rules match, the strictest wins.

## What happens

| Who | Asks | Sluss finds | Outcome |
|---|---|---|---|
| Marketing | Write a short LinkedIn post announcing our new office in Malmö. | — | cloud model |
| Developer | Write a git commit message for this diff that renames getUser to fe… | — | cloud model |
| Developer | Why does this fail? … | secrets_possible | own hardware |
| Customer support | Summarise this support ticket from customer 811218-9876 (anna.svens… | pii | cloud, masked (email, personnummer, phone) |
| Customer support | Draft a friendly reply to customer 811218-9876 (anna.svensson@examp… | pii | own hardware |
| Customer support | Summarise the call notes and confirm the callback number +44 20 794… | pii | cloud, masked (phone) |
| HR | Erik Larsson, 811218-9876, has been on sick leave since March with … | pii, health | own hardware |
| Finance | Here is the balance sheet and P&L for Q3 before the earnings releas… | financial | own hardware |
| Finance | Summarise this invoice note in two lines; payment goes to SE45 5000… | pii | cloud, masked (iban) |
| Legal | Review this NDA and the indemnification clause before we sign. | legal | own hardware |
| Defence supplier | HEMLIG enligt säkerhetsskyddslagen. Sammanfatta lägesbilden; API-ny… | security_classified (and more) | blocked |
| Anyone | Det är ingen hemlighet att vi vill växa; skriv en hemlig önskelista… | — | cloud model |
| Logistics | Mötet flyttas till 08-11-2024; ordernummer 0812-345 678 är leverera… | — | cloud model |
| An AI agent | Clean up the staging database tables older than 90 days. *(tools: drop_table, delete_rows)* | agent tools: destructive | own hardware |
| An AI agent | Find the three most recent release notes and summarise them. *(tools: search_docs, read_file)* | agent tools: read | cloud model |

*Own hardware* means the model on your own servers answered. *Cloud, masked* means a cloud model answered, with the listed identifiers replaced by placeholders such as `[EMAIL_1]` before it saw the question; the values were put back in the answer before the person saw it.

## What to notice

- **The same kind of data, different outcomes, for a reason.** Personal data alone is masked and sent (*support ticket*). Personal data with health data stays local (*HR sick leave*): the stricter rule wins.
- **Mask and send allows the cloud; it does not force it.** The open-ended support reply is rated high risk and needs a premium model. In this setup the only premium model is the local one, so the question stays local and nothing needs masking.
- **Values, not words.** The pasted key is found because it *is* a key, not because the word "key" appears. Lower-case *hemlig* is prose, not a stamp. A date and an order number are not phone numbers.
- **Agents are judged by what they can do.** The same Sluss sends a read-only agent to the cloud and keeps one that can drop tables on your own hardware, before any model sees the prompt.
- **A block holds even when another rule would keep the question local.** The classified document also mentions a key; it is blocked, not kept local.

## Add your own

Each case is a few lines in [`cases.yaml`](cases.yaml): who asks, the question, and the outcome you expect (`cloud`, `local`, `mask` or `block`), optionally the data classes Sluss must find and the types it must mask. Change [`policy.yaml`](policy.yaml) to try your own rules. Point the runner at your own instance to see what your policy does with the same questions:

```bash
go run ./cmd/usecases -url https://sluss.example.eu -key <admin key> -local <your local provider ids>
```

## Evaluating Sluss?

If you, or an agent working for you, are judging whether Sluss is fit for your organisation, start with [EVALUATE.md](../../EVALUATE.md): what to run, what to try to break, and how to write it up.
