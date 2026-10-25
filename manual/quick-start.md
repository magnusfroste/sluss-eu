# Set up the models and the first rule

**What you get:** Sluss answers questions. Everyday questions go to a cheap cloud model. Anything with personal data stays on a model you run yourself.

**What you need:** the admin login for this instance, a key for one cloud AI service (OpenRouter is the simplest), and if you have one, the address of a model on your own hardware (vLLM, Ollama, LM Studio).

**Time:** about 10 minutes.

## Steps

1. Open [Quick start]({{BASE}}/router/setup) in the console. It shows four steps, each with a live status.
2. **A cloud model.** If the step says *missing*, the cloud key is not set. Whoever runs the server adds `OPENROUTER_API_KEY` to the environment and restarts. The step then turns green by itself.
3. **A local model.** Fill in the form: a name, the address of your own model server, and the model name it serves. Press save. Sluss tags it as *local*, which is what the rules look for.
4. **Keep personal data local.** Press the button. It adds one rule: personal data and secrets may only go to local models. If no local model is reachable, the request is refused rather than sent to the cloud.
5. **See it work.** Press *Run the dry-run*. The first example, with a Swedish personal identity number, should say *local*. The second, a git commit message, should say *cloud*.

## How to check

The progress bar on Quick start says *4 of 4 done*, and the yellow banner on the dashboard disappears.

## If you are unsure

The dry-run sends nothing to any model. Try your own sentences there as many times as you like.
