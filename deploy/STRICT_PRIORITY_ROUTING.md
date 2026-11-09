# Strict account priority for OpenAI text groups

Enable selected groups in `gateway.openai_scheduler.strict_priority_group_ids`
and restart the application. The default empty list preserves existing routing.
This does not require a database migration, group changes or new API keys.

```yaml
gateway:
  openai_scheduler:
    strict_priority_group_ids: [6, 10]  # replace with this installation's group IDs
```

Within an enabled group, a lower numeric **account priority** wins absolutely.
Eligible accounts at priority 1 are considered before 2, then 3. Load, measured
latency, subscription preference, cost scores and soft session affinity cannot
promote a different priority. Equal-priority accounts retain existing scoring.
After a safe failover excludes an account, the next eligible priority is used.
Ordinary HTTP text requests skip pool-mode same-account retries on failover-
eligible errors; existing replay/semantic-output/error-classification guards
still decide whether switching is safe. Parameter and policy errors do not gain
new failover permission. Established WebSocket turn retries retain their policy.

A healthy but full preferred tier uses the existing bounded account queue. A
full queue or expired wait returns busy; spare capacity on higher-priority-number
backups is intentionally unused. Account concurrency, waiting limits, model and
capability eligibility, cooldowns, privacy checks and profit checks still apply.
A primary that does not support the requested model is not an eligible primary.

Safe new requests return to a recovered primary. A nonmovable previous response,
guardian parent or task owner retains its account to protect upstream state;
these are exceptions to ordinary priority selection, not load balancing. No
generated response is replayed after semantic output. Independent image/media
paths and other providers are excluded. Group billing and quota code is unchanged.
Admin priority changes are checked against the database before selection; a
conflicting stale priority snapshot fails closed until the snapshot catches up.

Rollback: remove the group IDs (or use `[]`) and restart, or restore the preceding
pinned fork image. Keep existing account priorities and billing multipliers.
