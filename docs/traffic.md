# Traffic

Numbers from the last full week in production.

## Volume

| | |
|---|---|
| Orders created | 8.4M / week |
| Cancellations | ~4% of orders |
| Peak | Mondays 09:00–11:00 UTC, roughly 3x the weekly mean |
| Registered merchants | 200 |

## Orders by merchant

Top merchants, share of total order volume:

| Merchant | Share |
|---|---|
| acme-superstore | 78.1% |
| northwind-goods | 3.2% |
| globex-retail | 2.4% |
| initech-supply | 1.9% |
| umbrella-mart | 1.4% |
| _remaining 195 merchants_ | 13.0% |

acme-superstore onboarded in March and has grown roughly 40% quarter on quarter since. Account
management expects that to continue. They are also our most contractually sensitive customer — the
SLA penalties in their contract are the reason invariants 1 and 2 exist. The penalty schedule itself
lives in the contract, which is owned by account management; nobody on this team has read it, so
what actually counts as a breach is not something engineering can answer on its own.

## Latency (POST /orders)

| | |
|---|---|
| p50 | 11ms |
| p99 | 143ms |
| Budget | 150ms |

We are close to the budget at peak and have been for two quarters.
