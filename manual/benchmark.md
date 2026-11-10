# Benchmark: measure Sluss, and measure anything else the same way

**What you get:** how well Sluss finds sensitive data on 444 labelled questions, how often it flags harmless ones, how that compares with an open-source PII detector, and how to put your current gateway (Zscaler, Netskope, Purview or anything else) through the same questions.

**What you need:** to read: nothing. To run: Go and a copy of the repository.

**Time:** 5 minutes to read, 10 seconds to run.

## Why a benchmark

Every AI gateway and DLP product says it finds sensitive data. Few say how often it does not, and fewer say how often it flags something harmless, which is what makes people work around it. No vendor publishes those numbers for its engine. We will not quote numbers nobody has published, and we will not invent them. What we can do is publish the method, the questions and our own result, and make it easy for you to run the same questions through any other tool and score it the same way.

## The questions

444 synthetic questions in Swedish and English (`benchmark/corpus.jsonl`), written by a generator with a fixed seed (`benchmark/generate.py`), so anyone gets exactly the same set. Nothing in it belongs to anyone: Swedish personal identity numbers come from Skatteverket's open list of test numbers, Finnish and Norwegian ones use the forms those countries reserve for tests, Swedish phone numbers come from the range the telecom authority reserves for fiction, card numbers use a test prefix, and names, companies and amounts are made up. Each question has the data classes a careful reader would give it, and a *kind*:

| Kind | Questions | What it tests |
|---|---|---|
| Identifier or key | 104 | A personal identity number (Swedish, Finnish, Norwegian, US), e-mail, phone number, address, card, IBAN, passport, registration number, date of birth, key or password, or pasted code |
| Term or marking | 70 | *HEMLIG*, a diagnosis, a balance sheet, an NDA, a list of trade union members, in words a department would use |
| Meaning only | 58 | Sensitive only because of what it says: a name with a story, an illness in plain words, a deal or a dispute without a financial or legal term |
| Look-alike | 110 | Looks sensitive and is not: dates, order and invoice numbers, versions, prices, *please be patient*, a question about password policy, *what is an IPO?*, a school essay about Astrid Lindgren, a poster for diabetes awareness week |
| Plain | 102 | Everyday work questions |

232 are sensitive and 212 are harmless.

**How the set is kept honest.** The set has changed twice, and both times the numbers are shown before and after (below). In 1.8.1 five faults it had found were fixed and it was regenerated with a new seed. In 1.9 the rules were made much broader (names, more identity numbers, more key formats, larger word lists), and the set got harder at the same time: new sensitive questions for what is new, and new look-alikes written to trip the new rules, such as famous people and general questions that use the new words. Every rule change is also tested on examples that are not in the set (`internal/classifier/benchmark_fixes_test.go`, `broader_test.go`). The people who write the rules also write the set; that is why the questions, the generator and the scoring are published, and why you are invited to add your own.

## Results

Version 1.9.

- **Sluss, rules only**: measured in process (`make benchmark`), each question five times.
- **Presidio**: Microsoft's open-source PII detector (`presidio-analyzer` 2.2.366 with spaCy 3.8 and `en_core_web_lg`, its English setup out of the box, score threshold 0.5, dates and URLs ignored), run with `benchmark/presidio.py`. Any entity it finds counts as *flagged*; a Swedish model or custom recognisers would change its numbers, and the script is there to try.

| | Sluss, rules only | Presidio |
|---|---|---|
| Sensitive questions flagged | 188 of 232 (81 %) | 159 of 232 (69 %) |
| Harmless questions flagged | 30 of 212 (14 %) | 51 of 212 (24 %) |
| — with an identifier or key | 104 of 104 (100 %) | 72 of 104 (69 %) |
| — with a term or marking | 70 of 70 (100 %) | 48 of 70 (69 %) |
| — with meaning only | 14 of 58 (24 %) | 39 of 58 (67 %) |
| — look-alikes left alone | 80 of 110 (73 %) | 63 of 110 (57 %) |
| — plain questions left alone | 102 of 102 (100 %) | 98 of 102 (96 %) |
| Right class, exactly | 365 of 444 (82 %) | 236 of 444 (53 %) |
| Same answer every run | yes (5 runs) | not measured |
| Time per question (median / p95) | 0.14 ms / 0.20 ms | 5 ms / 8 ms |

| Class | Sluss recall / precision | Presidio recall / precision |
|---|---|---|
| Personal data | 1.00 / 0.83 | 0.88 / 0.36 |
| Keys and secrets | 0.75 / 1.00 | — |
| Security-classified | 0.67 / 1.00 | — |
| Health | 0.62 / 0.55 | — |
| Financial | 0.62 / 0.94 | — |
| Legal | 0.64 / 0.78 | — |
| Source code | 1.00 / 1.00 | — |

*Recall* is the share of questions with that class in which the tool found it. *Precision* is the share of the tool's findings of that class that were right.

## What the numbers say

- **Rules find everything there is to find.** Every identifier, key, term and marking in the set, and now every full name, in a fraction of a millisecond and the same way every time. Presidio has no notion of keys, security markings, health, finance, legal or code; what it flags among those questions is a name or a place in them.
- **Names are found, and that has a price.** A given name followed by a family name is personal data: 14 of the meaning-only questions are caught that way. The rule cannot tell a customer from an author, so 11 questions about Astrid Lindgren, Olof Palme and Marie Curie are flagged too. They are kept on your own models; if that is wrong for you, `ROUTER_NAME_DETECTION=off` turns the rule off.
- **A general question is not data.** *What is an IBAN?*, *how should API keys be rotated?*, *what does a district court do?* use the words without holding anything: the rules now let them pass. A poster for diabetes awareness week does not pass: it is not a question, and *diabetes* is a health word. That is most of the remaining 14 %.
- **Rules still do not read meaning.** 44 of 58 meaning-only questions are missed: a password with no label, an illness never called one, a deal or a dispute in everyday words, a plan to protect a waterworks with no marking. No word list fixes that.
- **The second opinion is what reads meaning.** A model on your own hardware reading the question after the rules ([How Sluss decides](/manual/how-sluss-decides)). On version 1.8.1 and that version's questions it took sensitive questions flagged from 74 % to 92 %, with 13 % of harmless questions flagged and about a third of a second per question. It is measured again on this set after each release; run the commands below against your own model.

## Before and after

Rules only, as the rules and the set have changed:

| | 1.8.0 | 1.8.1 | 1.9 |
|---|---|---|---|
| The set | first | new seed | broader and harder |
| Sensitive questions flagged | 71 % | 74 % | 81 % |
| Harmless questions flagged | 18 % | 10 % | 14 % |
| — with an identifier or key | 90 % | 100 % | 100 % |
| — with meaning only | 7 % | 0 % | 24 % |
| — look-alikes left alone | 65 % | 80 % | 73 % |
| Right class, exactly | 75 % | 81 % | 82 % |

**1.8.1** fixed five faults the first run found: `db_password=…` and other labels behind an underscore; an address with the street in two words (*Lilla torget 47*); one complete SQL statement without a code fence; the words *email*, *phone* and *address* alone are no longer personal data; and a word about a code topic (*invoice*, *checkout*) is source code only when the task is a code task.

**1.9** made the rules broader, the way a DLP product's dictionaries are broad:

- **Names** in free text, and later mentions of the same given or family name.
- **More identity numbers:** Finnish and Norwegian (check digits), US and UK (format rules), Danish next to the word; a passport number, a vehicle registration and a date of birth next to the word that says what it is.
- **More key formats:** Stripe, GitLab, npm, Hugging Face, SendGrid, Twilio, DigitalOcean, Shopify, Azure storage keys, Slack, Discord and Teams webhook addresses; and the Swedish words for a password.
- **Larger word lists:** conditions, care and the papers around them; deals, credit and tax; courts and the documents a lawyer handles; registers of people, records an employer keeps, and the special categories of the GDPR; critical infrastructure and total defence.
- **A general question** does not get a class from a word alone.

The harmless share went up from 10 % to 14 % because the set now asks harder questions of broader rules: on the 1.8.1 set, these rules flag 80 % of the sensitive questions and 4 % of the harmless ones. That 4 % flatters: the rule for general questions was written by people who had seen those look-alikes.

`go run ./cmd/benchmark score -misses <predictions>` lists every question behind every number.

## Run it

```bash
make benchmark                                            # rules only, in process, also in CI
go run ./cmd/benchmark run -repeat 5 -out rules.jsonl    # the same by hand
go run ./cmd/benchmark score -markdown rules.jsonl        # the tables above
go run ./cmd/benchmark score -misses rules.jsonl          # every miss and false alarm
```

Against a running Sluss, to include a second opinion or your own policy's rules (the dry-run calls no model for an answer):

```bash
go run ./cmd/benchmark run -url {{BASE}} -key <api key> -out yours.jsonl
go run ./cmd/benchmark score rules.jsonl yours.jsonl
```

## Measure your current gateway the same way

```bash
go run ./cmd/benchmark export -dir benchmark-export
```

writes `questions.csv`, one text file per question in `questions/`, and `predictions-template.csv`. Send the questions through the tool you want to measure, the way your people would: pasted into ChatGPT behind Zscaler, uploaded as files, or sent to its API. For each question, write in the template whether the tool flagged or blocked it (`yes` or `no`) and, if it says, which kind of data (`pii;health`). Then:

```bash
go run ./cmd/benchmark score -markdown rules.jsonl benchmark-export/predictions-template.csv
```

gives the same tables with that tool beside Sluss. The questions are synthetic, so nothing real leaves your organisation while you do it. If you publish the result, publish the filled-in file with it, so it can be checked the way ours can.
