# How Sluss picks a model

**What you get:** you can explain why a question went to a certain model, and you know the two settings that decide it.

**What you need:** nothing to install. This page explains what already happens.

**Time:** 5 minutes to read.

## The short version

Sluss picks a model in three steps, every time, in the same order:

1. **Remove what is not allowed.** Your rules come first. Then every model that is too weak for the kind of question is removed, and so is any model whose server is down.
2. **Pick the best of what is left.** Among the allowed models, Sluss takes the best one for the price.
3. **Or stop.** If no model is left, the question is refused and recorded. It is never sent somewhere else to get an answer.

Step 1 is the floor Sluss protects. Step 2 is where it saves money. Nothing in step 2 can undo step 1.

## Step 1: what is not allowed

**Your rules.** A rule such as *personal data → only models tagged local* removes every model without the tag *local* for that question. A *block* rule stops the question. Rules are on [Policy]({{BASE}}/router/policy).

**The level the question needs.** Every model has a level, and every kind of question needs at least a certain level:

| Level | Meant for | Questions that need at least this level |
|---|---|---|
| **cheap** | Quick, simple work | Everyday questions, short texts, summaries, commit messages |
| **balanced** | Ordinary work | Hard debugging, long documents, and any question Sluss rates as high risk |
| **premium** | The hardest work, where a wrong answer costs the most | Security reviews, database changes, and unclear questions that look risky |

A question may always go to a model *above* the level it needs. A premium model can answer a simple question. It never goes the other way: a security review never goes to a cheap model.

**The level is your judgement, not a fact about the model.** You set it on [Models]({{BASE}}/router/models). It says: *I trust this model with this kind of work.* It does not depend on where the model runs or how big the hardware is. A model on your own servers can be premium if it does premium work well. Bigger hardware makes a model faster, not smarter.

**Models whose server is down** are skipped until they answer again.

## Step 2: the best of what is left

Among the allowed models, Sluss weighs mostly two things:

- **How good the model is.** By default this follows its level. If you have measured results for a kind of question, those count instead.
- **What it costs.** A cheaper model scores higher.

It also prefers a model whose level fits the question, and one that answers quickly.

**What this means in practice:** a model on your own hardware costs nothing per question, so it wins almost every time it is allowed to answer.

| Your local model is set to | Simple questions | Hard debugging | Security reviews and unclear risky questions |
|---|---|---|---|
| balanced | local | local | the premium cloud model |
| premium | local | local | local |

Setting the local model to premium sends **everything** to it. That is good for privacy and cost. Make sure it is good enough for security reviews, and that your hardware can take all the traffic.

## Step 3: when nothing is left

The question is refused, the person sees why, and the refusal is recorded. The most common cause: a rule says *local only*, but the local model's level is below what the question needs. The [dry-run]({{BASE}}/router/policy) names every model that was skipped and why, for example *tier balanced below minimum premium*.

## The two settings you have

1. **The level of each model**, on [Models]({{BASE}}/router/models). This decides which questions a model is trusted with.
2. **Your rules**, on [Policy]({{BASE}}/router/policy). These decide where each kind of data may go, or stop it.

That is all. Sending a certain kind of question to one particular model (for example public research to one specific cloud model) is not a console setting yet.

## How to see why

- In the chat, click the short line under an answer. The **Route** card in the inspector shows the kind of question, the data found, the rules that matched, and the model that answered.
- On [Policy]({{BASE}}/router/policy), the **dry-run** shows the same for any sentence, without sending it to any model.

## A simple starting point

- One **cheap** cloud model for everyday questions.
- One **local** model on your own hardware, set to **balanced**, tagged *local*.
- One **premium** cloud model for the hardest questions that contain no sensitive data.
- One rule: personal data and secrets → only models tagged *local*.

Then try your own questions in the dry-run. When the local model proves good enough for the hardest work, raise it to premium.
