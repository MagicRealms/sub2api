# Tester role and personal monitoring

Administrators can select **Tester** when creating or editing a user. Registration
still creates ordinary users. Testers retain the normal user gateway, balance,
subscription, key quota and group routing rules; this role grants no admin APIs,
supplier account details, free usage, or billing reset permission.

## Entry points

- Tester sidebar: Personal Monitoring (`/tester/monitor`).
- Tester usage records: Clear my usage records and a monitoring shortcut.
- Authenticated API: `GET /api/v1/tester/monitor` and
  `POST /api/v1/tester/usage/reset`.

Both API handlers require the live JWT subject to have the tester role. User IDs
are taken only from the authenticated subject. A key filter is checked for
ownership by the existing usage filter parser. Current admin-only middleware
continues to require exactly `admin`.

## Clearing and accounting

Migration 253 adds one optional `tester_usage_history` row per user. Clearing
atomically advances `visible_after`; it never deletes or edits usage logs, error
logs, billing deduplication records, balances, key counters, subscriptions or
rate limits. The database write additionally rechecks the active tester role.
Concurrent clears only move the boundary forward.

The boundary applies to the tester's record lists, record details, associated
usage statistics and explicitly marked test-session charts. Financial dashboard
totals and administrator audit/billing history retain their original meaning.
In-flight requests recorded after the clear may appear in the new session.
The boundary is retained if an administrator later changes the role; ordinary
users see their usual full history, and a later tester assignment resumes the
existing boundary.

## Measurements and performance

- Periods use Beijing calendar days: today, last seven days, last thirty days,
  or the current observation session (at most thirty days).
- One aggregate query scopes by user and a maximum 31-day range, optionally
  key, model and group. Model/group combinations are kept separate and capped
  at 200 displayed rows; a truncation flag is returned.
- Cache hit rate is token-weighted cache read / (ordinary input + cache write +
  cache read), matching the existing ops metric. It is not a request hit rate.
- TTFT and duration report valid-sample P50/P95; missing timings are null.
- Estimated output speed uses valid streaming output tokens divided by summed
  time after the first token, with a separate valid sample count.
- Actual charges and standard costs come from the existing usage ledger.
  Charge per million total tokens is an observed blended cost, not list price.
- Final customer-visible failure records are counted separately. Recovered
  upstream attempts and count-token requests are excluded; a metered record
  alone is not proof of a fully successful response.
- Reads have a 10-second context deadline and the existing panel heavy/global
  user rate limits. The screen loads on entry/filter/manual refresh without
  polling. Requests are cancelled and stale responses ignored when navigating
  or changing filters. No new work runs in the hot gateway request path.

## Rollback

This migration is additive. The preceding fork image can run with the new table
present and treats `tester` as a non-admin role. The tester screen will be absent
on that image; the production ledger remains authoritative. Do not drop the
table or delete usage records as part of an application rollback.
