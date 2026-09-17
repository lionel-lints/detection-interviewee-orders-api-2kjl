# orders-api

Order ingestion for the marketplace. Accepts orders from merchants, stores them, and tells the
fulfilment service what to do.

## Running it

```sh
go mod download    # do this before you start, not on the clock
make test          # unit tests
make test-race     # unit tests under the race detector
make load          # replay production-shaped traffic and print the numbers
make load-race     # the same, under the race detector
make run           # start the service on :8080
```

Everything runs locally. No Docker, no external services, no network at runtime. The database is
SQLite standing in for the Postgres we run in production.

## What it does today

| Endpoint | |
|---|---|
| `POST /orders` | create an order |
| `GET /orders/{id}` | fetch one |
| `POST /orders/{id}/cancel` | cancel an order, and tell fulfilment about it |

Cancellations are published to fulfilment over `internal/broker`, our partitioned event log (the
local implementation stands in for Kinesis). Order *creations* are not published to anything yet —
fulfilment currently finds out about new orders by a nightly CSV drop.

## The work

`PHASE_1.md` describes phase 1. There is a phase 2 in `PHASE_2.md` — leave it sealed until phase 1 is
done.

## Invariants

These are the guarantees the team has committed to. They are what we get paged for.

1. **Fulfilment must never miss an order.** If we accept an order, fulfilment learns about it.
   Exactly how quickly is negotiable; whether it happens at all is not.
2. **Order-to-fulfilment p99 under 2s, and event lag must not grow unboundedly.** A backlog that
   grows faster than it drains is an outage, even if every individual event eventually lands.
3. **`POST /orders` p99 under 150ms, regardless of downstream health.** Merchants integrate against
   this endpoint synchronously. It is enforced by a request timeout in `internal/api`.
4. **Fulfilment assumes per-merchant ordering.** Worth knowing: this assumption is inherited from
   the original integration and has never been confirmed with the fulfilment team.

## Things worth knowing

- `internal/broker`'s partition count is fixed at provisioning time. Changing it means a ticket to
  the platform team; historically that has taken a couple of weeks, and nobody here has done it
  before.
- The nightly reconciliation job compares our order count against fulfilment's and pages if they
  disagree. What on-call is supposed to *do* when it fires has never been written down.

## Operational context

- `docs/traffic.md` — what our traffic actually looks like
- `docs/incidents.md` — recent incidents
