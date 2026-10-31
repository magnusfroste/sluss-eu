# Works with your stack

**What you get:** a clear picture of where Sluss sits next to the tools your company already has, and what each one does. Sluss does not replace your web filter, your existing AI tools or your sign-in. It works between them.

**What you need:** nothing new. Each section says which page in Sluss connects the tool.

**Time:** 5 minutes to read.

## The picture

```
 People and tools          Web filter              Sluss                     AI models
 (browsers, chat,   →   (Zscaler, Netskope,  →   reads every question,  →   cloud models, your own
  coding tools,          Defender, Cloudflare,    applies your rules,        hardware, or an AI proxy
  AI platforms)          Umbrella)                picks the model,           you already run
                         blocks chatgpt.com        keeps the evidence
                         and sends people here
```

- The **web filter** decides which *websites* a company computer may open.
- **Sluss** decides where each *question* may go, picks the model, and records the decision.
- The **models** answer.

## In front of Sluss: your web filter

Tools like Zscaler, Netskope, Microsoft Defender, Cloudflare or Cisco Umbrella guard the door. They can block chatgpt.com and similar sites, and some can inspect or mask what is typed into them. They do not give people a safe place to ask instead, and they do not choose a model.

Together: the filter blocks the public AI sites and sends people to the Sluss block page, which points to the company chat. Every question then passes Sluss.

**How to connect:** [Stop ChatGPT, offer the company chat](/manual/stop-chatgpt) has the deny list, the block page and the steps for each filter.

## Behind Sluss: an AI proxy you already run

Some companies already run an AI proxy such as LiteLLM or Portkey to reach several models through one address. Keep it. Add it in Sluss as a provider: on [Providers]({{BASE}}/router/providers), give its address and the name of the environment variable that holds its key.

Two things to get right:

- **Let Sluss do the choosing.** Add each model the proxy serves as its own model in Sluss, with its exact name. If the proxy picks the model by itself, Sluss can no longer say which model answered, and its rules cannot tell local from cloud.
- **Tag honestly.** Tag the proxy *local* only if every model behind it runs on hardware the company controls. If it mixes cloud and local models, add it twice: once for the local models (tagged *local*) and once for the cloud ones.

## Beside Sluss: chat apps and AI platforms

Chat front ends and AI platforms that let you set an OpenAI-compatible address can send their questions through Sluss. That includes Open WebUI, AnythingLLM, LibreChat and many in-house tools. They keep their own look and features, and Sluss adds the rules, the choice of model and the evidence.

**How to connect:** [Connect clients]({{BASE}}/router/connect) has the address, the model name `auto` and a recipe per tool. Give each tool its own department key, so the request log shows which tool asked.

## Around Sluss: sign-in and monitoring

- **Sign-in.** Microsoft Entra ID, Okta, Google or Keycloak, over OpenID Connect. Sluss keeps no employee accounts. See [Start here](/manual).
- **Monitoring.** Sluss publishes its numbers on `{{BASE}}/metrics` for Prometheus or Grafana. The audit log is a file in the data volume that your log tools can collect, and that an auditor can check offline.

## What Sluss does not replace

- **A web filter.** Sluss does not stop people from opening websites. The filter does that.
- **Data loss prevention for e-mail and files.** Sluss reads questions to AI models, not e-mail or file shares.
- **An endpoint agent.** Sluss runs as one service. It does not install anything on computers. The scripts on [Rollout]({{BASE}}/router/rollout) only point tools at Sluss and check whether AI sites can still be reached.

## In one sentence

The web filter guards the door, Sluss decides where each question may go and proves it, and the models answer.
