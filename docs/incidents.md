# Incidents

## 2026-08-14 — Cancellations not reaching fulfilment

**Status: unresolved.**

Support escalated 40 orders that had been cancelled in the UI but were picked, packed and shipped
anyway. Customers were charged and we ate the return shipping.

All 40 showed `status = cancelled` in the database. Fulfilment had no record of receiving a
cancellation event for any of them. All 40 were acme-superstore. All were within the Monday morning
peak.

We could not reproduce it outside peak and the investigation stalled. The working theory at the
time was "something dropped them on the way to fulfilment" but nobody got further than that. We
added a manual reconciliation job that runs nightly and pages if the counts disagree; it has fired
twice since, both times during Monday peak, both times acme-superstore. Both times the on-call
engineer escalated to the team channel because there is no agreed procedure for what to do when it
fires, and no decision was ever recorded about whether replaying the missing events is safe.

Follow-up ticket was closed as "cannot reproduce".

## 2026-06-02 — Nightly CSV drop missed

Fulfilment's nightly import did not run. A day of orders went unfulfilled until it was noticed at
11:00 the next morning. Root cause was on the fulfilment side (expired credentials). Mentioned here
only because it is the reason the fulfilment team now want a real event stream.
