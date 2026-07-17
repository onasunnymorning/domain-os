# ADR 0005 — FX rates source: Frankfurter, daily, derived base currencies

- **Status:** Accepted
- **Date:** 2026-07-16
- **Deciders:** Platform / Registry
- **Supersedes:** none (complements ADR 0004)

## Context

Exchange rates back domain pricing: `DomainService.GetQuote` converts from a
phase's base currency into the requested transaction currency via the `fx`
table. The rates were refreshed hourly by the `update-fx` workflow, which
looped over a hardcoded list of seven currencies and called
`PUT /sync/fx/{cur}` per currency against the admin API, which in turn called
openexchangerates.org.

A sanity check of that chain found it effectively broken:

1. **The free openexchangerates plan only supports `base=USD`** — the other
   six bases (EUR, PEN, GBP, RUB, CAD, AUD) failed on every run. The OpenFX
   client never checked HTTP status codes, so the error bodies decoded to
   empty rate maps.
2. **`RefreshFXRates` dereferenced a nil response on API failure** (the error
   was printed to stdout and execution continued) — a panic in the API
   handler.
3. **The workflow swallowed all activity errors** and returned success, so
   scheduled runs always showed green regardless of outcome.
4. **`FXRepository.UpdateAll` was not transactional** (delete then insert as
   separate statements, insert ignoring `ctx`): a crash in between left a base
   currency with no rates at all, failing every non-base-currency quote with
   `ErrMissingFXRate` until the next run.
5. **`GetByBaseAndTargetCurrency` returned the oldest rate** — `First()`
   orders by primary key, which begins with `date` ascending.
6. It required the `OPENEXCHANGERATES_APP_ID` secret across the deploy
   surface (helm, compose, contract) for data that is freely available.

Separately, hourly refresh was pointless: reference rates from central banks
update on a daily cadence.

## Decision

1. **Source rates from the Frankfurter API v2** (`https://api.frankfurter.dev/v2/rates`,
   https://frankfurter.dev/): open, no API key, no quotas, aggregates
   reference rates from 84 central banks, ~165 quote currencies per base —
   including every currency previously configured (notably PEN and RUB, which
   the ECB-only sources lack). A dedicated client lives in
   `internal/infrastructure/api/frankfurter` with status-code checking,
   request timeouts, and context support. The `openfx` package and the
   `OPENEXCHANGERATES_APP_ID` secret are deleted (env registry, deploy
   contract, helm, compose).
2. **One direct-DB activity replaces the per-currency HTTP loop.**
   `FXActivities.UpdateFXRates` (same pattern as `LifecycleActivities`, per
   ADR 0004) fetches and stores each base's rates itself. The workflow makes
   a single activity call and returns a structured `UpdateFXResult`; it fails
   only when *no* base could be updated, and reports per-base failures
   otherwise. The old HTTP activity stays registered for drain queues only.
3. **Base currencies are derived, not hardcoded.** The activity queries the
   distinct `phases.base_currency` values (fallback `USD`) — exactly the set
   quoting converts from. New phase currencies need no code change. Callers
   may override via `UpdateFXParams.BaseCurrencies`.
4. **Storage is correct under failure.** `UpdateAll` runs delete + insert in
   one transaction; `GetByBaseAndTargetCurrency` orders by `date DESC` so the
   newest rate always wins. The manual `PUT /sync/fx/{currency}` endpoint
   keeps working, now Frankfurter-backed with proper error propagation.
5. **Schedule: daily at 18:00 UTC** (24h interval, 18h offset — after the ECB
   ~14:15 UTC reference publication), catchup window 24h. The bootstrap
   reconciler applies the change automatically on deploy.

## Consequences

**Positive**

- Rates for *all* configured base currencies actually update — previously
  only USD could ever have succeeded.
- FX failures are visible (red runs, structured failure lists) instead of
  silently green.
- No paid API dependency, no secret to provision or rotate.
- A crash can no longer wipe a base currency's rates or serve the oldest rate.

**Negative / accepted trade-offs**

- Frankfurter is a free community service without an SLA. Mitigations: rates
  are only refreshed daily and stale rates remain usable; a failed run keeps
  previous rates; Frankfurter supports self-hosting (Docker) should we ever
  need it, and the `FXRatesSource` interface keeps the provider swappable.
- Daily granularity means intraday FX moves are not reflected in quotes —
  acceptable for registry pricing, and no worse than the reference-rate
  cadence of the previous source.
- The workflow signature changed (`func(ctx) error` →
  `func(ctx, UpdateFXParams) (UpdateFXResult, error)`); in-flight runs at
  deploy will fail replay and be retried by the next schedule fire (runs are
  seconds long, daily — negligible).
