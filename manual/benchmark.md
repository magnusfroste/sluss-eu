# Benchmark: measure Sluss, and measure anything else the same way

**What you get:** how well Sluss finds sensitive data on 444 labelled questions, how often it flags harmless ones, how that compares with an open-source PII detector, and how to put your current gateway (Zscaler, Netskope, Purview or anything else) through the same questions.

**What you need:** to read: nothing. To run: Go and a copy of the repository.

**Time:** 5 minutes to read, 10 seconds to run.

## Why a benchmark

Every AI gateway and DLP product says it finds sensitive data. Few say how often it does not, and fewer say how often it flags something harmless, which is what makes people work around it. No vendor publishes those numbers for its engine. We will not quote numbers nobody has published, and we will not invent them. What we can do is publish the method, the questions and our own result, and make it easy for you to run the same questions through any other tool and score it the same way.

## The questions

444 synthetic questions in Swedish and English (`benchmark/corpus.jsonl`), written by a generator with a fixed seed (`benchmark/generate.py`), so anyone gets exactly the same set. Nothing in it belongs to anyone: personal identity numbers come from Skatteverket's open list of test numbers, Swedish phone numbers from the range the telecom authority reserves for fiction, card numbers use a test prefix, and names, companies and amounts are made up. Card numbers and IBANs have valid check digits. Each question has the data classes a careful reader would give it, and a *kind*:

| Kind | Questions | What it tests |
|---|---|---|
| Identifier or key | 104 | A personal identity number, e-mail, phone number, address, card, IBAN, key or password, or pasted code |
| Term or marking | 62 | *HEMLIG*, a diagnosis, a balance sheet, an NDA, in words a department would use |
| Meaning only | 58 | Sensitive only because of what it says: a name with a story, an illness in plain words, a deal or a dispute without a financial or legal term |
| Look-alike | 110 | Looks sensitive and is not: dates, order and invoice numbers, versions, prices, *please be patient*, a question about password policy |
| Plain | 110 | Everyday work questions |

224 are sensitive and 220 are harmless.

**How the set is kept honest.** The first run of this benchmark (version 1.8.0) found five faults in the rules. They were fixed in 1.8.1, each tested on new examples that are not in the set (`internal/classifier/benchmark_fixes_test.go`), and the set was then regenerated with a new seed. The questions follow the same patterns as before with other names, numbers and combinations, so the numbers below do show the fixes. That is the point of publishing both: the table under *Before and after* is the same rules measured before they had seen any of it.

## Results

Version 1.8.1.

- **Sluss, rules only**: measured in process (`make benchmark`), each question five times.
- **Presidio**: Microsoft's open-source PII detector (`presidio-analyzer` 2.2.366 with spaCy 3.8 and `en_core_web_lg`, its English setup out of the box, score threshold 0.5, dates and URLs ignored), run with `benchmark/presidio.py`. Any entity it finds counts as *flagged*; a Swedish model or custom recognisers would change its numbers, and the script is there to try.

| | Sluss, rules only | Presidio |
|---|---|---|
| Sensitive questions flagged | 166 of 224 (74 %) | 139 of 224 (62 %) |
| Harmless questions flagged | 22 of 220 (10 %) | 41 of 220 (19 %) |
| — with an identifier or key | 104 of 104 (100 %) | 74 of 104 (71 %) |
| — with a term or marking | 62 of 62 (100 %) | 27 of 62 (44 %) |
| — with meaning only | 0 of 58 (0 %) | 38 of 58 (66 %) |
| — look-alikes left alone | 88 of 110 (80 %) | 73 of 110 (66 %) |
| — plain questions left alone | 110 of 110 (100 %) | 106 of 110 (96 %) |
| Right class, exactly | 360 of 444 (81 %) | 251 of 444 (57 %) |
| Same answer every run | yes (5 runs) | not measured |
| Time per question (median / p95) | 0.09 ms / 0.14 ms | 5 ms / 8 ms |

| Class | Sluss recall / precision | Presidio recall / precision |
|---|---|---|
| Personal data | 0.82 / 0.94 | 0.94 / 0.41 |
| Keys and secrets | 0.75 / 0.71 | — |
| Security-classified | 0.67 / 1.00 | — |
| Health | 0.62 / 0.73 | — |
| Financial | 0.62 / 0.84 | — |
| Legal | 0.64 / 0.82 | — |
| Source code | 1.00 / 1.00 | — |

*Recall* is the share of questions with that class in which the tool found it. *Precision* is the share of the tool's findings of that class that were right.

## What the numbers say

- **Rules find everything there is to find.** Every identifier, key, term and marking in the set, in a tenth of a millisecond and the same way every time. Presidio has no notion of keys, security markings, health, finance, legal or code, which is most of what an organisation means by sensitive.
- **Rules do not read meaning, at all.** 0 of 58 meaning-only questions: every miss in the set is of this kind. Presidio flags 38 of them, every one because it recognises a person's name (PERSON). The same habit makes it flag names in harmless questions (precision 0.41 on personal data), and its 27 *term* hits are a name or a place in the question, sometimes a Swedish word taken for one, never the term itself.
- **Rules still over-flag on some words.** 10 % of harmless questions, all look-alikes: *password policy*, *how API keys should be rotated*, *what an IBAN is*, *patient*, *court cases*. A question about passwords is deliberately treated as possibly holding one. That costs no protection; such a question is kept on your own models instead of the cloud.
- **The second opinion is what reads meaning.** With a model on your own hardware reading the question after the rules ([How Sluss decides](/manual/how-sluss-decides)), measured on version 1.8.0 against the demo instance's local model (a 27B reasoning model with thinking turned off), each question twice: *meaning only* went from 4 to 43 of 58 and sensitive questions flagged from 71 % to 93 %, with 11 more harmless questions flagged (a git commit message read as code, *the summer party* as personal data) and about a quarter of a second per question including the network. It gave the same answer both times for all 444 questions; that is an observation on this model, not a guarantee, and only the rules' part is the same by construction. What it still missed: a performance review of a named person, a backup site for critical infrastructure with no marking, and *has had it tough since the accident*. A different model gives different numbers; measure yours with the commands below.

## Before and after

The same rules, before the benchmark had been run on them (version 1.8.0, the first set of questions) and after the five fixes (version 1.8.1, the regenerated set):

| | 1.8.0 | 1.8.1 |
|---|---|---|
| Sensitive questions flagged | 71 % | 74 % |
| Harmless questions flagged | 18 % | 10 % |
| — with an identifier or key | 90 % | 100 % |
| — with meaning only | 7 % | 0 % |
| — look-alikes left alone | 65 % | 80 % |
| Right class, exactly | 75 % | 81 % |

*Meaning only* went down because the four hits in 1.8.0 were accidents: a word such as *email* in a sentence that happened to be sensitive for another reason.

What was fixed:

- `db_password=…` and other labels behind an underscore are seen as a password.
- An address with the street in two words (*Lilla torget 47*, *Stora gatan 12*).
- One complete SQL statement without a code fence is code.
- The words *email*, *phone* and *address* alone are no longer personal data; the values still are.
- A word about a code topic (*invoice*, *subscription*, *checkout*) is source code only when the task is a code task. An invoice reminder is a letter.

What was left as it is: the word *password*, *token* or *api key* still flags a question as possibly holding a secret. A miss there is a leaked password; a false alarm is a question answered by your own model.

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
