# Onboarding för ny tenant

## Steg

1. Skapa tenant.
2. Skapa projekt.
3. Skapa API key.
4. Välj default policy.
5. Sätt provider allowlist.
6. Sätt budget.
7. Välj prompt logging setting.
8. Kör testrequest.
9. Visa dashboard.

## Kundinstruktion

Byt base URL:

```bash
export OPENAI_BASE_URL="https://router.example.com/v1"
export OPENAI_API_KEY="router_key"
```

Använd `model: auto`.

## Första verifiering

- Trivial prompt routeas cheap.
- Riskprompt routeas premium.
- Provider fallback testas via mock/degraded mode.
- Spend syns i dashboard.
