# When something does not work

## Quick start says "missing" for the cloud model

The cloud key is not in the server's environment. Whoever runs the server adds `OPENROUTER_API_KEY` and restarts. In EasyPanel, it goes under the service's *Environment*. If every model is to run on your own hardware, skip this step: add the local model in step 2 instead.

## Every answer says "no real model is connected yet"

That is Sluss's built-in test answer. No model is connected: there is no cloud key, and no model has been added on [Models]({{BASE}}/router/models). Do either, and the test answer stops. If a local model *is* added, open it and press *Test now* to see why it does not answer.

## The login page says "Too many attempts"

Five wrong passwords for an account within fifteen minutes engage a brake: further attempts are refused for a wait that doubles each time, up to fifteen minutes. Wait it out, or ask another administrator to check [Console users]({{BASE}}/router/users). Every wrong password is in the audit log with the address it came from, so a brake that nobody in the company caused is worth a look.

## I am locked out, or forgot my password

- Another administrator can set a new password for you under [Console users]({{BASE}}/router/users).
- Whoever runs the server can sign in with the break-glass password (`ROUTER_DASHBOARD_PASSWORD` in the server's environment, any username) and do the same. The audit log marks that session as *break-glass*.
- `ROUTER_ADMIN_USER` and `ROUTER_ADMIN_PASSWORD` only create the first account while none exists. Changing them later does nothing: change the password on [Your profile]({{BASE}}/me) instead.

## A question was refused with "no provider satisfies the required data-residency/compliance tags"

That is a rule working as intended. The question contained data that may only go to a local model, and no local model was available. Check on [Models]({{BASE}}/router/models) that your local model is enabled and tagged *local*. Then try the [dry-run]({{BASE}}/router/policy) to see which rule matched.

## The local model answers "connection refused", or Test now fails although the server is running

Sluss runs in a container, and the model server runs on the same machine. Inside the container, `localhost` is the container itself, so `http://localhost:11434` reaches nothing. Open [Providers]({{BASE}}/router/providers), click the connection and change the address to `http://host.docker.internal:11434`. Two things on the server side: the container must have been started with `--add-host=host.docker.internal:host-gateway` (the self-host guide's run command has it; the Compose file has it too), and the model server must listen on all interfaces, not only on `127.0.0.1` (Ollama: start it with `OLLAMA_HOST=0.0.0.0`).

## A model stopped answering, or the chat says "model unavailable"

Often the provider renamed or swapped the model, which happens a lot on your own lab hardware. Open [Models]({{BASE}}/router/models) and click the model. In the panel, press *Test now*. If the provider no longer knows the name, the panel lists the models it serves now. Press *Use this* on the right one. The change applies at once, and the panel also shows the model's failures from the last 24 hours.

## The "Sign in with …" button does not show

The sign-in settings are not in the server's environment, or the server has not been restarted since they were added. Two values are needed: the issuer and the client ID (the client secret only if your identity provider requires one). Sign-in also needs a data directory (`ROUTER_DATA_DIR`), which every container install has.

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
