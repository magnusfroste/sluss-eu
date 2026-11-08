# What Sluss catches, and what it does not

**What you get:** an honest picture of what the rules find in a question, measured on a fixed set of examples, so you know what to add rules for and what to tell people.

**What you need:** nothing. This page explains what already happens.

**Time:** 5 minutes to read.

## The short version

Sluss reads every question with fixed rules, never with another AI. The rules find **identifiers** (a personal identity number, an e-mail address, a phone number, an address, a card number, an IBAN), **keys and secrets** (an API key, a token, a private key, the word *password* next to one), **markings** (*HEMLIG*, *begränsat hemlig*, NATO and EU markings, the security protection act), **terms** (a patient record, a diagnosis, sick leave; a balance sheet, payroll, insider information; an NDA, a lawsuit; and their Swedish forms) and **code**.

They do not understand meaning. A text that is sensitive only because of what it says, with no identifier, marking or term in it, passes as harmless. That is the price of a decision you can read, and it is stated here rather than hidden.

## Measured

The numbers come from a fixed set of 46 examples that lives in the code and runs with every change (`internal/classifier/catch_corpus_test.go`). Each example states what a careful reader would call it; the test records what the rules do. When the rules change, the table changes with them, or the test fails. The table is as of version 1.3.

| Kind of data | Examples | Rules: classified as that kind | Rules: flagged as sensitive at all | Rules + local second opinion: flagged at all |
|---|---|---|---|---|
| Personal data | 9 | 6 of 9 | 6 of 9 | 7 of 9 |
| Keys and secrets | 8 | 7 of 8 | 7 of 8 | 8 of 8 |
| Security-classified | 5 | 4 of 5 | 4 of 5 | 4 of 5 |
| Health | 6 | 5 of 6 | 5 of 6 | 6 of 6 |
| Financial | 5 | 2 of 5 | 3 of 5 | 5 of 5 |
| Legal | 3 | 2 of 3 | 2 of 3 | 3 of 3 |
| Source code | 3 | 3 of 3 | 3 of 3 | 3 of 3 |
| Harmless (must stay harmless) | 7 | 7 of 7 left alone | — | 6 of 7 left alone |

*Classified as that kind* is the strict reading: the rules named the class a careful reader would. *Flagged as sensitive at all* is what matters for routing: any class keeps the question away from models your rules do not allow. Overall, 36 of 46 examples are classified as a careful reader would, and the rules flag 30 of the 39 sensitive ones.

**With a local second opinion** (last column), 36 of the 39 sensitive examples are flagged. It caught a list of names and ages, a password with no label, an absence explained only as *what happened in the spring*, an acquisition and results in plain words, and a dispute in everyday words. It still missed a name alone, a review about an unnamed person, and an infrastructure plan with no marking. It also flagged one harmless question, a git commit message, as code: an over-flag in the safe direction, which only keeps that question on your own models. Measured on version 1.4 against the demo instance's local model (a 27B reasoning model with thinking turned off), 46 questions, median 0.6 seconds and at most 1.0 second per decision including the second opinion. A different model gives different numbers: run the same set against yours (see below).

## What is caught

- **Personal data:** a Swedish personal identity number in any of its forms (checksum-checked, so a random number does not count), an e-mail address, a phone number as people write it (`070-123 45 67`, `+46 70 …`), a Swedish postal address (a street with a house number and a postcode), a card number, an IBAN.
- **Keys and secrets:** OpenAI, AWS, GitHub and Slack keys and tokens, private keys, and the words *api key*, *secret*, *token* and *password* when a question is about one.
- **Security-classified material:** the Swedish protection levels and the law, stamps written in capitals, NATO and EU markings. Lower-case everyday words (*en hemlig önskelista*) are left alone.
- **Health:** patient, record, diagnosis, prescription, sick leave, burnout, chemotherapy and their Swedish forms.
- **Financial and legal:** the terms a finance or legal department uses: balance sheet, payroll, insider information, IBAN; NDA, lawsuit, settlement, breach of contract; and their Swedish forms.
- **Code:** fenced code, inline code, stack traces, file paths, and code pasted without fences when two or more lines look like code.

## What is not caught

These are the misses in the set, by design of a rule-based reader:

- **A person without an identifier.** *Write a letter to our customer Anna Svensson about the late delivery.* A name is not enough; neither is a list of names and ages, nor a performance review that never names its subject.
- **A password with nothing around it.** *Use this to log in: Tr0ub4dor&3.* Without the word *password* or a known key format, a string is a string.
- **Critical-infrastructure content with no marking.** A plan for protecting the grid's control systems, written without a stamp.
- **An absence that is not called an illness.** *Erik is on leave until March because of what happened in the spring.*
- **A deal in plain words.** *We will buy the company for about 40 million next month; keep it quiet.* Also revenue figures about to be published, and a dispute described as *they say we broke the deal*.
- **A card number is flagged as personal data, not as financial.** It still stays local; only the label differs.

## How to close the gap

1. **Your own terms.** On [Policy]({{BASE}}/router/policy) a rule can match words of your own: project code names, the names of your products before launch, the word your company uses for its customer files. Sluss has no way to know those; you do.
2. **Default to local.** The simplest rule is also the safest: everything goes to your own models, and only the kinds of questions you name may go to the cloud. Then a miss costs nothing.
3. **Tell people what the rules see.** A person who knows that a name alone is not caught writes the customer's number instead, or sends the question to the local model on purpose. The chat's inspector shows what was found in every question.
4. **A local second opinion.** A small model on your own hardware reads the question after the rules and may only make the decision stricter: it can add a class the rules did not see (a name with context, an illness in plain words, a deal described without a financial term), never remove one, and the rule stays the primary reason. Whoever runs the server turns it on by naming one of your local models in `ROUTER_SECOND_OPINION_MODEL` (the model must be tagged *local*; a cloud model is refused). The inspector and the dry-run then show *second opinion from the local model …* under the data found. If the model is slow or down, the rules' decision stands alone; nothing waits for it beyond half a second.

## How to check for yourself

The [dry-run]({{BASE}}/router/policy) shows what the rules find in any sentence without sending it anywhere. Try your own. With a second opinion turned on, the dry-run asks it too and shows its line under the rule's.

To measure your own model the way the table above was measured, send the questions in `internal/classifier/catch_corpus_test.go` to `{{BASE}}/router/decision` one by one and read `decision_reasons`: the first line is the rules' verdict, a line starting with *second opinion* is the model's.
