# mihomoctl reference

> Status: **v0.4 reference**. Locked against PRD-0004 v0.6 (canonical `49873790`), ADR-0007 v0.2 (`3f123129`), ADR-0008 (`8223bf90`), ADR-0010 v0.2 (`9c414ffd`), and ADR-0011 v0.2 (`81dda8c5`); Examples and human-output samples are byte-verified against the v0.4 implementation on the canonical repo (`github.com/vika2603/mihomoctl`, `feature/v0.4-refactor-baseline`). The two-phase stability rule applies — JSON shape, exit codes, command tree, and flag names may change between minor 0.x releases; see [Stability and JSON contract](#stability-and-json-contract).

## Synopsis

```
mihomoctl <command> [flags]
```

Fifteen commands in v0.4 (counted as leaves; sustains the v0.3 reference v1.7 leaves convention):

| Command | Purpose |
| --- | --- |
| `status` | Show mihomo version, current mode, and current selection per proxy group. |
| `mode get` | Print the current mihomo mode. |
| `mode set <mode>` | Switch mihomo mode to `rule`, `global`, or `direct`. |
| `proxy list` | List every selectable proxy group with the currently-selected node. |
| `proxy set <group> <node>` | Select a node in a proxy group. |
| `group delay <group>` | Probe candidate-node latency for a proxy group (URLTest / Selector / Fallback / LoadBalance). _v0.2 added._ |
| `connections list` | Snapshot of currently-active mihomo proxy connections. _v0.2 added._ |
| `rules list` | Snapshot of mihomo's matching rules. _v0.3 added._ |
| `providers list` | Snapshot of proxy providers (subscription / inline / file / compatible) with current health state. _v0.3 added._ |
| `providers healthcheck <name>` | Trigger mihomo-side health refresh on a Proxy provider and return a provider-level summary. _v0.3 added._ |
| `connections watch` | Stream live mihomo connection events over a WebSocket until Ctrl-C. _v0.4 added._ |
| `dns query <domain>` | Resolve a domain through mihomo's internal DNS resolver (read-only). _v0.4 added._ |
| `cache clear fakeip` | Flush mihomo's fake-IP map (low-impact mutation). _v0.4 added._ |
| `cache clear dns` | Flush mihomo's DNS resolver cache (low-impact mutation). _v0.4 added._ |
| `cache clear all` | Flush both fake-IP and DNS caches in one call (low-impact mutation; partial failure reported under the JSON error envelope). _v0.4 added._ |

## Global flags

These apply to every command.

| Flag | Env | Default | Notes |
| --- | --- | --- | --- |
| `--endpoint <url>` | `MIHOMOCTL_ENDPOINT` | `http://127.0.0.1:9090` | mihomo external-controller URL. |
| `--secret <value>` | `MIHOMOCTL_SECRET` | _none_ | mihomo controller secret. **Use env. The flag value leaks into shell history and process listings.** |
| `--timeout <duration>` | _none_ | `5s` | HTTP request timeout for controller calls (cobra layer). Distinct from `group delay --delay-timeout`, which is the mihomo-side delay-probe timeout. For `group delay`, the request timeout is auto-extended to at least `delay-timeout + 1s` unless `--timeout` is set explicitly. |
| `--json` | _none_ | off | Emit stable JSON instead of human-readable output. See [Stability and JSON contract](#stability-and-json-contract). |
| `--help`, `-h` | _none_ | _none_ | Print help and exit 0. Supported at root (`mihomoctl --help`) and on each subcommand group (`mihomoctl proxy --help`, `mihomoctl group --help`, `mihomoctl connections --help`, `mihomoctl mode --help`). |

If both flag and env are set, the flag wins. **Global flags may appear before or after the command** — both `mihomoctl --json status` and `mihomoctl status --json` work, and so do positions in the middle for two-word commands like `mihomoctl proxy --json list`. Pick whichever reads better; the parser handles either.

## Stability and JSON contract

mihomoctl's stability bar follows a two-phase rule, set by ADR-0002 with the pre-1.0 framework added by ADR-0004 amendment:

**Pre-1.0 (0.x — current).** mihomoctl is experimental. The `--json` output shape, exit-code values, command tree, and flag names **may change between any two minor releases**. Every breaking change is documented in the [CHANGELOG](./CHANGELOG.md) under a `Breaking` entry with a one-line migration note. **No major version bump is required during 0.x.** Scripts written against a 0.x release should pin to that exact 0.x.y version and read the changelog before upgrading.

**v1.0 onward (locked contract).** From v1.0.0 the public surface — `--json` shape (field names, types, required/optional status, success/error envelope), exit-code values, command tree, and flag names — is locked. Any change that breaks that contract is a **breaking change** and requires both a `Breaking` changelog entry and a **major version bump** (v2.0.0). Adding optional JSON fields, new commands, new flags, or new exit codes that don't replace existing ones remains non-breaking.

Human-readable output is **never** part of the contract, in either phase. Don't script against it.

The two-phase rule reflects CEO direction (DM 2026-05-07): pre-1.0 surfaces are working drafts, v1.0 is the formal launch.

**Contract freeze (v0.8 → v1.0)**. The mechanism that turns "working draft" into "locked contract" is defined by [ADR-0011](./adr/0011-contract-freeze.md) (canonical `81dda8c5`): a JSON contract manifest enumerates every locked field name, type, optionality, and exit code; a golden contract test in CI fails the build on any unintended drift. The manifest and golden test land in v0.8, before v1.0. v0.4 ships under the same pre-1.0 two-phase rule as prior releases — the freeze mechanism does **not** retroactively lock v0.4's `--json` shapes. What v0.4 _does_ do is introduce two new contract surfaces ([streaming NDJSON](#mihomoctl-connections-watch) and the [JSON error envelope](#json-error-envelope-schema)) whose stable identifiers (`type`, `code`, `category`) are intended to survive into the v1.0 lock; surrounding wording and `details` payloads remain documentation-for-recognition.

## Auth and connection

mihomoctl reads the controller secret from `MIHOMOCTL_SECRET`. The `--secret` flag exists as an explicit override but is documented here only as an escape hatch — values passed via flag are visible in shell history (`~/.bash_history`, `~/.zsh_history`) and process listings (`ps aux`). For day-to-day use, set the env var in your shell profile.

If your mihomo config has no `secret:`, mihomoctl makes unauthenticated requests. The tool does not pre-validate that a secret is required — it sends the request and surfaces auth failures from the controller's response (see [Exit codes](#exit-codes), code 77).

If the controller is not on `127.0.0.1:9090`, set `MIHOMOCTL_ENDPOINT` (or `--endpoint`).

### Secret material guard (contract — ADR-0002 boundary)

Whether the secret is set via `MIHOMOCTL_SECRET` or `--secret <value>`, mihomoctl **never** writes the secret to any of the following five output surfaces:

1. `--help` / `-h` text on any command (root, subcommand group, or leaf). Env-loaded values do **not** appear as cobra flag defaults — `--secret` is rendered as `default ""` regardless of `MIHOMOCTL_SECRET`.
2. `--json` success-path output.
3. Structured error output (any future error envelope, including the JSON error envelope tracked in ADR-0010 for v0.4).
4. Plain `stderr` error messages.
5. Any debug / verbose / structured log surface (mihomoctl has no debug mode today; the guard pre-applies to any future `--verbose` / trace output).

Implementation: the secret is read from env or flag at command-execution time only and used solely to construct the `Authorization: Bearer <secret>` header for the controller HTTP call. CLI-framework default rendering is treated as a leak surface — sensitive material must merge at execution time, never as a cobra default.

Two regression tests guard this boundary (added in v0.2.0; coverage extended in v0.3.0):

- **Leak guard** (`TestSecretEnvDoesNotLeakInHelp`): with `MIHOMOCTL_SECRET=<canary>` exported, `--help` is exercised on every command — currently 12 surfaces (v0.2's 7 plus v0.3's 5 new: `rules`, `rules list`, `providers`, `providers list`, `providers healthcheck`). CI fails if the canary substring appears in any output. New commands added in future minor releases must extend this list.
- **Auth functional guard** (`TestSecretEnvUsedAtExecution`): an httptest controller requiring `Authorization: Bearer <secret>` accepts requests with `MIHOMOCTL_SECRET` set and no `--secret` flag, proving env-first auth still actually authenticates.

If you ever see a secret value in any mihomoctl output, treat it as a security regression and file a high-priority bug — the regression tests are designed to fail the build before such a release can ship.

## Exit codes

All commands follow this sysexits-style contract (canonical source: ADR-0002).

| Code | Meaning | Examples |
| --- | --- | --- |
| 0 | OK | Command completed successfully. |
| 64 | Usage error | Invalid command, flag, mode, or argument shape. |
| 66 | Not found | Requested proxy group or node does not exist. |
| 70 | Software error | Unexpected CLI bug or unclassified internal error. |
| 71 | System error | Local OS / runtime failure before a controller request completes. |
| 73 | Cannot create output | Cannot write requested output file or generated artifact. |
| 75 | Temporary failure | Controller unavailable, network timeout, or retryable transient failure. |
| 77 | Permission / auth error | Controller returned an auth or permission failure, including missing or invalid secret. |

Scripts can rely on these codes within a single 0.x release. Like the JSON shape (see [Stability and JSON contract](#stability-and-json-contract)), exit-code values may change between minor releases pre-1.0; each change is documented in the [CHANGELOG](./CHANGELOG.md) `Breaking` section. The set locks at v1.0.

---

## `mihomoctl status`

Show mihomo version, current mode, and the selected node for every proxy group.

### Synopsis

```
mihomoctl status [--json] [--endpoint URL] [--secret VALUE]
```

### Output (human-readable)

```
mode: rule
version: v1.18.5
groups:
  PROXY: HK-01
  AUTO: JP-fastest
```

### Output (`--json`)

```json
{
  "groups": [
    {"name": "AUTO", "type": "URLTest", "selected": "JP-fastest", "candidates": ["HK-01", "HK-02", "JP-fastest"]},
    {"name": "PROXY", "type": "Selector", "selected": "HK-01", "candidates": ["HK-01", "HK-02", "JP-01", "US-01"]}
  ],
  "mode": "rule",
  "version": "v1.18.5"
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `version` (string, required) — mihomo version string.
- `mode` (string, required) — one of `rule`, `global`, `direct`.
- `groups` (array, required) — one entry per group exposed by mihomo that has selectable candidate nodes (Selector, URLTest, Fallback, LoadBalance, Relay). Sorted alphabetically by `name`. Built-in proxies `DIRECT` and `REJECT` are excluded.
  - `name` (string, required) — group name.
  - `type` (string, required) — group type (`Selector`, `URLTest`, `Fallback`, `LoadBalance`, `Relay`). Only `Selector` accepts manual `proxy set`.
  - `selected` (string, required) — currently-selected node name.
  - `candidates` (array of strings, required) — all candidate node names in the group.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Status retrieved successfully. |
| 75 | Controller unreachable or timed out. |
| 77 | Auth failed (secret missing or wrong). |

### Examples

```bash
# Default: human-readable
$ mihomoctl status
mode: rule
version: v1.18.5
groups:
  PROXY: HK-01
  AUTO: JP-fastest

# JSON for scripting
$ mihomoctl status --json | jq '.mode'
"rule"

$ mihomoctl status --json | jq '.groups[] | select(.name=="PROXY") | .selected'
"HK-01"

$ mihomoctl status --json | jq '.groups[] | select(.type=="Selector") | .name'
"PROXY"
```

---

## `mihomoctl mode get`

Print the current mihomo mode.

### Synopsis

```
mihomoctl mode get [--json] [--endpoint URL] [--secret VALUE]
```

### Output (human-readable)

```
rule
```

(One line; one of `rule`, `global`, `direct`.)

### Output (`--json`)

```json
{"mode": "rule"}
```

Schema:

- `mode` (string, required) — one of `rule`, `global`, `direct`.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Mode retrieved. |
| 75 | Controller unreachable. |
| 77 | Auth failed. |

### Examples

```bash
$ mihomoctl mode get
rule

$ mihomoctl mode get --json | jq -r '.mode'
rule
```

---

## `mihomoctl mode set <mode>`

Switch mihomo mode. The command echoes the resulting state (mutation commands are not silent).

### Synopsis

```
mihomoctl mode set <rule|global|direct> [--json] [--endpoint URL] [--secret VALUE]
```

### Arguments

- `<mode>` — required. One of `rule`, `global`, `direct`. Anything else is a usage error (exit 64).

### Output (human-readable)

```
mode: global (was rule)
```

### Output (`--json`)

```json
{"mode": "global", "previous": "rule"}
```

Schema:

- `mode` (string, required) — new mode after the switch.
- `previous` (string, required) — mode before the switch.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Mode switched successfully. |
| 64 | `<mode>` not one of the three allowed values. |
| 75 | Controller unreachable. |
| 77 | Auth failed. |

### Examples

```bash
$ mihomoctl mode set global
mode: global (was rule)

$ mihomoctl mode set rule
mode: rule (was global)

$ mihomoctl mode set typo
Error: invalid mode "typo"; expected rule, global, or direct
$ echo $?
64
```

---

## `mihomoctl proxy list`

List proxy groups exposed by mihomo that have selectable candidate nodes, with the currently-selected node per group. Built-in proxies `DIRECT` and `REJECT` are excluded.

Includes both **Selector** groups (manually selectable via `proxy set`) and **policy groups** (`URLTest`, `Fallback`, `LoadBalance`, `Relay`) which auto-select by their own policy and are read-only here. The `type` field in `--json` distinguishes them; the human output does not, since the listing is for inspection. Use `proxy set` only on `Selector` groups — it errors on the others.

### Synopsis

```
mihomoctl proxy list [--json] [--endpoint URL] [--secret VALUE]
```

### Output (human-readable)

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

`*` marks the currently-selected node. Groups are sorted alphabetically by name.

### Output (`--json`)

```json
{
  "groups": [
    {"name": "AUTO", "type": "URLTest", "selected": "JP-fastest", "candidates": ["HK-01", "HK-02", "JP-fastest"]},
    {"name": "PROXY", "type": "Selector", "selected": "HK-01", "candidates": ["HK-01", "HK-02", "JP-01", "US-01"]}
  ]
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `groups` (array, required) — one entry per group with selectable candidates. Sorted alphabetically by `name`.
  - `name` (string, required) — group name.
  - `type` (string, required) — group type. One of `Selector`, `URLTest`, `Fallback`, `LoadBalance`, `Relay`.
  - `selected` (string, required) — currently-selected node.
  - `candidates` (array of strings, required) — all candidate nodes in this group.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | List retrieved successfully. |
| 75 | Controller unreachable. |
| 77 | Auth failed. |

### Examples

```bash
$ mihomoctl proxy list
AUTO -> JP-fastest
    HK-01
    HK-02
  * JP-fastest
PROXY -> HK-01
  * HK-01
  ...

$ mihomoctl proxy list --json | jq '.groups[].name'
"AUTO"
"PROXY"

# Filter to Selector groups (the ones you can `proxy set`)
$ mihomoctl proxy list --json | jq '.groups[] | select(.type=="Selector") | .name'
"PROXY"
```

---

## `mihomoctl proxy set <group> <node>`

Select a node in a proxy group. Echoes the resulting state.

### Synopsis

```
mihomoctl proxy set <group> <node> [--json] [--endpoint URL] [--secret VALUE]
```

### Arguments

- `<group>` — required. Name of an existing **Selector** proxy group. Only Selector-type groups accept manual node selection. Non-selector groups like `URLTest`, `Fallback`, `LoadBalance`, and `Relay` choose nodes by their own policy and reject manual `proxy set`; the controller returns an error and mihomoctl exits 75.
- `<node>` — required. Name of a node within `<group>`.

Names containing spaces, slashes, or non-ASCII characters are valid; quote them per shell rules.

### Output (human-readable)

```
PROXY: JP-01 (was HK-01)
```

### Output (`--json`)

```json
{"group": "PROXY", "selected": "JP-01", "previous": "HK-01"}
```

Schema:

- `group` (string, required).
- `selected` (string, required) — newly-selected node.
- `previous` (string, required) — previously-selected node.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Selection succeeded. |
| 64 | Missing `<group>` or `<node>` argument. |
| 66 | `<group>` does not exist, or `<node>` is not in `<group>`. |
| 75 | Controller unreachable. |
| 77 | Auth failed. |

### Examples

```bash
$ mihomoctl proxy set PROXY JP-01
PROXY: JP-01 (was HK-01)

$ mihomoctl proxy set PROXY DOES-NOT-EXIST
Error: node "DOES-NOT-EXIST" not found in group "PROXY", available: HK-01, HK-02, JP-01, US-01
$ echo $?
66

$ mihomoctl proxy set NO-SUCH-GROUP HK-01
Error: group "NO-SUCH-GROUP" not found, available: AUTO, PROXY
$ echo $?
66

# Names with spaces — quote per shell rules
$ mihomoctl proxy set 'My Group' 'Node A'
My Group: Node A (was Node B)
```

---

## `mihomoctl group delay <group>`

Probe candidate-node latency for a proxy group, returning per-node test results. Use this to pick the fastest node before `proxy set`.

### Group types

`group delay` works on the four group types that have selectable candidates:

- `URLTest` — automatic latency-based selection. Calling `delay` triggers a fresh probe.
- `Selector` — manual selection. Calling `delay` triggers a health check across candidates.
- `Fallback` — primary-with-fallback. Calling `delay` probes all candidates.
- `LoadBalance` — round-robin / hash-balanced. Calling `delay` probes all candidates.

`Direct` and `Reject` are rejected at the CLI layer with exit 64 — those types do not own candidate nodes, and probing them has no useful meaning.

### Side effect note

`/group/{name}/delay` is **not pure read-only**. The controller probes each candidate against `--url` and, for automatic policy groups, may clear the previously-fixed selection so the policy re-applies on the next traffic. We treat this as a safe controller-side refresh rather than a user-visible mutation: it does not change global mode, does not modify config, and does not emit a `previous` field. If you want to lock a node yourself, use `proxy set` after reading `delay` results. Source: <https://wiki.metacubex.one/en/api/>.

### Synopsis

```
mihomoctl group delay <group> [--delay-timeout DURATION] [--url URL] \
    [--json] [--endpoint URL] [--secret VALUE] [--timeout DURATION]
```

### Arguments

- `<group>` — required. Name of an existing proxy group of type `URLTest`, `Selector`, `Fallback`, or `LoadBalance`. Use `mihomoctl proxy list` to discover available group names.

### Flags

- `--delay-timeout <duration>` (default `5s`) — mihomo-side per-probe timeout. Passed as `?timeout=<ms>` to `/group/<name>/delay`. Per-node timeouts are reported as `status: "timeout"` data, not command failure (see [Per-node timeout is data, not failure](#per-node-timeout-is-data-not-failure) below).
- `--url <url>` (default `http://www.gstatic.com/generate_204`) — target URL the controller fetches when probing each candidate.
- The global `--timeout` (default `5s`) controls the HTTP request to the controller itself. For `group delay`, mihomoctl auto-extends the request timeout to at least `delay-timeout + 1s` unless `--timeout` is set explicitly, so the probe has time to complete.

### Output (human-readable)

```
PROXY (Selector) selected: HK-01
node	latency_ms	status
* HK-01	123	ok
  JP-01	98	ok
  US-01	-	timeout
```

The header line shows `<group> (<type>) selected: <node>`. The table is sorted by latency ascending; `timeout` rows sort last and tie-break by node name. `*` marks the currently-selected node; nodes with no measurement display `-` in the `latency_ms` column.

Human output is intentionally simple and is **not part of the contract** — script against `--json`.

### Output (`--json`)

```json
{
  "group": "PROXY",
  "type": "Selector",
  "selected": "HK-01",
  "url": "http://www.gstatic.com/generate_204",
  "test_timeout_ms": 5000,
  "results": [
    {"node": "JP-01", "latency_ms": 98, "status": "ok"},
    {"node": "HK-01", "latency_ms": 123, "status": "ok"},
    {"node": "US-01", "latency_ms": null, "status": "timeout"}
  ]
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `group` (string, required) — group name passed on the command line.
- `type` (string, required) — group type. One of `URLTest`, `Selector`, `Fallback`, `LoadBalance`.
- `selected` (string, required) — currently-selected node in the group.
- `url` (string, required) — probe target URL used (matches `--url` or its default).
- `test_timeout_ms` (integer, required) — probe timeout in milliseconds (matches `--delay-timeout` converted to ms).
- `results` (array, required) — one entry per candidate node. **Sort: `latency_ms` ascending; `timeout` rows sort last; ties break by `node` ascending.**
  - `node` (string, required) — candidate node name.
  - `latency_ms` (integer or null, required) — measured latency in milliseconds. `null` if `status` is `timeout`.
  - `status` (string, required) — `ok` or `timeout`.

> **Schema note**: the `results` field is independent of the `candidates: []string` field used by `proxy list` / `status`. `group delay` introduces per-node test metadata as objects; the v0.1 `candidates` field stays a flat string array. Don't confuse the two when writing `jq` queries.

### Per-node timeout is data, not failure

If the controller responds within the request timeout but one or more candidate nodes fail to reach `--url` within `--delay-timeout`, those rows come back as `{"latency_ms": null, "status": "timeout"}` and the command exits **0**. Per-node timeout is information, not error.

The command exits non-zero only if the controller itself fails — unreachable, network error, request timeout — see [Exit codes](#exit-codes-3) below.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Probe completed (some nodes may have `status: "timeout"` — that is data). |
| 64 | `<group>` is type `Direct` / `Reject`, or `--delay-timeout` is `<= 0`. |
| 66 | `<group>` does not exist. |
| 75 | Controller unreachable, network error, or HTTP request timed out (per global `--timeout`). |
| 77 | Auth failed (secret missing or wrong). |

### Performance budget

Pre-1.0 informational targets, not contract:

- Default-timeout invocation (`--delay-timeout 5s`): **p95 < 6s** end-to-end (mihomo probe time + CLI overhead < 1s).
- The CLI overhead alone (parsing, listing proxies for type validation, formatting) is typically <300ms; the rest is mihomo-side probe time.

### Examples

```bash
# Default: human-readable, latency-sorted
$ mihomoctl group delay PROXY
PROXY (Selector) selected: HK-01
node	latency_ms	status
  JP-01	98	ok
* HK-01	123	ok
  US-01	-	timeout

# JSON for scripting — pick the fastest non-timeout node
$ mihomoctl group delay PROXY --json | jq -r '[.results[] | select(.status=="ok")][0].node'
JP-01

# Tighter probe timeout (controller-side)
$ mihomoctl group delay PROXY --delay-timeout 2s --json | jq '.results[] | select(.status=="timeout") | .node'
"US-01"

# Test against a different URL
$ mihomoctl group delay PROXY --url https://example.com --json | jq '.url'
"https://example.com"

# Direct/Reject groups are rejected at the CLI layer
$ mihomoctl group delay GLOBAL-DIRECT
Error: group "GLOBAL-DIRECT" type "Direct" does not support delay test, applicable types: URLTest, Selector, Fallback, LoadBalance
$ echo $?
64

# Combine with `proxy set` to pick the fastest and switch
$ FAST=$(mihomoctl group delay PROXY --json | jq -r '[.results[] | select(.status=="ok")][0].node')
$ mihomoctl proxy set PROXY "$FAST"
PROXY: JP-01 (was HK-01)
```

---

## `mihomoctl connections list`

Snapshot the currently-active mihomo proxy connections. Useful for verifying that traffic is flowing through the node you just selected, or for debugging stuck connections.

This is a one-shot snapshot. For a live event stream that fires on every connection open / close, see [`connections watch`](#mihomoctl-connections-watch) (v0.4).

### Synopsis

```
mihomoctl connections list [--limit N] [--filter PATTERN] \
    [--json] [--endpoint URL] [--secret VALUE] [--timeout DURATION]
```

### Flags

- `--limit <n>` (default `50`) — return at most N connections after sorting. Must be `>= 1`; `--limit 0` is a usage error (exit 64). When the snapshot has fewer connections than `--limit`, all are returned with no error (limit is a cap, not a target).
- `--filter <pattern>` (optional, default empty = no filter) — substring match against `host`, `destination`, `source`, and `rule`. **OR semantics**: a connection is included if the pattern matches any of those four fields. Match is case-insensitive.

  > Filter scope note: `host` is the destination hostname (e.g. `www.google.com`), `destination` is the resolved `<ip>:<port>`. The filter checks both, so `--filter google.com` matches connections to that host whether the snapshot has a hostname or only an IP that happens to contain the pattern.

### Output (human-readable)

```
started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:00Z	192.168.1.10:55321	8.8.8.8:443	tcp	GEOIP,US,PROXY	PROXY > JP-01	1234/5678
2026-05-07T02:58:12Z	192.168.1.10:55320	1.1.1.1:443	tcp	DOMAIN-SUFFIX,cloudflare.com,PROXY	PROXY > JP-01	890/2456
```

Tab-separated; sorted by `started_at` descending. The `chains` column joins the proxy chain with ` > `. The `up/down` column is `<upload_bytes>/<download_bytes>`.

Empty snapshot prints `no active connections` and exits 0 (it is not a not-found condition).

### Output (`--json`)

```json
{
  "total": 2,
  "limit": 50,
  "connections": [
    {
      "id": "8a4f2b1e-...",
      "started_at": "2026-05-07T03:00:00Z",
      "network": "tcp",
      "source": "192.168.1.10:55321",
      "destination": "8.8.8.8:443",
      "host": "dns.google",
      "rule": "GEOIP,US,PROXY",
      "chains": ["PROXY", "JP-01"],
      "upload_bytes": 1234,
      "download_bytes": 5678
    }
  ]
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `total` (integer, required) — number of connections matching the filter **before** `--limit` is applied. Use this to detect when output was truncated.
- `limit` (integer, required) — the `--limit` value used for this call (default `50`).
- `connections` (array, required) — connections after filter and sort, capped at `limit`. **Sort: `started_at` descending; ties break by `id` ascending.**
  - `id` (string, required) — mihomo connection ID.
  - `started_at` (string, required) — RFC 3339 timestamp in UTC, or empty string if mihomo did not report a start time.
  - `network` (string, required) — `tcp` or `udp`.
  - `source` (string, required) — `<ip>:<port>` of the local client. Empty string if mihomo did not report it.
  - `destination` (string, required) — `<ip>:<port>` of the remote endpoint. Empty string if mihomo did not report it.
  - `host` (string, required) — destination hostname as resolved by mihomo (e.g. via DNS or SNI). May be empty.
  - `rule` (string, required) — the mihomo rule that matched this connection.
  - `chains` (array of strings, required) — proxy chain applied to this connection, ordered outermost → innermost as mihomo reports it.
  - `upload_bytes` (integer, required) — bytes sent client → remote on this connection.
  - `download_bytes` (integer, required) — bytes received remote → client on this connection.

> **Schema note**: this is a CLI-normalized flat shape. The raw mihomo upstream nests source / destination / host under a `metadata` object — that nested shape stays inside `internal/mihomo` and is **not** part of the public CLI contract.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Snapshot retrieved successfully (empty snapshot is still 0). |
| 64 | `--limit 0` or `--limit <` 0; passing extra positional arguments. |
| 75 | Controller unreachable, network error, or HTTP request timed out. |
| 77 | Auth failed. |

### Performance budget

Pre-1.0 informational target, not contract:

- Snapshot list with default flags: **p95 < 1s**. The cost is dominated by the controller's `/connections` response size; CLI overhead (filter, sort, format) is typically <50ms even on snapshots in the low thousands.

### Examples

```bash
# Default snapshot
$ mihomoctl connections list
started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:00Z	192.168.1.10:55321	8.8.8.8:443	tcp	GEOIP,US,PROXY	PROXY > JP-01	1234/5678
...

# Verify traffic is flowing through a specific node after `proxy set`
$ mihomoctl connections list --json \
    | jq '.connections[] | select(.chains[-1]=="JP-01") | .destination'
"8.8.8.8:443"
"1.1.1.1:443"

# Filter by destination host (matches host or destination IP substrings)
$ mihomoctl connections list --filter google.com
started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:00Z	192.168.1.10:55321	8.8.8.8:443	tcp	GEOIP,US,PROXY	PROXY > JP-01	1234/5678

# Filter by rule
$ mihomoctl connections list --filter GEOIP,US --json | jq '.total'
12

# Limit cap
$ mihomoctl connections list --limit 5 --json | jq '.connections | length'
5

# Detect truncation (total > limit)
$ mihomoctl connections list --limit 5 --json | jq '{total, limit, returned: (.connections | length)}'
{"total": 47, "limit": 5, "returned": 5}

# Empty snapshot
$ mihomoctl connections list
no active connections
$ echo $?
0
```

---

## `mihomoctl rules list`

Snapshot mihomo's matching rules in the order the matcher applies them. Use this to debug "why is this traffic going through that node" when the configured rule set is not behaving as you expect.

### Synopsis

```
mihomoctl rules list [--limit N] [--filter PATTERN] \
    [--json] [--endpoint URL] [--secret VALUE] [--timeout DURATION]
```

### Flags

- `--limit <n>` (default `50`) — return at most N rules after sorting. Must be `>= 1`; `--limit 0` is a usage error (exit 64). When the snapshot has fewer rules than `--limit`, all are returned with no error (limit is a cap, not a target).
- `--filter <pattern>` (optional, default empty = no filter) — substring match against `type`, `payload`, and `proxy`. **OR semantics**: a rule is included if the pattern matches any of those three fields. Match is case-insensitive.

  > Filter field names exactly mirror the JSON schema field names (`type` / `payload` / `proxy`). Don't introduce filter aliases in scripts or docs that reference this command — the documented field names are the contract.

### Output (human-readable)

<!-- Synthesized from Atlas msg=4b5e9b10 / Vince msg=71b3fbc6 r18 source line-cite review (cmd_rules.go:88 header `idx\ttype\tpayload\tproxy`; cmd_rules.go:104-106 sort.SliceStable by Idx ascending). Binary byte-verify pending r-N follow-up. -->

```
idx	type	payload	proxy
0	DOMAIN-SUFFIX	google.com	PROXY
1	DOMAIN-KEYWORD	cloudflare	PROXY
2	GEOIP	CN	DIRECT
3	MATCH		PROXY
```

Tab-separated; rules are emitted in the order mihomo's matcher evaluates them (rule index ascending). The `payload` column is empty for rules that do not take a payload (e.g. `MATCH`).

### Output (`--json`)

```json
{
  "total": 234,
  "limit": 50,
  "rules": [
    {"idx": 0, "type": "DOMAIN-SUFFIX", "payload": "google.com", "proxy": "PROXY"},
    {"idx": 1, "type": "DOMAIN-KEYWORD", "payload": "cloudflare", "proxy": "PROXY"},
    {"idx": 2, "type": "GEOIP", "payload": "CN", "proxy": "DIRECT"},
    {"idx": 3, "type": "MATCH", "payload": "", "proxy": "PROXY"}
  ]
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `total` (integer, required) — number of rules matching the filter **before** `--limit` is applied. Use this to detect when output was truncated (`total > (.rules | length)` means the snapshot had more rules than `--limit`).
- `limit` (integer, required) — the `--limit` value used for this call (default `50`).
- `rules` (array, required) — rules after filter, capped at `limit`. **Sort: `idx` ascending** (the order mihomo's matcher evaluates them).
  - `idx` (integer, required) — rule's evaluation order in mihomo's matcher (0-based). The matcher walks rules in `idx` ascending order and stops at the first hit, so `idx` is the only field that ranks a rule against its peers.
  - `type` (string, required) — rule type (e.g. `DOMAIN-SUFFIX`, `DOMAIN-KEYWORD`, `GEOIP`, `IP-CIDR`, `MATCH`). Upstream may add new types; treat as opaque if you do not recognize it.
  - `payload` (string, required) — rule payload (e.g. `google.com` for `DOMAIN-SUFFIX`, `CN` for `GEOIP`). Empty string for rules that take no payload (e.g. `MATCH`).
  - `proxy` (string, required) — target proxy / group name the rule routes to. May be a built-in (`DIRECT`, `REJECT`) or a user-defined group.

> **Filter scope note**: filtering matches `type`, `payload`, and `proxy` only — `--filter` does not match against `idx`. To find a specific index, post-filter with `jq` (e.g. `mihomoctl rules list --limit 1000 --json | jq '.rules[] | select(.idx==42)'`).

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Snapshot retrieved successfully. |
| 64 | `--limit <= 0`; passing extra positional arguments. |
| 75 | Controller unreachable, network error, or HTTP request timed out. |
| 77 | Auth failed. |

### Performance budget

Pre-1.0 informational target, not contract:

- Snapshot list with default flags: **p95 < 1s**. Cost is dominated by the controller's `/rules` response size; CLI overhead (filter, sort, format) is typically <50ms even on large rule tables.

### Examples

<!-- Verified against r18 source `f2e31e57` cmd_rules.go:75-122 + v03_test.go:12-72. Sample values illustrative. -->

```bash
# Find the rule that matches a specific domain
$ mihomoctl rules list --filter google.com
idx	type	payload	proxy
0	DOMAIN-SUFFIX	google.com	PROXY

# Filter by rule type — list every GEOIP rule
$ mihomoctl rules list --filter GEOIP --json | jq '.rules[] | {payload, proxy}'

# Filter by destination proxy — what routes through DIRECT?
$ mihomoctl rules list --filter DIRECT --json | jq '.total'
38

# Detect truncation (rule table is large; default --limit 50 may cap)
$ mihomoctl rules list --json | jq '{total, returned: (.rules | length)}'
{"total": 234, "returned": 50}

# Show all rules — pass a high cap (limit-as-cap)
$ mihomoctl rules list --limit 1000 --json | jq '.total'
234

# Find rules that route to a specific group
$ mihomoctl rules list --filter PROXY --json | jq '.rules[] | select(.proxy=="PROXY") | "\(.type) \(.payload)"'

# Empty rules table
$ mihomoctl rules list
no rules
$ echo $?
0

# Empty rules table — JSON envelope is still well-formed
$ mihomoctl rules list --json
{
  "total": 0,
  "limit": 50,
  "rules": []
}
```

---

## `mihomoctl providers list`

Snapshot mihomo's **proxy providers** with their current health state. Use this to spot subscription sources that have stale data, are unreachable, or have not yet been health-checked.

> **Scope**: `mihomoctl providers list` is **proxy providers only**. mihomo distinguishes proxy providers (the source of node lists you select from) from rule providers (the source of rule sets the matcher uses). Rule providers live behind a different upstream endpoint and are out of scope for v0.3 — they are tracked for v0.4+ as a separate command tree (`rule-providers list` etc.) so the proxy / rule namespaces stay clean.

### Synopsis

```
mihomoctl providers list \
    [--json] [--endpoint URL] [--secret VALUE] [--timeout DURATION]
```

### Flags

- No command-specific flags in v0.3. Provider counts are typically small (≤10 per deployment); pagination via `--limit` and substring filtering via `--filter` are not exposed. They may be added in a later release if user evidence supports the need; the JSON envelope already reserves the `limit` field for forward-compat.

### Output (human-readable)

<!-- Verified against r18 source `f2e31e57` cmd_providers.go:109 (header) + cmd_providers.go:111 (row format `%s\t%s\t%s\t%s\t%d\t%s\n`) + cmd_providers.go:105-107 (empty case) + cmd_providers.go:150-152 (sort by name ascending). Sample values illustrative. -->

```
name	type	vehicle_type	health	node_count	updated_at
local-pool	Proxy	Inline	unknown	5	
sub-A	Proxy	HTTP	healthy	42	2026-05-07T03:00:00Z
sub-B	Proxy	HTTP	unhealthy	18	2026-05-07T02:48:12Z
```

Tab-separated; sorted by `name` ascending. The `updated_at` column is empty (no `-`, no `null` — empty field) if mihomo has no recorded update timestamp for that provider.

When the snapshot has no proxy providers, the command prints `no proxy providers` (no header line) and exits 0.

### Output (`--json`)

```json
{
  "total": 3,
  "limit": 3,
  "providers": [
    {
      "name": "local-pool",
      "type": "Proxy",
      "vehicle_type": "Inline",
      "health": "unknown",
      "node_count": 5,
      "updated_at": ""
    },
    {
      "name": "sub-A",
      "type": "Proxy",
      "vehicle_type": "HTTP",
      "health": "healthy",
      "node_count": 42,
      "updated_at": "2026-05-07T03:00:00Z"
    },
    {
      "name": "sub-B",
      "type": "Proxy",
      "vehicle_type": "HTTP",
      "health": "unhealthy",
      "node_count": 18,
      "updated_at": "2026-05-07T02:48:12Z"
    }
  ]
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `total` (integer, required) — number of proxy providers in the snapshot.
- `limit` (integer, required) — **effective returned cap / passthrough**. In v0.3 this is always equal to `total` because `providers list` does not paginate; the field is included for envelope parity with `rules list` and `connections list` so a single `jq` template can apply across all three commands. **There is no `--limit` flag on `providers list` in v0.3** — do not script around one. If `--limit` is later added, `limit` will reflect the effective cap.
- `providers` (array, required) — proxy providers, sorted by `name` ascending.
  - `name` (string, required) — provider name as configured in mihomo's `proxy-providers:` block.
  - `type` (string, required) — always `"Proxy"` in v0.3. Retained as a forward-compat namespace signal — if rule providers later get their own command tree, the existing `Proxy` namespace stays unambiguous and scripts can `select(.type=="Proxy")` without re-reading docs.
  - `vehicle_type` (string, required) — provider source as mihomo's controller emits it. Common values include `HTTP` (subscription pulled over HTTP — the most common case for remote subscription URLs), `File` (file-vehicle pull from local disk), `Inline` (proxies declared inline in the mihomo config), and `Compatible` (mixed / fallback). The exact set is whatever mihomo's wire protocol returns; treat as opaque if you encounter a value not listed above. (`Rule` is **not** a possible value — rule providers are out of scope; see the Scope note above.)
  - `health` (string, required) — derived health state. `healthy` if the most recent health check succeeded; `unhealthy` if the most recent health check failed; `unknown` if no health check has run for this provider in the current mihomo session.
  - `node_count` (integer, required) — number of candidate nodes the provider currently exposes.
  - `updated_at` (string, required) — RFC 3339 timestamp of mihomo's most recent subscription / cache update for this provider, or empty string `""` if mihomo has no recorded timestamp. (The CLI emits an empty string here, not `null`; both human and JSON outputs use the same convention.)

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Snapshot retrieved successfully (empty list is still 0). |
| 64 | Passing extra positional arguments. |
| 75 | Controller unreachable, network error, or HTTP request timed out. |
| 77 | Auth failed. |

### Performance budget

Pre-1.0 informational target, not contract:

- Snapshot list with default flags: **p95 < 1s**. Cost is dominated by the controller's `/providers/proxies` response; CLI overhead is typically <30ms.

### Examples

<!-- Verified against r18 source `f2e31e57` cmd_providers.go:96-114. Sample values illustrative. -->

```bash
# Quick health overview
$ mihomoctl providers list
name	type	vehicle_type	health	node_count	updated_at
local-pool	Proxy	Inline	unknown	5	
sub-A	Proxy	HTTP	healthy	42	2026-05-07T03:00:00Z
sub-B	Proxy	HTTP	unhealthy	18	2026-05-07T02:48:12Z

# Find unhealthy providers for triage
$ mihomoctl providers list --json | jq '.providers[] | select(.health=="unhealthy") | .name'
"sub-B"

# Find never-checked providers (implies you have not run providers healthcheck yet)
$ mihomoctl providers list --json | jq '.providers[] | select(.health=="unknown") | .name'
"local-pool"

# Filter to HTTP subscription providers
$ mihomoctl providers list --json | jq '.providers[] | select(.vehicle_type=="HTTP") | {name, node_count}'

# Total candidate nodes across all healthy providers
$ mihomoctl providers list --json | jq '[.providers[] | select(.health=="healthy") | .node_count] | add'
42

# Empty case
$ mihomoctl providers list
no proxy providers
$ echo $?
0
```

---

## `mihomoctl providers healthcheck <name>`

Trigger a server-side health refresh on a Proxy provider and return a provider-level summary including a CLI-local trigger timestamp.

> **Scope**: Proxy providers only. Calling `providers healthcheck` with the name of a rule provider returns exit 66 not-found — rule providers are not in the proxy provider namespace this command operates over. To inspect rule providers, you will need a separate command tree (out of scope for v0.3).

### What this command does (and does not do)

- It **triggers** mihomo's per-provider health-check routine via `/providers/proxies/{name}/healthcheck`. mihomo probes each candidate node in the provider against its configured target URL.
- It then **re-fetches** the provider's snapshot from `/providers/proxies` and returns the post-trigger state — `health` / `node_count` / `updated_at` reflect mihomo's view immediately after the trigger.
- It does **not** return per-node probe results in v0.3. The `results: [{node, latency_ms, status}]` schema standardized for `group delay` (v0.2) is **not** extended to `providers healthcheck`. If you need per-node latency for a provider's nodes, use `group delay <group>` on a proxy group that includes those nodes; future per-node probe variants of this command may be added in a later release behind ADR-0008 mutation safety review.

### Side-effect note

`/providers/proxies/{name}/healthcheck` is **not pure read-only**. mihomo probes each candidate node and may update its internal selection state for automatic-policy groups that depend on this provider. We treat this as a safe controller-side refresh rather than a user-visible mutation: it does not change global mode, does not modify configuration, and emits no `previous` field. Source: <https://wiki.metacubex.one/en/api/>.

### Synopsis

```
mihomoctl providers healthcheck <name> \
    [--json] [--endpoint URL] [--secret VALUE] [--timeout DURATION]
```

### Arguments

- `<name>` — required. Name of an existing **proxy** provider. Use `mihomoctl providers list` to discover available names. Passing a rule provider name returns exit 66 (rule providers are not in the proxy provider namespace).

### Flags

- No command-specific flags in v0.3. The trigger endpoint does not accept per-node URL or timeout overrides; the CLI does not enumerate per-node calls (option A — see the design note above).

### Output (human-readable)

<!-- Verified against r18 source `f2e31e57` cmd_providers.go:140-141 (single-line tab-separated 7 fields). Sample values illustrative. -->

```
sub-A	Proxy	HTTP	healthy	42	2026-05-07T03:00:00Z	2026-05-07T03:00:01Z
```

A single tab-separated line in the order: `provider`, `type`, `vehicle_type`, `health`, `node_count`, `updated_at`, `triggered_at`. The two timestamps are deliberately on the same line so the difference between mihomo's `updated_at` (its cache time) and `triggered_at` (when this CLI invocation fired the refresh) is visible side-by-side.

> No header line is emitted. If you script against this output, parse the tab positions, not a header — the field order matches the JSON object below.

### Output (`--json`)

```json
{
  "provider": "sub-A",
  "type": "Proxy",
  "vehicle_type": "HTTP",
  "health": "healthy",
  "node_count": 42,
  "updated_at": "2026-05-07T03:00:00Z",
  "triggered_at": "2026-05-07T03:00:01Z"
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `provider` (string, required) — provider name passed on the command line.
- `type` (string, required) — always `"Proxy"` in v0.3 (forward-compat namespace signal — see `providers list` schema).
- `vehicle_type` (string, required) — same enum as `providers list` (`HTTP` / `File` / `Inline` / `Compatible` etc.; treat unknown values as opaque). See [`providers list` schema](#mihomoctl-providers-list) for the full discussion.
- `health` (string, required) — `healthy` / `unhealthy` / `unknown` after the refresh.
- `node_count` (integer, required) — number of candidate nodes the provider exposes after the refresh.
- `updated_at` (string, required) — RFC 3339 timestamp of mihomo's most recent subscription / cache update for the provider. **This is mihomo's clock**, not the time of this call.
- `triggered_at` (string, required) — RFC 3339 UTC timestamp set by **mihomoctl** at the moment it issued the trigger call. **This confirms the refresh fired in this invocation**, distinct from `updated_at`. Use this when scripting "did my trigger actually fire" assertions; do not assume `updated_at` advanced just because the trigger returned (mihomo's refresh may be async — `triggered_at` is the only stable signal of "I fired the trigger now").

> **Schema note**: this output is exactly a `providers list` row plus `triggered_at`. Scripts can build a single `jq` filter that handles both shapes by treating `triggered_at` as optional. The output deliberately does **not** include a `results: [...]` per-node array — see "What this command does (and does not do)" above.

### Two-step daily-use flow

`providers healthcheck <name>` is most useful as the **first** step in a refresh-and-inspect loop:

1. `mihomoctl providers healthcheck <name>` — fire the trigger; the returned summary tells you the provider responded and gives you `triggered_at` for proof.
2. `mihomoctl providers list` — fetch the global view; if you triggered multiple providers, this is where you compare their post-refresh `health` / `node_count` / `updated_at` side-by-side.

Don't expect `providers healthcheck` to give you per-node latencies — that is `group delay`'s job, on a proxy group that includes the nodes you want to probe.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Trigger fired and post-refresh summary returned (empty provider with `node_count: 0` is still 0). |
| 64 | Missing `<name>` argument; passing extra positional arguments. |
| 66 | `<name>` is not an existing proxy provider (includes the case where `<name>` is a rule provider). |
| 75 | Controller unreachable, network error, or HTTP request timed out. |
| 77 | Auth failed. |

### Performance budget

Pre-1.0 informational target, not contract:

- Default invocation: **p95 < 3s** end-to-end. The cost is one trigger call plus one `/providers/proxies` snapshot (two HTTP round-trips + mihomo's trigger handling). The trigger itself returns quickly because mihomo runs the per-node probes asynchronously.

### Examples

<!-- Verified against r18 source `f2e31e57` cmd_providers.go:116-143 + v03_test.go:165-182 (error wording verbatim). Sample values illustrative. -->

```bash
# Trigger a refresh and read the result
$ mihomoctl providers healthcheck sub-A
sub-A	Proxy	HTTP	healthy	42	2026-05-07T03:00:00Z	2026-05-07T03:00:01Z

# Did the trigger actually fire? Check triggered_at
$ mihomoctl providers healthcheck sub-A --json | jq -r '.triggered_at'
2026-05-07T03:00:01Z

# Trigger refresh on an unhealthy provider, then inspect global view
$ mihomoctl providers healthcheck sub-B --json | jq '.health'
"unhealthy"
$ mihomoctl providers list --json | jq '.providers[] | select(.health=="unhealthy") | .name'
"sub-B"

# Rule provider name → exit 66 (rule providers are out of scope)
$ mihomoctl providers healthcheck some-rule-provider
Error: proxy provider "some-rule-provider" not found, available: local-pool, sub-A, sub-B
$ echo $?
66

# Empty provider (node_count: 0) → exit 0 with the summary
$ mihomoctl providers healthcheck empty-pool --json
{
  "provider": "empty-pool",
  "type": "Proxy",
  "vehicle_type": "Inline",
  "health": "unknown",
  "node_count": 0,
  "updated_at": "",
  "triggered_at": "2026-05-07T03:00:01Z"
}
```

---

## `mihomoctl connections watch`

Stream live mihomo connection events over a WebSocket until interrupted. Use this when you want to watch traffic flow as it happens — e.g. during a node switch, or when chasing why a particular destination is being routed through an unexpected proxy. For a one-shot snapshot, use [`connections list`](#mihomoctl-connections-list) instead.

### Synopsis

```
mihomoctl connections watch [--filter PATTERN] [--interval DURATION]
                            [--no-reconnect] [--max-reconnect-attempts N]
                            [--json] [--endpoint URL] [--secret VALUE]
```

### Transport and auth

- **WebSocket only.** mihomoctl opens `ws(s)://<endpoint>/connections` and reads NDJSON event frames from the upstream stream. There is no HTTP-polling fallback in v0.4; if your environment cannot reach the WebSocket port, the command fails fast.
- **Authorization header at handshake.** When `MIHOMOCTL_SECRET` is set, the WebSocket handshake sends `Authorization: Bearer <secret>` as an HTTP header. mihomoctl does **not** use the `?token=` query-string variant — query tokens leak into proxy / load-balancer access logs.
- The global `--timeout` applies to the **initial** WebSocket connect and to per-read heartbeat stalls. It does **not** bound the lifetime of the stream itself; an open stream with quiet traffic is healthy and is not torn down on `--timeout`.

### Flags

| Flag | Default | Notes |
| --- | --- | --- |
| `--filter <pattern>` | _none_ | **CLI-local** substring match against `host` / `destination` / `source` / `rule` (OR semantics, case-insensitive). The CLI receives mihomo's full upstream event stream and filters client-side. mihomo's `/connections` WebSocket does not accept a server-side filter parameter, so dropping events upstream is not possible in v0.4. |
| `--interval <duration>` | _server default_ | Passthrough to mihomo's `?interval=ms` query parameter — controls how often the upstream pushes connection-table snapshots. |
| `--no-reconnect` | off | If the WebSocket disconnects, exit immediately instead of retrying. |
| `--max-reconnect-attempts <N>` | `100` | Cap on consecutive reconnect attempts after disconnect. `0` means unbounded — for supervised long-running sessions. The counter resets to 0 every time a successful event is received. |

### Output (human-readable)

Each tick from mihomo's WebSocket emits a snapshot of currently-active connections (filtered if `--filter` is set). Each snapshot prints a header row followed by one row per matching connection:

```
received_at	started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:05Z	2026-05-07T03:00:01Z	192.168.1.10:55320	1.1.1.1:443	tcp	DOMAIN-SUFFIX,cloudflare.com,PROXY	PROXY > JP-01	890/2456
```

Snapshots that match no connections (after filter) print `no matching active connections` instead of an empty table — non-empty so the user knows the watcher is alive. The header repeats on every non-empty snapshot, mirroring tools like `watch -d ls`.

> **Important — these are snapshots, not per-event open/close pushes.** mihomo's `/connections` WebSocket exposes a periodic poll of the entire connection table; mihomoctl forwards each poll as one event. Connection-level transitions (open / close) are inferred by diffing successive snapshots client-side; v0.4 does not expose `event_action: "open"|"close"` because the upstream wire does not. Per the implementation-truth pattern (same path as v0.3 `vehicle_type=HTTP`), the docs reflect what the wire actually emits.

### Output (`--json`)

NDJSON — **one JSON object per line**, with two `type` discriminants:

```jsonc
{"type":"event","data":{
  "event_action": "snapshot",                              // always "snapshot" in v0.4 — see human output note above
  "received_at":  "2026-05-07T03:00:05Z",                  // CLI-local RFC 3339 UTC, when the snapshot was received
  "connections":  [ /* same shape as `connections list --json .connections[]`, post-filter */ ]
}}
{"type":"error","error":{"code":"<stable_id>","category":"<sysexits_aligned>","message":"<human>","details":{ /* optional */ }}}
```

Readers MUST branch on `.type` before reading further fields — assuming every line is an event will misread the error rows. Error rows use the cross-cutting envelope shape; see [JSON error envelope schema](#json-error-envelope-schema) below.

A snapshot whose post-filter `.data.connections` array would be empty is **not emitted under `--json`** — the CLI suppresses empty events in JSON mode so scripts do not see "heartbeat" rows. (Human mode keeps the `no matching active connections` line so the operator gets the heartbeat signal.)

The streaming-NDJSON envelope (`{type, data | error}`) is itself a stable contract surface intended to survive into the v1.0 lock — the discriminant key `type` and the two values `event` / `error` will not change without a `Breaking` changelog entry. `event_action` is currently always `"snapshot"`; if mihomo upstream later exposes a per-event push API, additional values (e.g. `"open"` / `"close"`) may be added without a `Breaking` entry — readers that branch on `event_action` should treat unknown values as opaque-but-stable.

### Reconnect behaviour

If the WebSocket disconnects after at least one event was received, the CLI retries with capped exponential backoff (250 ms → 500 ms → 1 s → 2 s → 5 s, capped at 5 s, plus a 0–100 ms jitter). After every successful event the failure counter resets to 0, so steady streams that briefly hiccup do not march toward the cap.

When `--max-reconnect-attempts N` is exceeded (default `100`, `0` = unbounded for supervised sessions), the CLI emits a final NDJSON line `{"type":"error","error":{"code":"reconnect_exhausted","category":"tempfail","message":"mihomo websocket reconnect attempts exhausted","details":{"attempts":N}}}` and exits `75`.

When `--no-reconnect` is set, the first disconnect exits immediately — exit `75` for a controller / network failure, exit `77` if the WebSocket handshake itself was rejected for auth.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | Stream ended cleanly via Ctrl-C, EOF, or `EPIPE` (downstream pipe closed, e.g. `\| head`). |
| 64 | Usage error (invalid `--interval` / negative `--max-reconnect-attempts` / non-empty argument). |
| 73 | Non-EPIPE write error on stdout while emitting a frame. |
| 75 | WebSocket initial-connect failure, mid-stream upstream loss with reconnect attempts exhausted (`reconnect_exhausted`), or `--no-reconnect` triggered by a controller-side disconnect (`websocket_disconnected`). |
| 77 | Authorization failed at the WebSocket handshake — secret missing, wrong, or rejected by mihomo. |

`EPIPE` is intentionally **success** (exit 0): users routinely pipe `connections watch` to `head`, `tee`, or `grep` and expect the stream to terminate quietly when the consumer closes early.

### Examples

```bash
# Stream until Ctrl-C
$ mihomoctl connections watch
received_at	started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:05Z	2026-05-07T03:00:01Z	192.168.1.10:55320	1.1.1.1:443	tcp	DOMAIN-SUFFIX,cloudflare.com,PROXY	PROXY > JP-01	890/2456
2026-05-07T03:00:05Z	2026-05-07T03:00:00Z	192.168.1.10:55321	8.8.8.8:443	tcp	GEOIP,US,PROXY	PROXY > JP-01	1234/5678
^C

# Filter to a particular destination, peek at the first 5 lines including the header
$ mihomoctl connections watch --filter google.com | head -5
received_at	started_at	source	destination	network	rule	chains	up/down
2026-05-07T03:00:05Z	2026-05-07T03:00:01Z	192.168.1.10:55322	142.250.80.46:443	tcp	DOMAIN-SUFFIX,google.com,PROXY	PROXY > JP-01	256/1024

# JSON mode — one event object per line, suppress empty snapshots
$ mihomoctl connections watch --filter google.com --json
{"type":"event","data":{"event_action":"snapshot","received_at":"2026-05-07T03:00:05Z","connections":[{"started_at":"2026-05-07T03:00:01Z","source":"192.168.1.10:55322","destination":"142.250.80.46:443","network":"tcp","rule":"DOMAIN-SUFFIX,google.com,PROXY","chains":["PROXY","JP-01"],"up":256,"down":1024}]}}

# Supervised long-running session — unbounded reconnect, with structured error rows on every flap
$ mihomoctl connections watch --max-reconnect-attempts 0 --json | jq -c '. | select(.type=="error")'
{"type":"error","error":{"code":"websocket_disconnected","category":"tempfail","message":"mihomo websocket stream disconnected: ..."}}
```

---

## `mihomoctl dns query <domain>`

Resolve a domain through mihomo's internal DNS resolver. Use this to check what mihomo's DNS subsystem actually returns — useful when fakeip is involved, when a `geosite:` rule depends on a particular CNAME chain resolving a specific way, or when you want to confirm cache state after `cache clear dns`.

### Synopsis

```
mihomoctl dns query <domain> [--type RECORD_TYPE] [--json] [--endpoint URL] [--secret VALUE]
```

### Arguments

| Argument | Notes |
| --- | --- |
| `<domain>` | The domain to resolve. Required. |

### Flags

| Flag | Default | Notes |
| --- | --- | --- |
| `--type <record_type>` | `A` | DNS record type to query. Passthrough to mihomo. Common values: `A`, `AAAA`, `CNAME`, `TXT`, `MX`. Invalid types are rejected by mihomo with an HTTP 400, which the CLI maps to exit `64`. |

### Output (human-readable)

A single header line shows the domain, query type, and DNS status; subsequent lines are the answer records (one per line, tab-separated):

```
example.com	A	NOERROR
name	type	ttl	data
example.com	A	300	93.184.216.34
```

If `answers` is empty (e.g. `NXDOMAIN`, or a record type the domain has no record for), the output is:

```
example-nonexistent.invalid	A	NXDOMAIN
no answers
```

— still **exit 0** in both cases (the DNS protocol layer answered, even if the answer set is empty).

### Output (`--json`)

```jsonc
{
  "domain": "example.com",
  "query_type": "A",
  "status": "NOERROR",
  "answers": [
    {"name": "example.com", "type": "A", "ttl": 300, "data": "93.184.216.34"}
  ]
}
```

Schema (current — pre-1.0, may change between minor releases per [Stability and JSON contract](#stability-and-json-contract)):

- `domain` (string, required) — the queried domain, echoed back. Lowercased; mihomoctl normalizes.
- `query_type` (string, required) — the queried record type. Always one of mihomo's accepted types; lowercased input is normalized.
- `status` (string, required) — upstream DNS RCODE name (`NOERROR`, `NXDOMAIN`, `SERVFAIL`, `REFUSED`, …). The names come from miekg/dns's capitalized constants; mihomoctl's contract is the upstream string verbatim.
- `answers` (array, required, may be empty) — zero or more answer records.
  - `name` (string, required) — record owner name. Lower-cased / normalized snake_case from miekg/dns's capitalized field.
  - `type` (string, required) — record type as a string.
  - `ttl` (integer, required) — record TTL in seconds, as returned by upstream.
  - `data` (string, required) — the rdata payload as a string. For `A` / `AAAA` this is the textual address; for `CNAME` / `MX` / `TXT` it is the textual rdata.

**`NXDOMAIN` is exit 0**, with `status: "NXDOMAIN"` and `answers: []`. The DNS protocol returning "no such name" is a successful query at the CLI layer — sysexits convention is that "command succeeded" includes "the protocol-layer answer happened to be negative". Conflating `NXDOMAIN` with exit 66 / 75 would force callers into ambiguous error handling. Compare with `proxy set <missing_node>`, which IS exit 66 because there the CLI-level lookup truly failed.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | DNS query succeeded (any RCODE, including `NXDOMAIN`). |
| 64 | Missing `<domain>` argument, empty `<domain>`, empty `--type` value, or unsupported `--type` (CLI pre-flight rejection — mihomo never sees the request). |
| 75 | Controller unreachable, network failure, or controller-side error from the resolver. |
| 77 | Auth failed. |

### Examples

```bash
# Default A-record query
$ mihomoctl dns query google.com
google.com	A	NOERROR
name	type	ttl	data
google.com	A	300	142.250.80.46

# AAAA via --type
$ mihomoctl dns query google.com --type AAAA
google.com	AAAA	NOERROR
name	type	ttl	data
google.com	AAAA	300	2607:f8b0:4005:80c::200e

# JSON output for scripts
$ mihomoctl dns query google.com --json | jq -r '.answers[].data'
142.250.80.46

# NXDOMAIN — exit 0, empty answers
$ mihomoctl dns query example-nonexistent.invalid
example-nonexistent.invalid	A	NXDOMAIN
no answers
$ echo $?
0

# Invalid --type — pre-flight rejection, exit 64
$ mihomoctl dns query google.com --type NOT_A_TYPE
Error: --type NOT_A_TYPE is not a supported DNS record type; expected one of A, AAAA, ...
$ echo $?
64
```

---

## `mihomoctl cache clear <fakeip|dns|all>`

Flush mihomo's internal DNS / fake-IP caches. Three subcommands, each a **low-impact ephemeral mutation**: side effects are limited to short-term DNS lookups being repeated against upstream resolvers, with no impact on active connections, mihomo configuration, or the running service.

### Why this is "low-impact" (and what that means for safety guards)

mihomoctl distinguishes two mutation classes:

- **Low-impact ephemeral mutation** (this command): the controller side state is recreated on demand, no user-visible service interruption, no rollback semantics needed. No `--yes` / `--dry-run` / non-TTY refusal guard. The classification is locked by [ADR-0008](./adr/0008-mutation-safety.md) §4 Consequences 4.
- **High-impact mutation** (v0.5: `config reload`, `service restart`): tears down or replaces controller-side state visible to users. Requires a confirmation token (the resource identifier echoed back), `--yes` for non-interactive use, and a non-TTY refusal path by default. Tracked under the same ADR-0008 family.

`cache clear` does **not** require any of the high-impact safety guards and must not be cited as precedent for relaxing them on the v0.5 batch.

### Synopsis

```
mihomoctl cache clear <fakeip|dns|all> [--json] [--endpoint URL] [--secret VALUE]
```

A bare `cache clear` (no subcommand) exits `64` with a usage error.

### Subcommands

| Subcommand | What it does | Side effect |
| --- | --- | --- |
| `cache clear fakeip` | Flushes mihomo's fake-IP map. | Existing fake-IP-mapped connections continue using their mapped IPs until they close; new connections re-allocate fake IPs. |
| `cache clear dns` | Flushes mihomo's DNS resolver cache. | Subsequent DNS queries that would have hit the cache go to upstream resolvers instead. |
| `cache clear all` | Calls both endpoints in sequence. Partial failure is reported under the JSON error envelope — successful sub-flushes are **not** rolled back. | Same combined side effects. |

### Output (human-readable)

For `cache clear fakeip` and `cache clear dns`, a single confirmation line on success:

```
flushed mihomo dns cache
```

For `cache clear all`, one tab-separated line per sub-cache (no header). Successful sub-caches show `<name>\tcleared`; failed ones show `<name>\tfailed\t<error_message>` followed by the central renderer's stderr error envelope:

```
fakeip	cleared
dns	failed	cannot connect to mihomo at http://127.0.0.1:9090; ...
```

### Output (`--json`)

Per-subcommand success shape:

```json
{
  "cache": "dns",
  "cleared": true
}
```

`cache clear all` partial failure does NOT emit a success-shaped object with `cleared: false` and a sibling `results: [...]`. Instead, partial failure routes through the cross-cutting [JSON error envelope](#json-error-envelope-schema) with per-cache results nested in `details`:

```json
{
  "error": {
    "code": "cache_partial_failure",
    "category": "tempfail",
    "message": "cache clear partially failed",
    "details": {
      "cache": "all",
      "results": [
        {"cache": "fakeip", "cleared": true},
        {"cache": "dns",    "cleared": false, "error": {"code": "controller_unavailable", "category": "tempfail", "message": "..."}}
      ]
    }
  }
}
```

The successful sub-cache (`fakeip` here) is **not** rolled back — `cache clear` is intentionally non-transactional, consistent with mihomo's underlying endpoints. Scripts handling `cache clear all` should branch on the top-level envelope (presence of `.error`) and read `.error.details.results[]` to act per sub-cache.

### Exit codes

| Code | Condition |
| --- | --- |
| 0 | All requested caches cleared successfully. |
| 64 | Bare `cache clear` (no subcommand), or unknown subcommand. Leaf subcommands also exit 64 if given any positional argument (`cache clear fakeip foo` → exit 64). |
| 75 | `cache clear fakeip` / `cache clear dns`: controller unreachable. `cache clear all`: at least one sub-cache failed (partial or full); the JSON failure body's `code` is `cache_partial_failure` and the per-cache results are in `details.results`. |
| 77 | Auth failed. |

### Examples

```bash
# Single sub-cache
$ mihomoctl cache clear dns
flushed mihomo dns cache

# Both — full success
$ mihomoctl cache clear all
fakeip	cleared
dns	cleared

# Both — partial failure (controller dropped between sub-flushes)
$ mihomoctl cache clear all
fakeip	cleared
dns	failed	cannot connect to mihomo at http://127.0.0.1:9090; ...
Error: cache clear partially failed; retry the failed cache target
$ echo $?
75

# Same partial failure under --json — top-level envelope, results in details
$ mihomoctl cache clear all --json
{
  "error": {
    "code": "cache_partial_failure",
    "category": "tempfail",
    "message": "cache clear partially failed; retry the failed cache target",
    "details": {
      "cache": "all",
      "results": [
        {"cache": "fakeip", "cleared": true},
        {"cache": "dns",    "cleared": false, "error": {"code": "controller_unavailable", "category": "tempfail", "message": "..."}}
      ]
    }
  }
}

# Bare `cache clear` — usage error
$ mihomoctl cache clear
Error: cache clear requires fakeip, dns, or all
$ echo $?
64
```

---

## JSON error envelope schema

> v0.4 adds a single shared shape for every `mihomoctl --json` failure path. v0.1–v0.3 commands also adopt it under the v0.4 release as a `Breaking` change — see the [CHANGELOG](./CHANGELOG.md) `[0.4.0]` `Breaking` entry for the migration note.

When any command fails under `--json`, mihomoctl emits a single object with one top-level field, `error`:

```json
{
  "error": {
    "code": "<stable_identifier>",
    "category": "<sysexits_aligned>",
    "message": "<human_readable>",
    "details": { }
  }
}
```

Field roles:

- `code` (string, required) — **stable identifier**. Part of the contract. New codes may be added; existing codes will not be renamed without a `Breaking` changelog entry. Branch on this in scripts.
- `category` (string, required) — **sysexits-aligned coarse classification**. Part of the contract. Maps 1:1 to the CLI exit code (see table below).
- `message` (string, required) — human-readable rendering. **Documentation for recognition only — not part of the contract.** Wording may shift between minor 0.x releases without a `Breaking` entry. Do not match on `message` content.
- `details` (object, optional) — per-error-class structured data. Shape is documented per code; absent when no per-class structure is needed.

### Categories

The seven categories (per [ADR-0010](./adr/0010-error-envelope-schema.md) v0.2 canonical) are sysexits-aligned and map 1:1 to exit codes:

| Category | Exit code | Meaning | Example codes |
| --- | --- | --- | --- |
| `usage` | 64 | Invalid command, flag value, or argument shape. | `invalid_limit`, `missing_argument`, `unsupported_command_scope` |
| `not_found` | 66 | A named resource (group, node, proxy provider) does not exist. **DNS `NXDOMAIN` is NOT in this category** — see [`dns query`](#mihomoctl-dns-query-domain). | `group_not_found`, `provider_not_found`, `rule_not_found` |
| `software` | 70 | Unclassified internal CLI bug or unexpected upstream payload shape. | `decode_failed`, `internal_error`, `invalid_controller_payload` |
| `system` | 71 | Local OS / environment failure before a controller request completes. | `config_unreadable`, `path_unavailable` |
| `cant_output` | 73 | Non-EPIPE failure writing requested output. EPIPE is exit 0 for pipe-friendly streaming commands. | `output_failed` |
| `tempfail` | 75 | Controller unreachable, mid-flight network failure, streaming initial-connect timeout, or streaming reconnect cap exhausted. | `controller_unavailable`, `controller_timeout`, `controller_http_error` |
| `noperm` | 77 | Authorization failed at the controller — secret missing or wrong. Includes WebSocket handshake auth failure for `connections watch`. | `auth_failed` |

v0.5 adds an eighth category, `mutation_aborted`, for the `config reload` / `service restart` family under ADR-0008 — not present in v0.4 envelopes.

### Code naming convention

`error.code` is a `snake_case` stable machine identifier. There is **no required `_error` suffix** — codes are named after what failed (e.g. `auth_failed`, `controller_unavailable`, `cache_partial_failure`), not after their category. Scripts should match `error.code` for stable behavior; treat unfamiliar codes as opaque-but-stable, since new codes may be added in any minor release without a `Breaking` entry (existing codes will not be renamed without one).

### Initial code set (v0.4)

The following codes are guaranteed by the v0.4 central renderer (verified against source `8152db12` on the canonical repo). Scripts can switch on any of them.

| Code | Category | Used by |
| --- | --- | --- |
| `usage_error` | `usage` | Default for any usage-class failure that does not carry a more specific code (bad flag, bad subcommand, missing argument). |
| `not_found` | `not_found` | Default for any not-found failure (group / node / proxy provider lookups via `validateSelection`, `validateDelayGroup`, `validateProxyProvider`). |
| `software_error` | `software` | Default for unclassified software / decode failures. |
| `system_error` | `system` | Default for local OS / environment failures (read-only since v0.4 has no command paths that hit this category in normal flows; reserved for `man <directory>` filesystem failures). |
| `output_error` | `cant_output` | Default for non-EPIPE write failures on stdout (`cannot write JSON output`, `cannot create man page directory`, streaming output errors). |
| `controller_unavailable` | `tempfail` | Default for any controller dial / network / 5xx failure (`mihomo.Error.ErrUnavailable` mapped through `mapErr`). |
| `auth_failed` | `noperm` | Default for controller 401 / 403 (`mihomo.Error.ErrAuth`); also used by `connections watch` WebSocket handshake failure. |
| `internal_error` | `software` | Renderer-level fallback for non-`cliError` errors that escape command handlers (i.e. unexpected panics or boundary violations). |
| `cache_partial_failure` | `tempfail` | Emitted by `cache clear all` when at least one but not all sub-caches failed; carries `details: {cache: "all", results: [...]}` per the [`cache clear`](#mihomoctl-cache-clear-fakeip-dns-all) section. |
| `websocket_disconnected` | `tempfail` | Emitted by `connections watch` when the WebSocket disconnects mid-stream. Under `--json` this surfaces as a streaming error line (`{"type":"error","error":{"code":"websocket_disconnected", ...}}`); under `--no-reconnect` it is also the exit-causing error. |
| `reconnect_exhausted` | `tempfail` | Emitted by `connections watch` when consecutive reconnect attempts exceed `--max-reconnect-attempts` (default 100). Carries `details: {attempts: <N>}`. |

[ADR-0010](./adr/0010-error-envelope-schema.md) v0.2 §Decision lists additional reserved codes per category for forward use (e.g. `group_not_found` / `provider_not_found` / `rule_not_found` for finer-grained `not_found` discrimination, `controller_timeout` / `controller_http_error` for finer-grained `tempfail` discrimination, `decode_failed` / `invalid_controller_payload` for `software` discrimination, `config_unreadable` / `path_unavailable` for `system` discrimination); commands may upgrade from the default codes to these specific codes in any minor release without a `Breaking` entry. Treat unfamiliar codes as opaque-but-stable.

### Streaming envelope

`connections watch --json` emits **two** kinds of NDJSON line per [its own section](#mihomoctl-connections-watch): `{type:"event", data:{...}}` for connection events and `{type:"error", error:{...}}` for streaming-stage errors. The error-line payload uses the same shape documented above; the wrapping `{type, ...}` discriminator is a streaming-only addition and does not appear on non-streaming commands.

### Side-by-side example: pre-envelope vs envelope

```jsonc
// v0.3 status --json on controller-unreachable (PRE-envelope):
{"error": "cannot connect to mihomo at http://127.0.0.1:9090; check --endpoint or whether external-controller is enabled: dial tcp ..."}

// v0.4 status --json on controller-unreachable (envelope):
{"error": {
  "code": "controller_unavailable",
  "category": "tempfail",
  "message": "cannot connect to mihomo at http://127.0.0.1:9090; check --endpoint or whether external-controller is enabled: ...",
  "details": {"endpoint": "http://127.0.0.1:9090"}
}}
```

Migration: scripts that grep `stderr` for human wording continue to work — the envelope's `message` field carries the same wording. Scripts that previously read the top-level `error` string under `--json` MUST switch to reading `.error.message` (for human display) or `.error.code` / `.error.category` (for branching). Pin to an exact `0.x.y` if your script depends on the prior pre-envelope failure shape.

---

## Common errors

> Error wording is **not** part of the contract. The exit codes and the broad error category are stable; the strings below are documented for recognition only and may shift between minor releases. v0.4 introduces stable `code` / `category` fields — see [JSON error envelope schema](#json-error-envelope-schema). Scripts should branch on exit code or on `.error.code` (under `--json`), not on string content.

| Error | Exit | Fix |
| --- | --- | --- |
| `cannot connect to mihomo at <url>; check --endpoint or whether external-controller is enabled: ...` | 75 | mihomo not running, wrong endpoint, or firewall. Check `external-controller:` in mihomo config; check `MIHOMOCTL_ENDPOINT`. |
| `missing/invalid secret; set MIHOMOCTL_SECRET, or use --secret <value> if you accept the leak risk` | 77 | Auth failed. Set `MIHOMOCTL_SECRET` to mihomo's `secret:` value, or unset if config has no secret. |
| `invalid mode "<arg>"; expected rule, global, or direct` | 64 | Use one of the three valid modes. |
| `group "XYZ" not found, available: ...` | 66 | Run `mihomoctl proxy list` for valid group names. |
| `node "XYZ" not found in group "PROXY", available: ...` | 66 | Run `mihomoctl proxy list`; check the candidate list for that group. |
| `group "X" type "Y" does not support delay test, applicable types: URLTest, Selector, Fallback, LoadBalance` | 64 | `Direct` and `Reject` groups have no candidates to probe; only the four listed types accept `group delay`. |
| `--delay-timeout must be > 0` | 64 | Use a positive duration (e.g. `--delay-timeout 3s`). |
| `--limit must be > 0` | 64 | Use `--limit 1` or higher. To return everything, pass a large cap (the snapshot is bounded by mihomo's connection / rule table). |
| `proxy provider "X" not found, available: ...` | 66 | Run `mihomoctl providers list` for valid proxy provider names. **Note**: rule provider names also trigger this error in v0.3 — rule providers are out of scope and not present in the proxy provider namespace, so the lookup fails as not-found. |

## Out of scope for v0.4

These ship in later versions or are permanently excluded:

- **Per-node probe results from `providers healthcheck`** — the `results: [{node, latency_ms, status}]` schema standardized for `group delay` in v0.2 is not extended to `providers healthcheck`. Backlog for a future release, behind ADR-0008 mutation safety contract (per-node enumeration pulls in concurrency / partial failure / fan-out semantics that exceed read-only scope). Until then, use `group delay` on a proxy group that includes the nodes you need to probe.
- **Rule providers** (`/providers/rules` namespace) — out of scope. Tracked as a separate command tree (`rule-providers list` / `rule-providers healthcheck` etc.) for a future release so the proxy / rule namespaces stay clean. The `providers list` and `providers healthcheck` commands are explicitly **proxy providers only**.
- `connections watch` aggregation, sticky header, interactive table — backlog; v0.4 ships the simple per-event NDJSON stream.
- `connections watch --filter` upstream-passthrough variant — v0.4 implements `--filter` as **CLI-local substring filter only** because mihomo's `/connections` WebSocket does not expose a server-side filter parameter (Otto pre-research). If a future mihomo release adds a server-side filter, mihomoctl will lock the matching wire-passthrough path per the implementation-truth pattern (same path as PRD-0003's `vehicle_type=HTTP` enum lock).
- `dns query` reverse lookup (PTR record from IP) — backlog; user-evidence triggered.
- `dns query --server`, `source`, and `latency_ms` fields — backlog. Otto pre-research did not confirm a wire interface in mihomo's external-controller for these; v0.4 does not prescribe shape that is not actually present upstream.
- `cache clear` partial-flush variants (specific domain / TTL-based) — backlog; v0.4 ships the three coarse-grained subcommands only.
- DNS cache stats (`cache stats` / `cache list`) — backlog; debug-class, user-evidence triggered.
- `connections kill` — terminate an active connection. Backlog; mutation-safety design pending under ADR-0008 family.
- `group force-reselect` / `group delete-fixed-selection` — mutation variants of `group delay`. Backlog.
- Batch `group delay` over multiple groups — backlog; the existing single-group form composes via shell loops.
- `connections list` custom sort / order — backlog. Default `started_at desc` is locked.
- `providers update` (subscription pull / re-fetch URL contents) — backlog; behind ADR-0008 mutation-safety contract.
- `rules match <host>` (lookup which rule a host hits) — backlog; user evidence triggered.
- Remote HTTPS controller / TLS endpoint configuration — backlog.
- Multi-profile config (managing multiple mihomo instances) — backlog.
- `config reload`, `restart`, `upgrade`, `geo update` — tracked for **v0.5** as a high-impact mutation batch under ADR-0008 (confirmation-token + dry-run + non-TTY refusal). v0.4's `cache clear` is intentionally classified as **low-impact** ephemeral mutation and does **not** require those guards; it must not be cited as precedent for the v0.5 batch.
- Service management (`start` / `stop` / `install` of mihomo itself) — permanently out of scope.
- TUI mode — backlog.
- mihomoctl-side configuration files — permanently out of scope.

## See also

- [Quickstart](./quickstart.md) — install, first-switch, and the v0.2 test/switch/verify loop.
- [CHANGELOG](./CHANGELOG.md) — release history. Read before upgrading minor 0.x releases (pre-1.0 contract may shift; every change is documented).
- ADR-0002 — architecture and exit-code contract source of truth. Amended by ADR-0004 (pre-1.0 stability framework), ADR-0005 (cobra dependency boundary in v0.1.2+), and ADR-0006 (conditional package boundaries: 2-command band stays single-package, 3+ commands triggers `internal/cli` split).
- [mihomo external-controller docs](https://wiki.metacubex.one/en/api/) — upstream API reference. Note that mihomoctl normalizes some shapes (e.g. flat `connections` schema vs upstream nested `metadata`); the public mihomoctl JSON contract is what this reference documents, not the upstream wire format.
