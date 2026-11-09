# Benchmark: measure Sluss, and measure anything else the same way

**What you get:** how well Sluss finds sensitive data on 444 labelled questions, how often it flags harmless ones, how that compares with an open-source PII detector, and how to put your current gateway (Zscaler, Netskope, Purview or anything else) through the same questions.

**What you need:** to read: nothing. To run: Go and a copy of the repository.

**Time:** 5 minutes to read, 10 seconds to run.

## Why a benchmark

Every AI gateway and DLP product says it finds sensitive data. Few say how often it does not, and fewer say how often it flags something harmless, which is what makes people work around it. No vendor publishes those numbers for its engine. We will not quote numbers nobody has published, and we will not invent them. What we can do is publish the method, the questions and our own result, and make it easy for you to run the same questions through any other tool and score it the same way.

## The questions

444 synthetic questions in Swedish and English (`benchmark/corpus.jsonl`), written by a generator with a fixed seed (`benchmark/generate.py`), so anyone gets exactly the same set. Personal identity numbers, card numbers and IBANs have valid check digits; names, companies and amounts are made up. Each question has the data classes a careful reader would give it, and a *kind*:

| Kind | Questions | What it tests |
|---|---|---|
| Identifier or key | 104 | A personal identity number, e-mail, phone number, address, card, IBAN, key or password, or pasted code |
| Term or marking | 62 | *HEMLIG*, a diagnosis, a balance sheet, an NDA, in words a department would use |
| Meaning only | 58 | Sensitive only because of what it says: a name with a story, an illness in plain words, a deal or a dispute without a financial or legal term |
| Look-alike | 110 | Looks sensitive and is not: dates, order and invoice numbers, versions, prices, *please be patient*, a question about password policy |
| Plain | 110 | Everyday work questions |

224 are sensitive and 220 are harmless. The set was written before the rules were scored on it, and the rules have not been tuned to it; a rule fixed because of a question here is tested on new examples and the set regenerated with a new seed, or the numbers would flatter.

## Results

Version 1.7, rules only, measured in process (`make benchmark`). Presidio is Microsoft's open-source PII detector (`presidio-analyzer` 2.2.366 with spaCy 3.8 and `en_core_web_lg`, its English setup out of the box, score threshold 0.5, dates and URLs ignored), run on the same machine with `benchmark/presidio.py`. Any entity it finds counts as *flagged*; a Swedish model or custom recognisers would change its numbers, and the script is there to try.

| | Sluss, rules only | Presidio |
|---|---|---|
| Sensitive questions flagged | 160 of 224 (71 %) | 138 of 224 (62 %) |
| Harmless questions flagged | 40 of 220 (18 %) | 34 of 220 (15 %) |
| — with an identifier or key | 94 of 104 (90 %) | 75 of 104 (72 %) |
| — with a term or marking | 62 of 62 (100 %) | 31 of 62 (50 %) |
| — with meaning only | 4 of 58 (7 %) | 32 of 58 (55 %) |
| — look-alikes left alone | 71 of 110 (65 %) | 79 of 110 (72 %) |
| — plain questions left alone | 109 of 110 (99 %) | 107 of 110 (97 %) |
| Right class, exactly | 332 of 444 (75 %) | 261 of 444 (59 %) |
| Same answer every run | yes (5 runs) | not measured |
| Time per question (median / p95) | 0.08 ms / 0.13 ms | 5 ms / 8 ms |

| Class | Sluss recall / precision | Presidio recall / precision |
|---|---|---|
| Personal data | 0.79 / 0.82 | 0.97 / 0.44 |
| Keys and secrets | 0.56 / 0.75 | — |
| Security-classified | 0.67 / 1.00 | — |
| Health | 0.62 / 0.76 | — |
| Financial | 0.62 / 0.84 | — |
| Legal | 0.64 / 0.82 | — |
| Source code | 0.88 / 0.45 | — |

*Recall* is the share of questions with that class in which the tool found it. *Precision* is the share of the tool's findings of that class that were right.

## What the numbers say

- **Rules are strong where there is something to find.** Every term and marking, and 90 % of identifiers and keys, in well under a millisecond and the same way every time. Presidio has no notion of keys, security markings, health, finance, legal or code, which is most of what an organisation means by sensitive.
- **Rules do not read meaning.** 4 of 58 meaning-only questions. Presidio flags more of them, but only because it recognises a person's name: every one of its 32 hits there is a PERSON. The same habit makes it flag names in harmless questions (precision 0.44 on personal data), and every one of its 31 *term* hits is a name or a place in the question, sometimes a Swedish word taken for one, never the term itself.
- **Rules over-flag on words.** 18 % of harmless questions are flagged, nearly all look-alikes: *email* or *phone* in a question with no address or number in it, *password policy*, *patient*, *court cases*, and an invoice number written with `#`. That costs no protection; such a question is kept on your own models instead of the cloud. It is still a cost, and it is listed below to be fixed.
- **The second opinion is for the meaning column.** A model on your own hardware reading the question after the rules ([How Sluss decides](/manual/how-sluss-decides)) is what can move *meaning only* from 7 %. Measure it on your model with the commands below; the 49-example set on [What Sluss catches](/manual/what-sluss-catches) went from 32 to 38 of 41 with the demo's local model.

## Misses and false alarms found by the benchmark

These are listed as found, not fixed, so the numbers above are honest:

- `db_password=…` is not seen as a password (the underscore before the word).
- An address on a *torget* or another street word the rules do not know.
- One line of SQL without a code fence.
- The words *email*, *phone* and *password* in a harmless question give a class without an address, number or password.
- `#` followed by digits (an invoice number) is read as code.

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
