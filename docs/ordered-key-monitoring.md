# Ordered API keys and channel monitoring

An API key can contain ordered groups from multiple providers. Its first group
does not restrict the key to that provider. Each request uses its requested model
and supported gateway protocol to find the first eligible route. The same key can
be used by OpenAI, Anthropic and Gemini clients with their own endpoint formats.

## Connection addresses

| Client | Base URL | Final request |
| --- | --- | --- |
| Anthropic SDK / Claude Code | `https://codex.magicrealms.top` | `/v1/messages` |
| OpenAI SDK / Codex | `https://codex.magicrealms.top/v1` | `/v1/responses` or `/v1/chat/completions` |
| Native Gemini client | `https://codex.magicrealms.top` | `/v1beta/models/{model}:generateContent` |

The channel monitor accepts either an origin or a base ending in `/v1` and
deduplicates complete path segments. Its provider selection controls the body,
authorization headers and request path. Selecting an Anthropic-capable key alone
does not turn an OpenAI request body into an Anthropic request body. The form
shows the resulting request URL. Its key picker checks all selected groups and
displays their saved order. Reopening the picker refreshes keys and groups.

The normal key usage dialog also offers a configuration example for each selected
group. Changing the example never changes saved route priority or billing.
WebSocket and stateful asynchronous APIs retain the primary group. Secondary
group examples therefore do not offer the WebSocket transport, and selecting a
different group resets the client tab even when both groups share a provider.

## Probe cost and interpretation

Each model probe follows the same ordered routing and charging rules as normal
traffic. It does not fan out to every group. To measure one group independently,
create a key restricted to that group. A multi-group key measures the combined
route's availability, including eligible failover. No client-controlled group
header is added to bypass normal policy.

For OpenAI self-checks, “Use current service” defaults to Responses. Its default
body disables storage, caps output at 50 tokens, and uses low reasoning for known
reasoning model families. Ordinary models receive no unsupported reasoning field.
Explicit request-template settings still take precedence. Normal customer models,
reasoning, output limits and prompts are unchanged.

## Verified upstream request compression

Existing OAuth compression stays enabled by `gateway.openai_request_gzip_enabled`.
Third-party compression additionally requires explicit exact-host opt-in:

```yaml
gateway:
  openai_request_gzip_enabled: true
  openai_request_gzip_hosts:
    - sharellm.net
```

Enable a host only after a real compressed request completes with output. The
third-party path is restricted to HTTPS `/v1/responses`, port 443, no query or URL
credentials, an authenticated replayable body between 64 KiB and 64 MiB, and no
existing content encoding or body signature. Compression preserves every decoded
request byte and requires at least 10% size reduction. Two encoder slots bound
CPU use; busy, cancelled, incompressible or unsupported requests use their original
body without waiting. An ambiguous encoded upstream request is never replayed as
plaintext as part of this mechanism.

This reduces upload cost for large contexts. It does not remove the provider's
queue, prefill or reasoning time, and does not guarantee lower generation TTFT.
