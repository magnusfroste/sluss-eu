# SLO och SLA

## Interna SLO för MVP

| SLO | Mål |
|---|---:|
| Router availability | 99.5 % |
| Fast path overhead p95 | < 100 ms |
| Fast path overhead p99 | < 250 ms |
| Router error rate | < 0.1 % |
| Decision log coverage | > 90 % |
| Provider fallback success | > 90 % |

## Exkludera från router-SLO

- Provider total latency.
- Provider modellkvalitet.
- Klientens nätverk.

## Error budget

Följ upp:

- Router 5xx.
- Authfel som beror på router.
- Felaktig policyaktivering.
- Fallbackfailures.

## SLA senare

Extern SLA bör inte erbjudas förrän:

- Multi-region eller robust failover finns.
- Providerberoenden är tydligt avgränsade.
- Incidentprocessen är testad.
