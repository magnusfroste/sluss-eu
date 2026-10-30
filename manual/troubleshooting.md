# When something does not work

## Quick start says "missing" for the cloud model

The cloud key is not in the server's environment. Whoever runs the server adds `OPENROUTER_API_KEY` and restarts. In EasyPanel, it goes under the service's *Environment*.

## A question was refused with "no provider satisfies the required data-residency/compliance tags"

That is a rule working as intended. The question contained data that may only go to a local model, and no local model was available. Check on [Models]({{BASE}}/router/models) that your local model is enabled and tagged *local*. Then try the [dry-run]({{BASE}}/router/policy) to see which rule matched.

## A model stopped answering, or the chat says "model unavailable"

Often the provider renamed or swapped the model, which happens a lot on your own lab hardware. Open [Models]({{BASE}}/router/models) and click the model. In the panel, press *Test now*. If the provider no longer knows the name, the panel lists the models it serves now. Press *Use this* on the right one. The change applies at once, and the panel also shows the model's failures from the last 24 hours.

## The "Sign in with …" button does not show

The sign-in settings are not in the server's environment, or the server has not been restarted since they were added. All three values must be set: the issuer, the client ID and the client secret.

## Signing in fails with a redirect error

The address registered with the identity provider must be exactly `{{BASE}}/router/oidc/callback`, with no extra slash at the end.

## Someone signs in but lands in the wrong department, or is not an admin

Microsoft Entra ID sends groups as long ID numbers, not names. Copy the group's *Object ID* from Entra, not its display name.

## chatgpt.com still opens on a company computer

- The browser may be going around the filter. Deploy the browser policy from Rollout, which turns off DNS-over-HTTPS.
- The computer may not be using the company filter, for example on a home network without the filter's client. Check that the filter's app is installed and connected.
- Run the coverage script on that computer. If it says *open*, the filter does not apply there yet.

## The chat says "search unavailable"

Sluss could not get results from the search service. Either the search service is not running, the address in `ROUTER_SEARCH_URL` is wrong, or the search service has JSON output turned off. The ready configuration in `deploy/searxng/settings.yml` turns it on.

## The coverage script reports "Sluss unreachable"

The computer cannot reach `{{BASE}}`. Check the network from that computer, and that the department key in the script is valid.

## The console footer shows an old version after a deploy

The deploy did not land. In EasyPanel, press *Stop*, then *Deploy*. The footer shows the version that is actually running.
