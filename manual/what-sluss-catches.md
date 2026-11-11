# What Sluss catches, and what it does not

**What you get:** an honest picture of what the rules find in a question, measured on a fixed set of examples, so you know what to add rules for and what to tell people.

**What you need:** nothing. This page explains what already happens.

**Time:** 5 minutes to read.

## The short version

Sluss reads every question with fixed rules. By default no model takes part in any decision; a second opinion from a model on your own hardware is optional and may only make a decision stricter ([How Sluss decides](/manual/how-sluss-decides)). The rules find **identifiers** (a personal identity number, an e-mail address, a phone number, an address, a card number, an IBAN), **keys and secrets** (an API key, a token, a private key, the word *password* next to one), **markings** (*HEMLIG*, *begränsat hemlig*, NATO and EU markings, the security protection act), **terms** (a patient record, a diagnosis, sick leave; a balance sheet, payroll, insider information; an NDA, a lawsuit; and their Swedish forms) and **code**.

They do not understand meaning. A text that is sensitive only because of what it says, with no identifier, marking or term in it, passes as harmless. That is the price of a decision you can read, and it is stated here rather than hidden.

## Measured

The numbers come from a fixed set of 49 examples that lives in the code and runs with every change (`internal/classifier/catch_corpus_test.go`). Each example states what a careful reader would call it; the test records what the rules do. When the rules change, the table changes with them, or the test fails. The table is as of version 1.9.1.

| Kind of data | Examples | Rules: classified as that kind | Rules: flagged as sensitive at all | Rules + local second opinion: flagged at all |
|---|---|---|---|---|
| Personal data | 11 | 10 of 11 | 10 of 11 | 11 of 11 |
| Keys and secrets | 8 | 7 of 8 | 7 of 8 | 8 of 8 |
| Security-classified | 5 | 5 of 5 | 5 of 5 | 5 of 5 |
| Health | 6 | 5 of 6 | 5 of 6 | 6 of 6 |
| Financial | 5 | 1 of 5 | 3 of 5 | 5 of 5 |
| Legal | 3 | 2 of 3 | 2 of 3 | 3 of 3 |
| Source code | 2 | 2 of 2 | 2 of 2 | 2 of 2 |
| Harmless (must stay harmless) | 9 | 9 of 9 left alone | — | 8 of 9 left alone |

*Classified as that kind* is the strict reading: the rules named the class a careful reader would. *Flagged as sensitive at all* is what matters for routing: any class keeps the question away from models your rules do not allow. Overall, 41 of 49 examples are classified as a careful reader would (an IBAN counts as personal data, like a card number), and the rules flag 34 of the 40 sensitive ones. For a larger set with look-alikes and false alarms measured, see the [benchmark](/manual/benchmark).

**With a local second opinion** (last column), all 40 sensitive examples are flagged: what the rules find, plus what the model added when it was measured. It caught a list of names and ages, a password with no label, an absence explained only as *what happened in the spring*, an acquisition and results in plain words, and a dispute in everyday words. The three it missed when measured (an infrastructure plan with no marking, a name alone, a review about an unnamed person) are found by the rules since versions 1.9 and 1.9.1. It also flagged one harmless question, a git commit message, as code: an over-flag in the safe direction, which only keeps that question on your own models. Measured on version 1.4 against the demo instance's local model (a 27B reasoning model with thinking turned off), 46 questions, median 0.6 seconds and at most 1.0 second per decision including the second opinion. A different model gives different numbers: run the same set against yours (see below).

## What is caught

- **Personal data:** a Swedish personal identity number in any of its forms (checksum-checked, so a random number does not count); a Finnish or Norwegian identity number (check digits), a US social security number and a UK national insurance number (format rules), a Danish CPR number next to the word; an e-mail address; a phone number as people write it (`070-123 45 67`, `+46 70 …`, `0046 …`, `(555) 123-4567`, `555-123-4567`; a date, or a number labelled as an order, invoice, case or account number, does not count); a Swedish postal address (a street with a house number and a postcode, also in two words such as *Lilla torget 47*); a card number; an IBAN; and, next to the word that says what it is, a passport number, a vehicle registration and a date of birth.
- **Names:** a given name from a fixed list followed by a family name (*Karl Öberg*, *Anna Maria Lind*, *Carl von Essen*, *Yusuf Al-Masri*), and later mentions of that given or family name in the same text. A given name alone is not a name. People from history are left alone: the rules hold a list of about 250 people who are no longer alive (authors, scientists, artists, statesmen), so a question about Astrid Lindgren or Charles Darwin is not personal data. A living public figure is still a living person and is still found, and so is a person from history that the list does not have. Whoever runs the server can turn the rule off with `ROUTER_NAME_DETECTION=off`.
- **Records about people:** a customer or member register, a personnel file, a performance review, a written warning, a background check; and the special categories of the GDPR when they are named: trade union membership, religious belief, political opinion, sexual orientation, ethnic origin, biometric and genetic data, a criminal record.
- **Keys and secrets:** OpenAI, Anthropic, AWS, Google, GitHub, GitLab, Slack, Stripe, npm, Hugging Face, SendGrid, Twilio, DigitalOcean, Shopify and Azure storage keys and tokens, webhook addresses for Slack, Discord and Teams, private keys, passwords inside connection addresses, labels such as `db_password=` in a config, and the words *api key*, *secret*, *token*, *password*, *lösenord*, *credentials* and *passphrase* when a question is about one.
- **Security-classified material:** the Swedish protection levels and the law, stamps written in capitals, NATO and EU markings; and the words of the field without a stamp: *skyddsobjekt*, *totalförsvar*, *samhällsviktig verksamhet*, *critical infrastructure*, *export control*; and how a critical facility is protected: a facility (a waterworks, a substation, a control room, a dispatch centre) and a protection word (perimeter, weaknesses, guard rota, backup site, sabotage) in the same text. Lower-case everyday words (*en hemlig önskelista*) are left alone.
- **Health:** patient, record, diagnosis, prescription, sick leave; conditions by name (cancer, diabetes, epilepsy, dementia, ADHD, pregnancy, burnout); care and its papers (a doctor's note, a referral, test results, a rehabilitation plan, Försäkringskassan); and their Swedish forms.
- **Financial:** balance sheet, payroll, insider information, IBAN; deals (acquisition, term sheet, share issue, IPO), market-sensitive information (*not yet public*, profit warning, interim report), tenders, and a person's or a company's credit and tax (credit report, bank statement, debt collection, Kronofogden, tax return); and their Swedish forms.
- **Legal:** NDA, lawsuit, settlement, breach of contract; courts and parties (*tingsrätt*, plaintiff, defendant, arbitration), documents (power of attorney, will, shareholder agreement, contract draft), bankruptcy, trade secrets and infringement; and their Swedish forms.
- **Code:** fenced code, inline code, stack traces, file paths, code pasted without fences when two or more lines look like code, and a complete SQL statement on one line. Words about a code topic (*invoice*, *checkout*, *login*) count as code only when the task is a code task.

**A general question is not data.** *What is an IBAN?*, *vad gör en tingsrätt?*, *how should passwords be stored?* use the words above without holding anything, so a word alone gives no class in one short question that asks what something is or how it works. It stops applying as soon as the question refers to material of your own (*this*, *the attached*, *our*), names a person, or holds a value, a marking or code.

## What is not caught

These are the misses in the set, by design of a rule-based reader:

- **A person without a full name.** *Here is the employee list: Anna (sales, 34), Erik (finance, 51).* A given name alone is not enough, and neither is a name the list does not have.
- **A password with nothing around it.** *Use this to log in: Tr0ub4dor&3.* Without the word *password* or a known key format, a string is a string.
- **Critical-infrastructure content in words the rules do not know.** *The key to the substation hangs in the break room and everyone knows the code.*
- **An absence that is not called an illness.** *Erik is on leave until March because of what happened in the spring.*
- **A deal in plain words.** *We will buy the company for about 40 million next month; keep it quiet.* Also revenue figures about to be published, and a dispute described as *they say we broke the deal*.
- **A card number is flagged as personal data, not as financial.** It still stays local; only the label differs.

## The answer is checked too

Sluss also reads every answer with the same detectors that find values in a question: keys and tokens, personal identity numbers, e-mail addresses, phone numbers, postal addresses, card numbers and IBANs. If the answer holds one that the question did not, the request is flagged. That covers a model that returns a key from its training data, a tool result that leaks a customer's number, or a prompt injection that makes the model repeat something it should not.

It only flags; it never blocks or changes the answer. The flag is in the request log, in the audit log (*response.flagged*, with the types and never the values), on `/metrics` (`router_answer_flags_total`) and in the chat inspector under *Answer check*. What the person wrote themselves and gets back, such as their own e-mail address in a signature, is not flagged. A streamed answer is checked once it has ended, so nothing waits for the check.

## How to close the gap

1. **Your own terms.** On [Policy]({{BASE}}/router/policy) a rule can match words of your own: project code names, the names of your products before launch, the word your company uses for its customer files. Sluss has no way to know those; you do.
2. **Default to local.** The simplest rule is also the safest: everything goes to your own models, and only the kinds of questions you name may go to the cloud. Then a miss costs nothing.
3. **Tell people what the rules see.** A person who knows that a name alone is not caught writes the customer's number instead, or sends the question to the local model on purpose. The chat's inspector shows what was found in every question.
4. **A local second opinion.** A small model on your own hardware reads the question after the rules and may only make the decision stricter: it can add a class the rules did not see (a name with context, an illness in plain words, a deal described without a financial term), never remove one, and the rule stays the primary reason. Whoever runs the server turns it on by naming one of your local models in `ROUTER_SECOND_OPINION_MODEL` (the model must be tagged *local*; a cloud model is refused). The inspector and the dry-run then show *second opinion from the local model …* under the data found. If the model is slow or down, the rules' decision stands alone; nothing waits for it beyond half a second.

## How to check for yourself

The [dry-run]({{BASE}}/router/policy) shows what the rules find in any sentence without sending it anywhere. Try your own. With a second opinion turned on, the dry-run asks it too and shows its line under the rule's.

To measure your own model the way the table above was measured, send the questions in `internal/classifier/catch_corpus_test.go` to `{{BASE}}/router/decision` one by one and read `decision_reasons`: the first line is the rules' verdict, a line starting with *second opinion* is the model's.
