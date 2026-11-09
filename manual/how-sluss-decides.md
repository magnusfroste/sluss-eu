# How Sluss decides: rules only, or rules plus a second opinion

**What you get:** which of the two ways Sluss can decide fits your organisation, and how to show which one was in use.

**What you need:** nothing for *rules only*, which is how Sluss starts. For a second opinion, a small model on your own hardware.

**Time:** 5 minutes to read.

## Two modes

| | **Rules only** (how Sluss starts) | **Rules plus a local second opinion** |
|---|---|---|
| Who decides | Fixed rules you can read. No model takes part in any decision. | The same rules, then a model on your own hardware that may add a kind of data the rules did not see. |
| What it catches | Identifiers, keys, markings, terms and code. See [What Sluss catches](/manual/what-sluss-catches). | The same, plus meaning without identifiers: a name with a story, an illness in plain words, a deal described without a financial term. |
| Same question, same decision | Always. | The rules' part, always. The model's addition can vary from one time to the next. |
| What the model may do | Nothing: there is none. | Only make a decision stricter. It can add a kind of data and raise the risk; it can never remove one, lower a level, or overrule a rule. |
| Where the question goes to be classified | Nowhere: Sluss reads it in memory. | To the model on your own hardware, which must be tagged *local*. A cloud model is refused. |
| If the model is slow or down | — | The rules decide alone after half a second (`ROUTER_SECOND_OPINION_TIMEOUT`). |
| Turned on by | — | Whoever runs the server, with `ROUTER_SECOND_OPINION_MODEL` naming one of your local models. |

## Which one to choose

**Choose rules only** if your organisation does not want any AI involved in deciding where data may go, if every decision must be repeatable exactly, or if you do not have a local model. It is the default, and it is a complete product: add your own terms as rules, and keep sensitive kinds of question on your own hardware.

**Add the second opinion** if the misses on [What Sluss catches](/manual/what-sluss-catches) matter to you, and you have a model on your own hardware that answers quickly. Measured on the demo's local model, it raised the share of sensitive examples caught from 32 to 38 of 41, and flagged one harmless question that did not need it.

Either way, the rules are the floor. Nothing the model says can make a decision less strict than the rules made it.

## How to show which mode was in use

A security officer or an auditor will ask. There are four places, and they agree:

- **The log at start.** The first lines Sluss writes say `Decisions  rules only — no model takes part in any decision`, or name the second-opinion model.
- **[Version]({{BASE}}/router/system)**, in the console, under *How decisions are made*.
- **`{{BASE}}/version`**, for a script: `"decisions": "rules-only"` or `"rules+second-opinion"`.
- **The audit log.** Every start writes a `router.started` record with the mode. Because the audit log is tamper-evident, this shows for any period how decisions were made: from one start record to the next, the mode cannot have changed.

With a second opinion on, each decision it touched also says so: the dry-run and the chat inspector show a line *second opinion from the local model …*, after the rule's own reason.

## What never involves a model

In both modes, these are rules and nothing else: blocking, keeping a question on your own hardware, *mask and send*, checking the answer, the audit log and everything in it. The second opinion can only add a kind of data to what the rules found, before those rules are applied.
