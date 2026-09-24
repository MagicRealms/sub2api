# Proxy transport cooldown recovery

A durable transport error (proxy authentication, refused connection, DNS or
routing failure) pauses an OpenAI account for ten minutes. Previously a healthy
proxy did not release this cooldown; operators had to clear each account or
wait for expiry.

Observed incident: 2026-09-24 12:52:49 UTC+08, accounts 1336 and 1338
received SOCKS5 connection-refused errors and were paused until 13:02:49.

The recovery worker polls the authoritative database every 15 seconds, including
accounts omitted from scheduler snapshots. It only selects active, schedulable
OpenAI OAuth accounts with an active, unexpired proxy and the gateway-owned
`upstream transport error (proxy/network): ` reason. Direct accounts, API-key
routes, other providers, credential-refresh cooldowns and custom cooldown rules
are outside this recovery mechanism.

One probe per proxy configuration and scan makes a fresh authenticated proxy
connection and verifies TLS to `https://chatgpt.com/backend-api/codex/models`.
No model generation or account credentials are used. HTTP 200/401 indicates
connectivity; all other statuses, redirects and transport failures fail closed.
Two successive healthy scans are required. A failed scan, proxy edit, new
transport deadline or disappearance of candidates resets the health streak.
For a single healthy proxy, recovery normally occurs in approximately 15–35
seconds after connectivity returns, plus scheduler propagation time. Five-second
probe and thirty-second scan deadlines bound network work. The original ten-minute
cooldown remains as a fallback, not a guarantee of proxy recovery.

Recovery compares account ID, update timestamp, proxy ID, cooldown deadline and
reason, plus proxy update timestamp/status/expiry. A single SQL statement clears
only the matching temporary cooldown and inserts a scheduler outbox event.
The account snapshot is refreshed immediately; existing runtime block logic
uses the refreshed persistent cooldown as its source of truth. Quota cooldowns,
manual disabling, account errors and overload state are never cleared.

Events: `proxy.transport_recovery_started`, `proxy.transport_recovery_restored`,
`proxy.transport_recovery_scan_failed`, `proxy.transport_recovery_clear_failed`.
No proxy credentials or raw network errors are logged by this worker.

Validation includes shared-proxy probe deduplication, consecutive successes,
proxy edits, scan failures, fresh failures, cancellation, SQL result decoding and
conditional atomic recovery. The production PostgreSQL version was also exercised
using transaction-local temporary tables (rolled back), confirming that quota
reasons, manual disables, changed proxy bindings/configuration and newer account
state/deadlines survive an old probe; rate-limit state remains intact after a
successful transport recovery. Live connectivity was checked without changing
any real account's scheduling state.
