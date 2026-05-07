# mihomoctl quickstart

Get from zero to your first node switch in under five minutes.

## What you need

- A server running [mihomo](https://wiki.metacubex.one/) with the **external-controller** enabled. mihomo config:
  ```yaml
  external-controller: 127.0.0.1:9090
  secret: <optional secret>
  ```
- SSH access to that server.
- Optional: the mihomo `secret` value if your config sets one.

## Install

> `<TBD>` is replaced with the actual release URL once Otto's binary is published.

Single binary, no config files:

```bash
# Linux amd64
curl -L <TBD release URL>/mihomoctl-linux-amd64 -o /usr/local/bin/mihomoctl
chmod +x /usr/local/bin/mihomoctl

# Linux arm64 / macOS / Windows binaries are in the same release.

# Verify
mihomoctl --help
```

## Configure auth (env-first)

mihomoctl reads the controller secret from `MIHOMOCTL_SECRET`. **Use the env var, not the `--secret` flag** — flag values leak into shell history and process listings.

```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export MIHOMOCTL_SECRET='<your mihomo secret>'

# If your mihomo config has no secret, skip this step.
```

If your controller is not on `127.0.0.1:9090`, also set `MIHOMOCTL_ENDPOINT`:

```bash
export MIHOMOCTL_ENDPOINT='http://127.0.0.1:9090'
```

## Verify the controller is reachable

```bash
mihomoctl status
```

Expected output:

```
mode: rule
version: v1.18.5
groups:
  PROXY: HK-01
  AUTO: JP-fastest
```

Common failures:

- `cannot connect to mihomo at http://127.0.0.1:9090` — controller isn't running, or the endpoint is wrong. Check mihomo is up; verify `external-controller:` in mihomo config; check `MIHOMOCTL_ENDPOINT`.
- `controller returned 401` — secret is missing or wrong. Re-check `MIHOMOCTL_SECRET` against mihomo's `secret:` config.

## Switch a node

List proxies to find a group and a node:

```bash
mihomoctl proxy list
```

Output:

```
AUTO -> JP-fastest
    HK-01
    HK-02
  * JP-fastest
PROXY -> HK-01
  * HK-01
    HK-02
    JP-01
    US-01
```

`*` marks the currently-selected node. Groups with `type: Selector` (visible via `--json`) accept `proxy set`; policy groups (`URLTest`, `Fallback`, etc.) are read-only — try `proxy set` on them and the command exits 75.

Switch the `PROXY` group to a different node:

```bash
mihomoctl proxy set PROXY JP-01
```

Output:

```
PROXY: JP-01 (was HK-01)
```

Re-check:

```bash
mihomoctl status
```

You should see `PROXY: JP-01` in the `groups:` section. That's the node-switch loop.

## Test, switch, verify (the full daily-use loop)

`proxy set` is "switch", but in real use you usually want to **test** candidate nodes first and **verify** the new node is actually carrying traffic afterwards. v0.2 adds the two commands that close that loop.

### 1. Test — `group delay`

Probe every candidate node in the group against a small target URL (default `http://www.gstatic.com/generate_204`) and rank by latency:

```bash
mihomoctl group delay PROXY
```

Output:

```
PROXY (Selector) selected: HK-01
node	latency_ms	status
  JP-01	98	ok
* HK-01	123	ok
  US-01	-	timeout
```

Sorted by latency ascending; `*` marks the currently-selected node; `timeout` rows sort last and show `-` in the latency column.

`group delay` works on `URLTest`, `Selector`, `Fallback`, and `LoadBalance` groups. It is rejected at the CLI layer (exit 64) for `Direct` and `Reject` groups, which have no candidates to test.

A node-level timeout is **data, not failure** — the command exits 0 and the timed-out node is reported as `status: "timeout"`. Only a controller-side failure (unreachable, network error, request timeout) exits non-zero (75).

### 2. Switch — pick the fastest and `proxy set`

In a script, pick the fastest non-timeout node from `--json` and switch:

```bash
FAST=$(mihomoctl group delay PROXY --json | jq -r '[.results[] | select(.status=="ok")][0].node')
mihomoctl proxy set PROXY "$FAST"
# PROXY: JP-01 (was HK-01)
```

`results` is sorted by latency ascending with timeouts last, so `[0]` after the `status=="ok"` filter is always the fastest live candidate.

### 3. Verify — `connections list`

Snapshot the active connections and confirm new traffic is going through the node you just selected:

```bash
mihomoctl connections list --limit 10
```

Output (tab-separated, sorted by `started_at` descending):

```
started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:05Z	192.168.1.10:55321	8.8.8.8:443	tcp	GEOIP,US,PROXY	PROXY > JP-01	1234/5678
2026-05-07T03:00:01Z	192.168.1.10:55320	1.1.1.1:443	tcp	DOMAIN-SUFFIX,cloudflare.com,PROXY	PROXY > JP-01	890/2456
```

Look at the `chains` column: new connections after `proxy set PROXY JP-01` should end with `JP-01`. Older connections that started before the switch may still show the previous node — they live until the original peer closes them; mihomoctl does not kill connections in v0.2.

For scripts, the `--json` shape is a flat envelope with a top-level `total` (count before `--limit` truncation), so you can detect when output was capped:

```bash
mihomoctl connections list --json | jq '{total, returned: (.connections | length)}'
# {"total": 47, "returned": 47}

# Filter by destination host (matches host/destination/source/rule, OR-semantics, case-insensitive)
mihomoctl connections list --filter google.com --json | jq '.connections[].destination'

# Verify the new node is carrying traffic
mihomoctl connections list --json \
  | jq '.connections[] | select(.chains[-1]=="JP-01") | .destination'
```

Empty snapshot prints `no active connections` and exits 0 — that is not an error, just a quiet network.

That is the full v0.2 daily-use loop: `group delay` → `proxy set` → `connections list`.

## Debug rules and providers (v0.3)

When traffic does not go where you expect, the question is usually one of two: "which rule matched this destination" or "is the provider that owns the candidate node actually healthy". v0.3 adds three commands for that loop.

### Find the matching rule

```bash
mihomoctl rules list --filter google.com
```

<!-- Verified against r18 source `f2e31e57` cmd_rules.go:88-91. -->

```
idx	type	payload	proxy
0	DOMAIN-SUFFIX	google.com	PROXY
```

`--filter` is a substring match against `type`, `payload`, and `proxy` (OR semantics, case-insensitive). `idx` is the rule's evaluation order — the matcher walks rules top-to-bottom and stops at the first hit, so `idx 0` here means this is the first rule and the only one that matters for `google.com`.

If you need the full table, raise `--limit`:

```bash
mihomoctl rules list --limit 1000 --json | jq '.total'
# 234
```

`total` is the count after filtering but before `--limit` truncation, so `total > (.rules | length)` tells you when output was capped.

### Inspect provider health

```bash
mihomoctl providers list
```

<!-- Verified against r18 source `f2e31e57` cmd_providers.go:109-111 (header includes `type` column; rows sorted by name ascending; `updated_at` is empty string when unset). -->

```
name	type	vehicle_type	health	node_count	updated_at
local-pool	Proxy	Inline	unknown	5	
sub-A	Proxy	HTTP	healthy	42	2026-05-07T03:00:00Z
sub-B	Proxy	HTTP	unhealthy	18	2026-05-07T02:48:12Z
```

`health` is one of `healthy` / `unhealthy` / `unknown` — `unknown` means the provider has never been health-checked in the current mihomo session. `vehicle_type` is whatever mihomo's controller emits — common values are `HTTP` (subscription pulled over HTTP), `File` (file-vehicle pull), `Inline` (proxies declared inline in mihomo config), and `Compatible` (mixed). Treat unknown values as opaque if mihomo upstream adds new ones.

> `providers list` is **proxy providers only** in v0.3. Rule providers are out of scope and use a different upstream endpoint.

### Refresh a specific provider

If a provider shows `unhealthy` or `unknown`, trigger a health refresh:

```bash
mihomoctl providers healthcheck sub-B
```

<!-- Verified against r18 source `f2e31e57` cmd_providers.go:140-141 (single-line tab-separated 7 fields, no header). -->

```
sub-B	Proxy	HTTP	unhealthy	18	2026-05-07T02:48:12Z	2026-05-07T03:30:01Z
```

A single tab-separated line; the field order is `provider type vehicle_type health node_count updated_at triggered_at`. The two timestamps sit on the same line so the difference between `triggered_at` (when this CLI invocation fired the refresh) and `updated_at` (mihomo's own subscription / cache timestamp) is visible side-by-side. `triggered_at` may **not** match `updated_at` because mihomo can run the per-node probes asynchronously.

For the global view, follow with `providers list`:

```bash
mihomoctl providers list
```

This is the **two-step flow**: `providers healthcheck <name>` to fire the trigger and confirm it fired; `providers list` to see the post-refresh state side-by-side across all providers.

### What `providers healthcheck` does not give you

`providers healthcheck` returns a **provider-level summary**. It does **not** return per-node probe results — there is no `results: [{node, latency_ms, status}]` array in the response (that schema is specific to `group delay`). If you need per-node latency, use `group delay <group>` on a proxy group that includes the nodes you want to probe.

```bash
# Quick example: find which group exposes a provider's nodes
mihomoctl proxy list --json | jq '.groups[] | select(.candidates[] | contains("sub-B-node-3")) | .name'
# Then probe latencies on that group
mihomoctl group delay <that-group>
```

That is the full v0.3 debug loop: `rules list` to understand routing, `providers list` to see provider health, `providers healthcheck` to refresh and confirm a trigger fired, with `group delay` as the per-node companion when you need per-node latencies.

## Live monitoring + DNS debug (v0.4)

When the question is "is traffic hitting the right node **right now**" or "why is mihomo's DNS giving me this answer", v0.3's snapshot commands (`connections list`, `rules list`) are not enough — you want a stream and a resolver probe. v0.4 adds three commands for that loop: `connections watch` (stream connection events over a WebSocket), `dns query` (resolve a domain through mihomo's resolver), and `cache clear` (flush DNS / fake-IP caches when you want to retest from cold).

This works particularly well over SSH: keep `connections watch` running in one tmux pane, drive `dns query` / `cache clear` / `proxy set` from another, and watch the stream react.

### 1. Stream live connections — `connections watch` _(or alias `conns watch`, v0.4.1)_

In one terminal, start the watcher:

```bash
mihomoctl connections watch --filter google.com
# or, equivalently (v0.4.1):
mihomoctl conns watch --filter google.com
```

**On a TTY** (interactive terminal, v0.4.1), the watcher renders an in-place table that redraws on each upstream snapshot — alternate-screen + sticky header showing `received_at` + match count + filter, sober single-color styling, Ctrl-C cleanly restores the screen and exits 0. See [reference § connections watch — TTY output](./docs/reference.md#tty-interactive-default-v041) for the full rendering spec.

**On a pipe / non-TTY** (e.g. `connections watch | grep ...`, `connections watch | tee log`, or `TERM=dumb`), the watcher falls back to the v0.4 tab-separated row append behavior — preserved byte-identical, except the `up`/`down` column now uses IEC binary units (v0.4.1):

```
received_at	started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:05Z	2026-05-07T03:00:01Z	192.168.1.10:55322	142.250.80.46:443	tcp	DOMAIN-SUFFIX,google.com,PROXY	PROXY > JP-01	256 B/1.0 KiB
```

Cap snapshot output at N connections per emit with `--limit N` (v0.4.1). Applies to all three paths (TTY in-place, non-TTY append, `--json` NDJSON); `--limit 0` or omitted preserves the v0.4 unlimited default.

> **These are snapshots, not per-event open/close pushes.** mihomo's `/connections` WebSocket exposes a periodic poll of the entire connection table; mihomoctl forwards each poll. `event_action` in the JSON envelope is always `"snapshot"` in v0.4 — see [reference § connections watch](./docs/reference.md#mihomoctl-connections-watch).

Snapshots that match no connection (after filter) print `no matching active connections` instead of an empty table — non-empty so you know the watcher is alive.

`--filter` matches `host` / `destination` / `source` / `rule` (OR semantics, case-insensitive). It is **CLI-local** — the CLI receives mihomo's full upstream snapshot and filters client-side, because mihomo's `/connections` WebSocket does not expose a server-side filter parameter. That distinction matters when you are debugging "events not arriving": the upstream is sending all of them, the CLI is dropping anything that does not match.

The stream runs until you Ctrl-C (clean exit 0). Pipe it to `head` for a quick look — the CLI treats `EPIPE` as a successful exit, so `mihomoctl connections watch | head -5` does what you expect.

For scripts, `--json` emits NDJSON with two `type` discriminants: `{"type":"event", "data":{...}}` for connection events and `{"type":"error", "error":{...}}` for streaming-stage errors. Always branch on `.type` before reading further fields.

### 2. Probe DNS — `dns query`

In another terminal:

```bash
mihomoctl dns query google.com
```

A header line shows `domain`, `query_type`, and `status`; subsequent lines list the answer records:

```
google.com	A	NOERROR
name	type	ttl	data
google.com	A	300	142.250.80.46
```

The JSON shape is `{domain, query_type, status, answers: [{name, type, ttl, data}]}`. `status` is the upstream RCODE name (`NOERROR`, `NXDOMAIN`, `SERVFAIL`, …); `NXDOMAIN` is **exit 0**, not exit 66 — the DNS protocol returning "no such name" is a successful query at the CLI layer, with `answers: []`.

`--type` controls the record type (`A` by default). For example:

```bash
mihomoctl dns query google.com --type AAAA
mihomoctl dns query google.com --type CNAME
```

Invalid types fail with exit 64 (usage error), not exit 75 — mihomo rejects the query and the CLI surfaces it as a CLI input error, not a controller failure.

### 3. Flush caches and retest — `cache clear`

If `dns query` gives you a cached answer and you want to see what the upstream resolver actually says, flush mihomo's DNS cache:

```bash
mihomoctl cache clear dns
mihomoctl dns query google.com   # this hit goes upstream
```

`cache clear` has three subcommands: `cache clear fakeip` (flush the fake-IP map), `cache clear dns` (flush the DNS resolver cache), `cache clear all` (both). Bare `cache clear` (no subcommand) is a usage error.

This is **low-impact mutation**: side effects are short-term (subsequent DNS lookups go to upstream resolvers, fake-IP-mapped connections re-allocate as they reopen), with no impact on active connections, mihomo configuration, or the running service. Unlike v0.5's `config reload` / `service restart`, `cache clear` does not require `--yes` or a confirmation token.

`cache clear all` partial failure is reported under the JSON error envelope — the successful sub-cache is **not** rolled back. Scripts handling `cache clear all --json` should branch on the top-level `.error` and read `.error.details.results[]` to act per sub-cache.

### Putting it together

```bash
# Pane 1 (SSH-attached tmux): live stream
mihomoctl connections watch --filter google.com

# Pane 2: probe DNS resolution path, then flush and re-probe
mihomoctl dns query google.com
mihomoctl cache clear dns
mihomoctl dns query google.com   # this one goes upstream

# Pane 2: optionally drive a node switch and watch pane 1 react
mihomoctl proxy set PROXY JP-01
```

That is the full v0.4 live-debug loop: `connections watch` for traffic visibility, `dns query` for resolver introspection, `cache clear` to retest from cold.

## Switch the mode

```bash
mihomoctl mode get
# rule

mihomoctl mode set global
# mode: global (was rule)

mihomoctl mode set rule
# mode: rule (was global)
```

Modes are `rule` (default; only matched traffic uses the proxy), `global` (all traffic via proxy), and `direct` (no proxy).

## Scripting with `--json`

All commands accept `--json` for scriptable output:

```bash
mihomoctl status --json | jq '.mode'
# "rule"

mihomoctl proxy list --json | jq '.groups[] | select(.name=="PROXY") | .selected'
# "JP-01"

# Filter to manually selectable (Selector) groups
mihomoctl proxy list --json | jq '.groups[] | select(.type=="Selector") | .name'
# "PROXY"

mihomoctl mode set rule --json | jq -r '.mode'
# rule
```

**Pre-1.0 stability**: while mihomoctl is in 0.x, `--json` field names, exit codes, and command surface may change between minor releases. Every breaking change is listed in the [CHANGELOG](./CHANGELOG.md) `Breaking` section with a one-line migration note. v1.0 is the first locked contract. Pin to an exact 0.x.y in scripts and read the changelog before upgrading. See [reference § Stability and JSON contract](./reference.md#stability-and-json-contract) for the full rule.

Human-readable output is **never** part of the contract. Don't script against it.

## Common errors

| Error | What it means | Fix |
| --- | --- | --- |
| `cannot connect to mihomo at <url>` | Controller not running or wrong endpoint | Verify `external-controller:` in mihomo config; check `MIHOMOCTL_ENDPOINT`. |
| `controller returned 401` | Auth failed | Set `MIHOMOCTL_SECRET` to mihomo's `secret:` value. |
| `invalid mode "<arg>"; expected rule, global, or direct` | Bad mode argument | Use one of the three valid modes. |
| `group "XYZ" not found, available: A, B, C` | Group name typo | `mihomoctl proxy list` for valid names. |
| `node "XYZ" not found in group "PROXY", available: ...` | Node not in that group | `mihomoctl proxy list`. |

For the full exit-code catalog see the [reference](./reference.md#exit-codes).

## Next steps

- [Full reference](./docs/reference.md) — every command, flag, exit code, and JSON schema (including the v0.4 [JSON error envelope](./docs/reference.md#json-error-envelope-schema)).
- [CHANGELOG](./changelog/CHANGELOG.md) — release history. Pin to an exact `0.x.y` for scripts and read this before upgrading.
- v0.2 commands you may not have tried yet: `group delay` (latency probe), `connections list` (snapshot active connections).
- v0.3 debug commands: `rules list`, `providers list`, `providers healthcheck`.
- v0.4 live-debug commands: `connections watch`, `dns query`, `cache clear` — covered above.
