# Proxy transport recovery and temporary direct routing

A durable transport error (proxy authentication, refused connection, DNS or
routing failure) pauses an OpenAI account for ten minutes. Previously a healthy
proxy did not release this cooldown; operators had to clear each account or
wait for expiry.

Observed incident: 2026-09-24 12:52:49 UTC+08, accounts 1336 and 1338
received SOCKS5 connection-refused errors and were paused until 13:02:49.

The recovery worker polls the authoritative database every 15 seconds, including
accounts omitted from scheduler snapshots. Only active, schedulable OpenAI OAuth
accounts with an active, unexpired proxy are eligible. By default it monitors
accounts with the gateway-owned `upstream transport error (proxy/network): `
cooldown. API-key routes and other providers are outside this mechanism.

Operators can opt into automatic direct routing in `config.yaml`:

```yaml
gateway:
  proxy_transport_recovery:
    allow_direct_fallback: true # default false
    failure_threshold: 3       # consecutive failed proxy probes; default 3
```

When enabled, all eligible proxy-bound accounts are monitored, even without a
cooldown or current requests. Three successive failed proxy probes trigger one
direct-connect probe of the same HTTPS endpoint. Only if direct connectivity
works does the worker set `proxy_id=NULL`, save the original ID in
`proxy_fallback_origin_id`, and add `extra.proxy_transport_fallback=true`.
Accounts sharing the failed proxy switch together. Only gateway-owned transport
cooldowns are cleared; quota cooldowns, rate limits, overload, manual disabling
and unrelated metadata remain intact. A healthy probe, scan failure, disappearance
of candidates or proxy configuration edit resets the failure streak. Fresh request
failures do not reset a confirmed proxy-outage failure streak.

The saved origin and ownership marker survive application restarts. The worker
continues probing that original proxy while accounts use direct connections.
Two consecutive healthy probes restore `proxy_id` and remove the origin and
marker. Recovery continues even if the operator disables new direct fallbacks.
Ordinary direct accounts and proxy-expiry fallbacks have no worker ownership
marker and are never automatically rebound. An explicit single or bulk proxy
edit cancels transport restoration, including selecting direct during fallback.
Manual revert also removes the marker.

One probe per proxy configuration and scan makes a fresh authenticated proxy
connection and verifies TLS to `https://chatgpt.com/backend-api/codex/models`.
No model generation or account credentials are used. HTTP 200/401 indicates
connectivity; all other statuses, redirects and transport failures fail closed.
Two successive healthy scans are required. A failed scan, proxy edit, new
gateway transport deadline or disappearance of candidates resets the health streak.
For a single healthy proxy, recovery normally occurs in approximately 15–35
seconds after connectivity returns, plus scheduler propagation time. Five-second
probe and thirty-second scan deadlines bound network work. The original ten-minute
cooldown remains the behavior when direct fallback is disabled or direct
connectivity also fails. Failed probes are counted once per proxy per scan,
not once per account; the direct probe is shared across affected proxies.
Route changes affect future requests and do not replay an already started stream.
The OpenAI OAuth transport-cooldown writer also compares the request's proxy
binding before pausing an account. A late failure from an old proxy cannot pause
an account already switched to direct, and a late direct failure cannot pause
an account already restored to its proxy. Active unrelated cooldowns are preserved.

Cooldown recovery compares account ID, update timestamp, proxy ID, cooldown
deadline and reason, plus proxy update timestamp/status/expiry. Route transitions
compare account ID/update timestamp/binding and lock the checked proxy configuration;
restoration also checks the saved origin and ownership marker. A single SQL
statement performs each conditional change and inserts a scheduler outbox event.
The account snapshot is refreshed immediately; existing runtime block logic
uses the refreshed persistent cooldown as its source of truth. Quota cooldowns,
manual disabling, account errors and overload state are never cleared.

Events: `proxy.transport_recovery_started`, `proxy.transport_recovery_restored`,
`proxy.transport_recovery_scan_failed`, `proxy.transport_recovery_clear_failed`,
`proxy.transport_direct_fallback`, `proxy.transport_direct_fallback_failed`.
No proxy credentials or raw network errors are logged by this worker.

Validation includes shared-proxy probe deduplication, consecutive successes,
consecutive failures, direct-connect verification, restart recovery, proxy edits,
scan failures, fresh failures, cancellation, explicit admin choices, nullable
SQL result decoding and conditional atomic recovery. PostgreSQL was also exercised
using transaction-local temporary tables (rolled back), confirming that quota
reasons, manual disables, changed proxy bindings/configuration and newer account
state/deadlines survive an old probe; rate-limit state remains intact after a
successful transport recovery. Live connectivity was checked without changing
any real account's scheduling state.
