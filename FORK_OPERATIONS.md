# MagicRealms fork: updates and stream stability

Production updates are built from `MagicRealms/sub2api`, with an immutable Git
commit recorded in the binary and local Docker image. Online binary replacement
is disabled for source builds, including both rollback endpoints. A source build
does not read old official-release cache entries or contact GitHub to display its
version. The admin version panel links to this fork.

## Transport change

The inherited HTTP/2 fallback excluded SOCKS5/SOCKS5H proxies and only observed
failures before response headers. The fork also observes terminal body reads,
recognizes `http2: client connection lost`, and includes both SOCKS schemes.
Client cancellation, timeouts, and ordinary EOF do not trip the circuit. A
successful header is no longer counted as a completed stream. Partial bytes and
the original error reach the caller unchanged; no response is replayed. Existing
threshold, time window and cooldown settings remain in effect; healthy responses
do not erase failures that still fall inside the configured window. Proxy passwords
are omitted from fallback logs.

Compressed response cleanup closes the underlying HTTP body before waiting for
the decoder. A zstd decoder can read ahead into the next frame even after an SSE
completion event has been received. Waiting for that decoder before closing its
network input can otherwise stall response completion and hold request capacity.
The regression test keeps the upstream open after a valid compressed SSE frame:
the old close order blocks, while the corrected order releases it without
changing output bytes or hiding the transport close result.

Production may continue using `gateway.openai_http2.enabled: false` when HTTP/1.1
has proven more reliable for its route. Fixing the fallback is not a reason to
re-enable HTTP/2 without a controlled comparison.

The changes do not alter model selection, reasoning effort, context, output
token limits, or retry budgets. They address connection stability, not model
computation time. Compare latency by model, context size, output length, and
traffic window; do not interpret full response duration as first-token latency.

## Build and deploy

1. Keep `origin` pointing to `https://github.com/MagicRealms/sub2api.git`.
   Fetch and check out a reviewed commit from this fork; do not reset local
   modifications to the upstream repository.
2. From a clean checkout run `bash deploy/build-fork.sh`. Optionally pass an
   explicit unique version, e.g. `0.2.5-mr.1`. Docker, Git and Python 3 are required. Build caches
   default to `$HOME/.cache/sub2api-fork`; `SUB2API_BUILD_CACHE` can select a
   persistent alternative. Stages run sequentially with CPU and memory limits.
   The script runs focused backend regression tests and the frontend production
   build (including locale completeness and TypeScript checks). It creates a
   **local** `magicrealms/sub2api:<version>` image; it does not publish an image.
3. Before deploying, back up PostgreSQL, application data/configuration and
   Compose configuration. Keep a runnable image of the actual current version;
   an image tag can be stale after historical online binary updates.
4. Pin the new image in the existing Compose override, preserving resource
   limits, mounts and environment. Wait for active requests and queues to drain.
   Then run `docker compose up -d --no-deps --pull never sub2api`.
   For the standard deployment, `bash deploy/deploy-fork-image.sh IMAGE COMPOSE_DIR`
   performs the backup, drain check, image-only override change, health check,
   and image rollback on failure. It requires the existing local admin API key
   and the `sub2api` PostgreSQL database/user. For a legacy online-updated
   container, explicitly provide `FORK_ROLLBACK_IMAGE` containing its actual
   running binary; the helper refuses to assume its old image tag is sufficient.
5. Verify `/health`, runtime version, both account streaming tests, the custom
   recharge page, and model-matched error/latency records. Short smoke tests do
   not prove long-term reliability. Roll back to the saved runnable image if
   verification fails. Review database migrations before attempting a version
   rollback that crosses a schema change.

## Incorporating a later upstream release

Add `https://github.com/Wei-Shaw/sub2api.git` as the separate `upstream` remote.
Merge a chosen stable upstream tag into a review branch based on this fork,
resolve conflicts while retaining these patches, run CI and streaming checks,
then update the fork. Build and deploy the resulting fork commit using the steps
above. Do not use the panel updater or the former `update-official-latest.sh`.

Account tests send a short fixed `hi` prompt. Their wall-clock time is neither
TTFT nor a representative measurement for a long customer conversation.
