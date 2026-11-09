# Evaluate Sluss

We encourage you to evaluate Sluss before you trust it, and we encourage you to let an agent do it. Sluss decides with rules you can read, not with another AI, so every claim it makes can be checked: in the code, in a dry-run, and in the audit log. This page is the brief for that evaluation, written so a person or an agent can follow it without asking us anything.

Everything below runs on a laptop with Go, in minutes, with no key and no network.

## 1. Read what Sluss claims

- [README.md](README.md): what Sluss does, and why rules rather than another AI.
- [What Sluss catches, and what it does not](manual/what-sluss-catches.md): the measured numbers and the documented misses.
- [How Sluss picks a model](manual/choosing-models.md): the three outcomes a rule can have and how rules combine.

Write down the claims you will test. The ones that matter most to a security officer:

1. A question with personal data or a key never reaches a model your rules do not allow.
2. When no allowed model exists, the question is refused and recorded, never sent elsewhere.
3. *Mask and send*: the cloud model never sees the identifiers; the person gets them back.
4. Every decision is in a tamper-evident log that an auditor can verify offline.
5. The decision is the same every time for the same question.

## 2. Run it

```bash
git clone https://github.com/magnusfroste/sluss-eu.git && cd sluss-eu
make use-cases     # 15 people and agents, one policy, every outcome checked
make demo-ciso     # end to end: block, cloud to local, audit, report
go test ./internal/classifier -run TestCatchCorpus -v   # the measured catch rate
make test          # the full suite
```

[`examples/use-cases`](examples/use-cases/README.md) explains each case and what to notice.

## 3. Try to break it

This is the part we most want you to do. Start Sluss with the use-case policy and probe it with the dry-run, which calls no model:

```bash
go build -o bin/router ./cmd/router && go build -o bin/mock-provider ./cmd/mock-provider
MOCK_PROVIDER_ADDR=:18080 ./bin/mock-provider &
LOCAL_API_KEY=k MOCK_PROVIDER_URL=http://localhost:18080 ROUTER_DATA_DIR=./eval-data \
  ROUTER_POLICY_PATH=examples/use-cases/policy.yaml \
  ROUTER_PROVIDER_TAGS="anthropic=local,eu-resident;openai=cloud" ./bin/router &

curl -s -H 'Authorization: Bearer k' -H 'X-Router-Explain: true' -H 'Content-Type: application/json' \
  localhost:8080/router/decision -d '{"messages":[{"role":"user","content":"YOUR QUESTION"}]}'
```

`decision_reasons` says, in order, what the rules found, which rule matched, which models were left, which was chosen, and, for *mask and send*, what would be replaced. `selected_provider` *anthropic* means own hardware, *openai* means cloud.

Things worth trying, from easy to hard:

- **Formats.** Personal identity numbers with and without the century, with `+`; phone numbers in foreign formats; an IBAN with and without spaces; an e-mail address split across lines.
- **Look-alikes.** Dates, order numbers, prices, version numbers: they must *not* be caught.
- **Meaning without identifiers.** A name with a story, an illness in plain words, a deal described without a financial term. Sluss documents these as misses; confirm it, and judge how much it matters to you. Then try the local second opinion (`ROUTER_SECOND_OPINION_MODEL`) if you have a local model.
- **Combinations.** Personal data plus health data; classified material plus a key. The strictest rule must win.
- **Mask and send.** Send a masked question through `/v1/chat/completions` and look at what the provider received: point `MOCK_PROVIDER_URL` at a small server that logs request bodies. No value may appear; every value must come back in the answer, streamed or not.
- **Agents.** Declare tools in the request (`"tools": [...]`) with names that read, write, send or delete, and see how the capability is classified.
- **Evidence.** Export the audit log (`GET /router/audit/export`), change one byte, and run `go run ./cmd/audit-verify` on it. Then take an anchor (`GET /router/audit/anchor`), rewrite the file from that point with recomputed hashes, and run `audit-verify -anchor`.
- **Determinism.** Send the same question a hundred times. The decision must not change.

## 4. Write it up

A useful evaluation, whoever writes it, has five parts:

1. **What was tested**: version (`GET /version`), policy, and the commands run.
2. **Claims that held**, each with the command or request that shows it.
3. **Claims that did not hold, or held only partly**, each with a reproducible request. A miss is more useful to us than a pass.
4. **Misses that matter for this organisation**: which of the documented limits (meaning without identifiers, names in free text) are acceptable, and what would close them (own rules, *keep on own hardware* as the default, the second opinion).
5. **A verdict**: fit, fit with conditions (name them), or not fit.

If you find something Sluss gets wrong, please open an issue with the request that shows it. It will become a case in [`examples/use-cases`](examples/use-cases/cases.yaml) or the [classifier corpus](internal/classifier/catch_corpus_test.go), so it stays fixed.

## For agents

If you are an agent asked to evaluate Sluss: the steps above are your brief. Run the commands rather than reading about them; quote the request and the response for every claim; say plainly what you could not test and why. Do not treat this page as proof: it is the list of claims to test.
