# Changelog

All notable changes to mihomoctl are documented here. Format: [Keep a Changelog 1.1](https://keepachangelog.com/en/1.1.0/), versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added

- `mihomoctl system ping` and `mihomoctl system version` — read-only controller runtime checks for v1.0. Both commands use `GET /version`: `system ping` treats a successful version response as the controller health check and reports endpoint plus version; `system version` prints the controller version directly. Both commands support `--json`. (PR <TBD>, PRD-0010 §2.4, inventory v0.3, ADR-0015 §5.2)
- `mihomoctl groups list` and `mihomoctl groups get <name>` — read-only proxy group inspection commands for v1.0, backed by `GET /group` and `GET /group/{name}`. (PR <TBD>, PRD-0010 §2.4, inventory v0.3, ADR-0015 §5.4)
- `mihomoctl proxy-providers list`, `mihomoctl proxy-providers get <name>`, `mihomoctl proxy-providers update <name>`, `mihomoctl proxy-providers healthcheck <name>`, `mihomoctl rule-providers list`, and `mihomoctl proxy delay <node>` — provider/probe command surfaces for v1.0. Routes: `GET /providers/proxies`, `GET /providers/proxies/{provider}`, `PUT /providers/proxies/{provider}`, `GET /providers/proxies/{provider}/healthcheck`, `GET /providers/rules`, and `GET /proxies/{name}/delay`. `proxy-providers update` is a medium-impact mutation per ADR-0014 §4.2; `proxy-providers healthcheck` is a low-impact side-effecting probe with no confirmation prompt, `--yes` as a no-op, and `--dry-run` rejected. `rule-providers` intentionally exposes only `list` in this batch; ADR-0015 §5.5 does not allow invented rule-provider get/healthcheck commands. (PR <TBD>, PRD-0010 §2.4, inventory v0.3, ADR-0014 §4.2, ADR-0015 §5.4/§5.5)
- `mihomoctl connections watch --limit <N>` — cap snapshot output at N connections per emit. Applies to all three output paths (TTY in-place table / non-TTY tab-separated row append / `--json` NDJSON literal); `--limit 0` or omitted = unlimited (sustained from v0.4 default behavior). G1 fix per Iris vika UX catch — `connections list` had `--limit` but `connections watch` did not. (PR <TBD>, PRD-0009 §2.5)
- `--columns <a,b,c>` flag on `mihomoctl connections list`, `mihomoctl groups list`, `mihomoctl proxy-providers list`, and `mihomoctl rule-providers list` — selects which human-output columns to render and in what order. The full per-command column set and the default subset are listed in each command's `--help`. `--json` is unchanged (always full schema). Unknown column names exit `64` with the available list. CEO direct UX request msg=d9e32328 ("字段不可选"). (PR <TBD>, PRD-0010 §3.6 polish dim 3)
- Charm/lipgloss table renderer is now reused for `connections list`, `groups list`, `proxy-providers list`, and `rule-providers list` human output on TTY (sustaining the v0.4.1 `connections watch` TUI path). Non-TTY pipe output remains tab-separated and byte-stable for `grep`/`awk`/`cut` scripts; `--json` is untouched. Per CEO directive msg=83cdc23d ("不要自己写，自己计算") — command implementations no longer hand-compute column widths. (PR <TBD>, PRD-0010 §3.6 polish dim 3)

### Breaking

- `mihomoctl group delay <name>` is removed for v1.0 and replaced by `mihomoctl groups delay <name>`. The upstream endpoint is unchanged (`GET /group/{name}/delay`); only the CLI namespace moves to the plural `groups` command family per ADR-0015 §5.4. Migration: replace `mihomoctl group delay X` with `mihomoctl groups delay X`.
- Removed the old `mihomoctl providers` command tree. Use `mihomoctl proxy-providers list` for `providers list`, `mihomoctl proxy-providers healthcheck <name>` for `providers healthcheck <name>`, and `mihomoctl proxy-providers update <name>` for the v1.0 provider update mutation. Rule providers use the separate `mihomoctl rule-providers list` namespace. No long-term `providers` alias is frozen per ADR-0015 §5.5/§7.

### Changed

- `mihomoctl connections watch` human output now renders an **in-place table on TTY** (alternate-screen + clear+home + sticky header showing `received_at` + match count + filter, redraw on each upstream snapshot, Ctrl-C clean exit 0 with cursor restored). **Non-TTY / pipe** behavior is unchanged: tab-separated row append per snapshot, byte-identical to v0.4 (`connections watch | grep ...` scripts unaffected). **`--json` mode** is unchanged: NDJSON literal regardless of TTY. TTY detection via `isatty(stdout)` + `TERM=dumb` fallback. Sober styling per CEO directive (single-color, no gradient/alarm-color, light border, bold for emphasis instead of color, `NO_COLOR` env var honored). Implementation uses `github.com/charmbracelet/lipgloss/table` as a render-only helper (no Bubble Tea event loop import); `internal/streaming` supervisor / Ctrl-C / EPIPE / NDJSON paths are unchanged. (PR <TBD>, PRD-0009 §2.3)
- `mihomoctl connections` command tree gains alias `conns` (cobra `Aliases:["conns"]`). `mihomoctl conns` / `mihomoctl conns list` / `mihomoctl conns watch` are equivalent to their `connections` counterparts; the long form is retained (no deprecation). All docs / help / error text continue to use `connections` as the canonical name; cobra surfaces the alias automatically in `--help`. (PR <TBD>, PRD-0009 §2.2)
- `mihomoctl connections list` and `mihomoctl connections watch` human output now formats the `up`/`down` columns with **IEC binary units** (`B` / `KiB` / `MiB` / `GiB`, one decimal place; values `< 1024` show as `N B` with no decimal). `--json` `upload_bytes` / `download_bytes` remain `int64` byte counts — JSON contract is unchanged. Implementation lives in `internal/render.FormatBytes`. (PR <TBD>, PRD-0009 §2.4)
- `mihomoctl cache clear` (no subcommand) error wording is now actionable: `cache clear requires a target. Use 'cache clear fakeip', 'cache clear dns', or 'cache clear all'.` (was: `cache clear requires fakeip, dns, or all`). The cobra `Long` text also enumerates the three valid subcommands and includes a usage example block. Exit code `64` (cobra usage error) is unchanged. G4 fix per Iris vika UX catch. (PR <TBD>, PRD-0009 §2.6)

### Internal

- New dependencies: `github.com/charmbracelet/lipgloss/table` (render-only table styling/borders) and `github.com/charmbracelet/x/term` (terminal width detection for bounded 80-column rendering). No Bubble Tea event-loop import. Added to `go.mod` / `go.sum`; both dependencies are confined to `internal/render`, and `internal/cli` / `internal/streaming` / `internal/mihomo` do not import them. (PRD-0009 §2.3 + ADR-0013)
- `internal/render/format_bytes.go` — `FormatBytes(int64) string` helper, IEC binary units, used by `connections list/watch` human formatters. Pure int64 → string conversion, no secret path. (PRD-0009 §2.4)
- `internal/render/connections_table.go` — TTY in-place table renderer for `connections watch`, lipgloss/table-based, sober styling defaults. Honors `NO_COLOR` env var; `TERM=dumb` short-circuits to non-TTY append path. (PRD-0009 §2.3)
- Secret-leak `--help` regression coverage extended to **24 / 24** command surfaces cumulative (v0.4 21 sustained + 3 new alias invocation paths: `conns --help`, `conns list --help`, `conns watch --help`). The command-object count remains 15 leaves (alias is not a new command per cobra `Aliases` semantic). The auth functional regression `TestSecretEnvUsedAtExecution` continues unchanged from v0.2.

### Stability

- v0.4.1 is **additive + human-cosmetic** under [ADR-0011](../docs/adr/0011-contract-freeze.md): no `--json` contract change, no exit-code change, no envelope schema change. The `--limit` flag addition is additive (default unlimited preserves v0.4 behavior); the `conns` alias is backward-compatible (`connections` long form retained); IEC bytes formatting is human-default-only (`upload_bytes` / `download_bytes` `int64` JSON fields are byte-identical); the `cache clear` bare-error wording change is polish (exit code 64 sustained). No `Breaking` migration is required.

## [0.4.0] - 2026-05-07

### Added

- `mihomoctl connections watch` — stream live mihomo connection events over a WebSocket until Ctrl-C. Implements the **streaming pattern** locked in [ADR-0007](../docs/adr/0007-streaming-pattern.md) (canonical `3f123129`): WS-only transport, `Authorization: Bearer <secret>` HTTP header at handshake (**not** `?token=` query param), and supervised reconnect with exponential backoff. Flags: `--filter <pattern>` (CLI-local substring match against `host` / `destination` / `source` / `rule`, OR semantics — **events are filtered client-side after the CLI receives mihomo's full upstream stream; mihomo's `/connections` WebSocket does not accept a server-side filter parameter**); `--interval <duration>` (passthrough to mihomo's `?interval=ms` query, controls upstream push cadence); `--no-reconnect` (exit immediately on disconnect, no retry); `--max-reconnect-attempts <N>` (default `100`; `0` = unbounded for supervised sessions; counter resets on every successful event). With `--json`, output is NDJSON, one object per line, two `type` discriminants: `{"type":"event","data":{...}}` for connection events and `{"type":"error","error":{"code","category","message","details"}}` for streaming-stage errors (envelope shared with ADR-0010 below). Exit codes: `EPIPE` (downstream pipe closed, e.g. `| head`) → `0` (silent success); other write errors → `73`; reconnect attempts exhausted → `75`; `Ctrl-C` → `0`. Global `--timeout` applies to the initial WebSocket connect and per-read heartbeat stall (not to the lifetime of the stream itself). (PR <TBD>, PRD-0004, ADR-0006 stay-in-band, ADR-0007)
- `mihomoctl dns query <domain>` — resolve a domain through mihomo's internal DNS resolver (read-only). Flag: `--type <record_type>` (default `A`; passthrough to mihomo; invalid type is rejected by mihomoctl before contacting the controller and exits `64`). JSON shape `{domain, query_type, status, answers: [{name, type, ttl, data}]}` (4-field envelope; field names are CLI-normalized snake_case from mihomo's upstream miekg/dns capitalized fields). DNS `NXDOMAIN` is **exit 0** with `{"status":"NXDOMAIN","answers":[]}` — sysexits convention: the CLI command succeeded; the DNS protocol layer reporting "no such domain" is not a CLI-level "not found". `--server`, `source`, and `latency_ms` fields are explicitly out of scope for v0.4 (Otto pre-research did not confirm a wire interface; tracked behind user-evidence triggers for v0.5+). (PR <TBD>, PRD-0004)
- `mihomoctl cache clear <fakeip|dns|all>` — flush mihomo's internal caches as a **low-impact ephemeral mutation**. Three subcommands: `cache clear fakeip` flushes the fake-IP map; `cache clear dns` flushes the DNS resolver cache; `cache clear all` hits both endpoints in sequence. Bare `cache clear` (no subcommand) → exit `64` usage error. Side effect: short-term DNS lookups are repeated against upstream resolvers; **active connections, mihomo configuration, and the running service are unaffected**. This classification is intentionally distinct from the v0.5 `reload` / `restart` planned high-impact mutations (ADR-0008 confirmation-token pattern); `cache clear` does not require `--yes` or a confirmation token. JSON shape `{cache, cleared: true}` per subcommand on success. `cache clear all` partial failure is reported under the top-level ADR-0010 envelope with per-cache results in `details` — successful caches are **not rolled back**. (PR <TBD>, PRD-0004, ADR-0008 §4 Consequences 4 explicit classification)
- **JSON error envelope (cross-cutting)**. All `mihomoctl --json` failure paths now share a single envelope locked in [ADR-0010](../docs/adr/0010-error-envelope-schema.md) (canonical `9c414ffd`):

  ```json
  {
    "error": {
      "code": "<stable_identifier>",
      "category": "<sysexits_aligned>",
      "message": "<human_readable>",
      "details": { ... }
    }
  }
  ```

  `code` and `category` are part of the contract; `message` is documentation-for-recognition and may evolve between minor 0.x releases without bumping the contract. The seven categories (per [ADR-0010](../docs/adr/0010-error-envelope-schema.md) v0.2 canonical) are sysexits-aligned: `usage` (64), `not_found` (66), `software` (70), `system` (71), `cant_output` (73), `tempfail` (75), and `noperm` (77). v0.5 adds the `mutation_aborted` code under category `usage` (exit 64) for the `config reload` / `service restart` family — the category enum remains the seven listed above; new categories require an ADR-0010 amendment. See [reference § JSON error envelope schema](../docs/reference.md#json-error-envelope-schema) for the full code/category contract and a side-by-side example showing a v0.3 `status --json` failure rendered in the new envelope alongside a v0.4 `cache clear all --json` partial failure.

### Breaking

- **JSON error envelope retroactive align across v0.1–v0.3**. The v0.4 envelope above is **retrofitted to all 11 commands shipped in v0.1–v0.3** (`status`, `mode get`, `mode set`, `proxy list`, `proxy set`, `group delay`, `connections list`, `rules list`, `providers list`, `providers healthcheck`, plus root error rendering). `--json` failure output for these commands now emits the `{error: {code, category, message, details?}}` envelope; previously each command rendered its own ad-hoc error string under `--json`. **Migration**: scripts that grep `stderr` for human error wording continue to work — the envelope `message` field is the same wording. Scripts that parse the JSON failure body MUST switch to reading `.error.code` (stable) or `.error.category` (stable) rather than positional fields. **Pin to an exact `0.x.y`** if your script depends on the prior pre-envelope failure shape; per the pre-1.0 stability framework ([ADR-0004](../docs/adr/0004-pre-1.0-stability-framework.md)) breaking changes are allowed across 0.x minor versions with a migration note, which this entry serves as. Affected commands' migration examples are listed in [reference § JSON error envelope schema](../docs/reference.md#json-error-envelope-schema). (ADR-0010, PR <TBD>)

### Schema notes

- `connections watch --json` NDJSON envelope is **two-discriminant**: `{"type":"event"}` rows carry connection events, `{"type":"error"}` rows carry streaming-stage errors using the same shape as the cross-cutting JSON error envelope. A reader that consumes `connections watch --json` MUST branch on `.type` before reading further fields — assuming every line is an event will misread the error rows. (ADR-0007 v0.2 §Event contract)
- `dns query --json` `status` is the upstream DNS RCODE name (`NOERROR`, `NXDOMAIN`, `SERVFAIL`, …); CLI exit code follows sysexits semantics independently — `NOERROR` and `NXDOMAIN` are both exit 0 (the query itself succeeded), while controller-unreachable / `SERVFAIL` route to controller-error categories per the envelope above.
- `cache clear all --json` partial failure: top-level envelope is the standard error shape; per-cache results live in `details: {cache: "all", results: [{cache, cleared, error?}]}`. Successful caches are not rolled back — the operation is intentionally non-transactional, consistent with mihomo's underlying endpoints.

### Internal

- Central error rendering moved to `internal/render` — single `render.RenderError(err, jsonMode bool)` entry point shared by all commands, replacing per-command ad-hoc rendering and keeping the ADR-0010 envelope out of cobra command handlers. (PR <TBD>, ADR-0010, ADR-0013)
- `internal/streaming` owns reconnect/backoff, NDJSON emission, EPIPE handling, single-emission streaming errors, and the 1-slot latest-snapshot/drop-old flow-control helper required by ADR-0013.
- `internal/mihomo` client gains `Connections.Watch(ctx, opts)` (WebSocket transport), `DNS.Query(ctx, domain, queryType)`, `Cache.FakeIPFlush(ctx)`, `Cache.DNSFlush(ctx)`, and `Cache.ClearAll(ctx)`. Client-layer streaming types stay named `WatchEvent`; the ADR-0010 presentation envelope stays in `internal/render` / `internal/streaming`.
- Secret-leak `--help` regression coverage extended to **21 / 21** command surfaces cumulative (v0.3's 12 + v0.4's 9 new: `connections watch --help`, `dns --help`, `dns query --help`, `cache --help`, `cache clear --help`, `cache clear fakeip --help`, `cache clear dns --help`, `cache clear all --help`, plus the new envelope error wording's `details` redaction guard). The auth functional regression `TestSecretEnvUsedAtExecution` continues unchanged from v0.2.

### Stability

- v0.4 introduces two new contract surfaces (the streaming NDJSON envelope from ADR-0007 and the cross-cutting JSON error envelope from ADR-0010). Both are **stable identifiers** (`type`, `code`, `category`) intended to survive into v1.0; surrounding wording and `details` payloads remain documentation-for-recognition under the pre-1.0 two-phase rule.
- The v1.0 contract-lock mechanism is now defined: [ADR-0011](../docs/adr/0011-contract-freeze.md) (canonical `81dda8c5`) introduces a JSON contract manifest and golden contract test, scheduled to land in v0.8. v0.4 ships under the same pre-1.0 two-phase rule as prior releases — the manifest and golden test do **not** retroactively freeze v0.4 shapes.
- mihomoctl is still pre-1.0. Scripts depending on the v0.4 envelopes (streaming NDJSON, JSON error envelope, `cache clear` shape) should pin to an exact `0.4.x` and read this file before upgrading.

[Unreleased]: https://github.com/vika2603/mihomoctl/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/vika2603/mihomoctl/compare/release/v0.3.0...v0.4.0

## [0.3.0] - 2026-05-07

<!--
Release artifacts (Iris vika acceptance PASS, msg=d9907e47):
- source r18 commit f2e31e57ab3cffeaf32e438ca032676a29506155
- source SHA256 445df94b5ec3024c0ada9fbe9e631c4d84b9298ed87a54cb2b7a8db1db2c9c58
- platform binaries (linux/amd64, linux/arm64, darwin/amd64): attachment IDs pending Iris release-attachment publish (TBD: backfill when published)
- temp public source repo (used during attachment 5xx workaround): https://github.com/vika2603/mihomoctl-r18-1778115158.git (commit f2e31e57)
-->

### Added

- `mihomoctl rules list` — snapshot of mihomo's matching rules. Flags: `--limit <n>` (default `50`, must be `>= 1`; `--limit 0` is exit 64); `--filter <pattern>` (substring match against `type`, `payload`, or `proxy`, OR semantics, case-insensitive). JSON shape `{total, limit, rules: [{idx, type, payload, proxy}]}`; `total` is the count after filter and **before** `--limit` truncation, so scripts can detect when output was capped (same envelope semantics as `connections list`). The per-rule `idx` field is the rule's evaluation order in mihomo's matcher (0-based); it is the only field that ranks a rule against its peers. Sorted by `idx` ascending (the order mihomo's matcher applies). (PR <TBD>, PRD-0003, ADR-0006 3+ command band)
- `mihomoctl providers list` — snapshot of mihomo's **proxy providers** with current health state. JSON shape `{total, limit, providers: [{name, type, vehicle_type, health, node_count, updated_at}]}`. The `limit` field is a passthrough equal to `total` — `--limit` is **not** exposed as a flag in v0.3 (provider counts are typically small, ≤10). The `type` field is always `"Proxy"` in v0.3 (the namespace is proxy providers only); it is retained as a forward-compat namespace signal — Rule providers use a different upstream endpoint and are out of scope for v0.3. The `vehicle_type` field is whatever mihomo's controller emits — common values include `HTTP` (subscription pulled over HTTP), `File`, `Inline`, and `Compatible`; treat unknown values as opaque. Sorted by `name` ascending. (PR <TBD>, PRD-0003)
- `mihomoctl providers healthcheck <name>` — trigger mihomo-side health refresh on a Proxy provider and return a provider-level summary. JSON shape `{provider, type: "Proxy", vehicle_type, health, node_count, updated_at, triggered_at}` (7 fields = `providers list` row + `triggered_at`). The `triggered_at` field is an RFC 3339 UTC timestamp set by mihomoctl at trigger time (CLI-local, **not** read from mihomo); it confirms the refresh fired in this invocation, distinct from `updated_at` which is mihomo's own subscription/cache timestamp. (PR <TBD>, PRD-0003)

### Schema notes

- `providers healthcheck --json` returns a **provider-level summary**, not per-node probe results. The `results: [{node, latency_ms, status}]` schema introduced in v0.2 for `group delay` remains specific to `group delay` — v0.3 does not extend it to `providers healthcheck`. Future per-node probe commands may reintroduce the standardized `results: [...]` shape (tracked behind ADR-0008 mutation safety batch and user-evidence triggers); v0.3 explicitly does not. Read this when scripting against multiple test/probe commands so a single `jq` filter does not assume the shape is shared.
- `rules list --json` `total` is the count after `--filter` is applied but before `--limit` truncation — same envelope semantics as `connections list`. Detect truncation with `total > (.rules | length)`.
- `providers list --json` `limit` is documented as an effective returned cap / passthrough; in v0.3 it is always `= total` since the command does not paginate. Do **not** assume `limit` exists across every mihomoctl JSON envelope — only commands that take `--limit` (or have schema parity with one that does) include it.
- All v0.3 commands follow deterministic ordering: `rules list` by rule index ascending, `providers list` by `name` ascending. Same human and JSON ordering — JSON does not re-sort relative to human output. (Locked alongside the v0.2 `group delay` / `connections list` ordering rules.)

### Scope guard

- `providers healthcheck` is **Proxy providers only**. Calling `providers healthcheck <rule_provider_name>` returns exit 66 `proxy provider "X" not found, available: A, B, C` — rule providers are not in the proxy provider namespace, so the lookup fails as not-found rather than as a usage error. This boundary is locked and will not collapse if rule providers later enter scope (a separate `rule-providers list` / `rule-providers healthcheck` command tree would be added; the existing `providers` tree continues to mean proxy providers).

### Internal

- Package layout escalates per ADR-0006 to the **3+ command default band**. The `cmd/mihomoctl` package shrinks to `main` and root wiring only; CLI tree, output formatting, error mapping, and argument validation move to `internal/cli`. The `internal/mihomo` API client gains `Rules.List(ctx, opts)`, `ProxyProviders.List(ctx)`, and `ProxyProviders.Healthcheck(ctx, name)` methods. No `internal/auth` split (env-first secret + flag override is small enough to live inline; v1.0-full triggers like profiles, multi-controller, TLS trust, and token storage are not in v0.3 scope). (PR <TBD>, ADR-0006)
- `TestSecretEnvDoesNotLeakInHelp` regression extended to cover the **5 new v0.3 help surfaces** (`rules --help`, `rules list --help`, `providers --help`, `providers list --help`, `providers healthcheck --help`), making cumulative coverage **12/12** command surfaces (v0.2's 7 surfaces continue to be guarded). The auth functional regression `TestSecretEnvUsedAtExecution` is unchanged from v0.2 and continues to assert env-first auth still authenticates. Together these double-guard the ADR-0002 secret material boundary across every shipped help surface.

### Stability

- mihomoctl is still pre-1.0. The `--json` shape introduced for `rules list` / `providers list` / `providers healthcheck`, the `--limit` / `--filter` flag semantics on `rules list`, and the `triggered_at` timestamp contract may change between minor 0.x releases per the existing two-phase rule (see [reference § Stability and JSON contract](./reference.md#stability-and-json-contract)). v0.3 is not a contract freeze. Scripts that rely on the shapes above should pin to an exact 0.3.x and read this file before upgrading.

## [0.2.0] - 2026-05-07

### Added

- `mihomoctl group delay <group>` — probe candidate-node latency for a proxy group of type `URLTest`, `Selector`, `Fallback`, or `LoadBalance`. `Direct` and `Reject` groups are rejected at the CLI layer (exit 64). Flags: `--delay-timeout <duration>` (default `5s`, mihomo-side per-probe timeout, distinct from the global `--timeout`); `--url <url>` (default `http://www.gstatic.com/generate_204`). Per-node timeouts come back as `{"latency_ms": null, "status": "timeout"}` data with exit 0; only controller-side failures (unreachable / network / request timeout) exit 75. (PR <TBD>, PRD-0002, ADR-0006 2-command band)
- `mihomoctl connections list` — snapshot of currently-active mihomo proxy connections. Flags: `--limit <n>` (default `50`, must be `>= 1`; `--limit 0` is exit 64); `--filter <pattern>` (substring match against `host`, `destination`, `source`, or `rule`, OR semantics, case-insensitive). JSON shape is a CLI-normalized flat envelope `{total, limit, connections: [...]}`; raw mihomo upstream `metadata` nesting stays inside `internal/mihomo` and is not part of the public CLI contract. Empty snapshot prints `no active connections` and exits 0. Streaming (`connections watch`) is tracked for v0.4 alongside the JSON error envelope (ADR-0007 + ADR-0010). (PR <TBD>, PRD-0002)
- Global `--timeout <duration>` flag (default `5s`) — controls the HTTP request timeout for controller calls. For `group delay`, the request timeout is auto-extended to at least `delay-timeout + 1s` unless `--timeout` is set explicitly. (PR <TBD>)

### Schema notes

- `group delay --json` introduces a new `results: [{node, latency_ms, status}]` schema with per-node test metadata. This is **a separate schema, not an extension of the v0.1 `candidates: []string`** field used by `proxy list` / `status`. Don't confuse the two when writing `jq` queries against multiple commands.
- `connections list --json` uses CLI-normalized flat per-connection objects. The top-level envelope is `{total, limit, connections: [...]}`; `total` is the count after filtering but **before** `--limit` truncation, so scripts can detect when output was capped.
- `group delay --json` `results[]` is sorted by `latency_ms` ascending with `status: "timeout"` rows last; ties break by `node`. `connections list --json` `connections[]` is sorted by `started_at` descending; ties break by `id`. Both orderings are deterministic and locked.

### Fixed

- Secret leak in `--help` output. Prior to this release, when `MIHOMOCTL_SECRET` was set in the environment, cobra rendered the env value as the `--secret` flag's default in `--help` text on every command. Any user running `--help` and pasting the output to docs / debug logs / CI artifacts / GitHub issues would expose their controller secret. Fix: cobra's `--secret` default is now always `""`; the env value is read only at command-execution time when `--secret` was not provided. Env-first authentication and the `--secret` override are both unchanged. (PR <TBD>, ADR-0002)

### Internal

- Two regression tests guard the ADR-0002 secret-material boundary, in addition to existing tests:
  - `TestSecretEnvDoesNotLeakInHelp` — sets `MIHOMOCTL_SECRET` to a canary token and asserts the canary substring does not appear in `--help` output for root, `group --help`, `group delay --help`, `connections --help`, `connections list --help`, `status --help`, `proxy --help`, `proxy set --help`, `mode --help`, `mode set --help`.
  - `TestSecretEnvUsedAtExecution` — runs `status` against an httptest controller that requires `Authorization: Bearer <secret>`, with the secret only in env and no `--secret` flag, proving env-first auth still actually authenticates.
- Together these double-guard the boundary: leak test alone proves "not exposed", functional test alone proves "still works"; both together prove the env-first contract is intact end-to-end. (PR <TBD>, ADR-0002)
- `internal/mihomo` client gains `GroupDelay(ctx, name, opts) (map[string]int, error)` and `ListConnections(ctx) (ConnectionsSnapshot, error)`. Group-type validation for `delay` reuses the existing `ListProxies` call rather than adding a separate `GetType` round-trip. Package boundary unchanged: `cmd/mihomoctl` + `internal/mihomo` only — v0.2 sits in the ADR-0006 2-command band, no `internal/cli` split.

### Stability

- mihomoctl is still pre-1.0. The `--json` shape introduced for `group delay` and `connections list`, the `--delay-timeout` / `--filter` / `--limit` flag names, and any new exit codes can change between minor 0.x releases per the existing two-phase rule (see [reference § Stability and JSON contract](./reference.md#stability-and-json-contract)). v0.2 is not a contract freeze.

## [0.1.3] - 2026-05-07

### Removed

- `mihomoctl-man.1` artifact generated by the hidden `man <directory>` command. Its content was the root help text, not man-page documentation — a misleading file for downstream packaging. Build/packaging consumers should use cobra's standard man-page output (`mihomoctl.1` plus per-subcommand pages), which `man <directory>` continues to emit. (PR <TBD>)

### Internal

- Regression test guarding the ADR-0005 boundary "no cobra `completion` command": asserts `completion` is absent from root help and that `mihomoctl completion --help` exits 64. Prevents accidental flips of `CompletionOptions.DisableDefaultCmd` on cobra upgrades. No user-visible change. (PR <TBD>)

## [0.1.2] - 2026-05-07

### Changed

- CLI parsing migrated to [cobra](https://github.com/spf13/cobra). Hand-rolled command tree, flag registration, `--help` text, and global-flag splitting are removed; cobra handles them. User-facing behavior is unchanged: same five commands, same global flags accepted in any position, same JSON shape, same exit codes. Cobra's default `completion` command is disabled — shell completions are out of scope per ADR-0005. (PR <TBD>, ADR-0005)
- Man page (`mihomoctl(1)`) is now autogenerated via cobra's `GenManTree()` and ships with the binary instead of being hand-written. Content reflects the cobra-managed help tree, kept in sync with the binary build by definition. A hidden `mihomoctl man <directory>` generator command exposes the autogen for build/release pipelines and stays out of user-facing help. (PR <TBD>, ADR-0005)

## [0.1.1] - 2026-05-07

Source-only release. v0.1.0 binaries continue to apply — no rebuilt binaries are shipped because runtime behavior is byte-for-byte identical to v0.1.0.

### Changed

- Internal package layout: `cmd/mihomoctl` split into `main.go` / `help.go` / `commands.go` / `output.go` / `errors.go` / `validate.go`, all under the same package, for maintainability and faster diff review. No user-facing surface change — JSON shape, exit codes, command tree, flags, and human output are unchanged from v0.1.0. (PR <TBD>)
- Test surface strengthened: per-command JSON shape assertions (`status`, `mode get`/`set`, `proxy list`/`set`); error-path coverage by classification (usage / controller 5xx / network error / timeout / decode failure); `splitGlobalFlags` edge cases (`--flag=value` and `--flag value` forms, mid-command flag position, short-form `-s value`, missing values). Tests are anchored on the contract surfaces documented in the [reference](./reference.md) — JSON shape, exit codes, error classification — not on text wording or ordering. (PR <TBD>)

## [0.1.0] - 2026-05-07

### Added

- Initial release. Five commands targeting the SSH-into-server use case where mihomo's web UI is too slow:
  - `mihomoctl status` — show mihomo version, current mode, and selected node per proxy group.
  - `mihomoctl mode get` — print the current mode (`rule`, `global`, `direct`).
  - `mihomoctl mode set <rule|global|direct>` — switch mode and echo the new state.
  - `mihomoctl proxy list` — list selectable proxy groups and their nodes; mark the current selection.
  - `mihomoctl proxy set <group> <node>` — select a node in a group and echo the new state.
- Auth: `MIHOMOCTL_SECRET` env var (recommended); `--secret` flag exists as an explicit override and is documented with shell-history / process-list leak warnings. (PR <TBD>, ADR-0002)
- Endpoint configuration: `MIHOMOCTL_ENDPOINT` env or `--endpoint` flag; default `http://127.0.0.1:9090`. (PR <TBD>)
- `--json` flag on every command; the output shape is documented in the [reference](./reference.md) and follows the two-phase stability rule (see [Stability](#stability) below). (PR <TBD>)
- Exit-code contract following sysexits-style conventions (canonical source: ADR-0002):
  - `0` OK / `64` usage error / `66` not found (group or node) / `70` software error / `71` system error / `73` cannot create output / `75` temporary failure (controller unreachable, network timeout) / `77` permission/auth error.
- Error mapping: clear, user-actionable error strings for unreachable controller, 401 auth failure, missing groups, and missing nodes — never a raw HTTP status. (PR <TBD>)
- Platforms shipped: Linux amd64, Linux arm64, macOS amd64. macOS arm64 and Windows amd64 are tracked for v0.2 build pipeline. Linux amd64 is the canonical target. (PR <TBD>)

### Documentation

- Quickstart (`docs/cli/mihomoctl/quickstart.md`) — install + first node switch + mode switching in under 5 minutes.
- Reference (`docs/cli/mihomoctl/reference.md`) — every command, flag, exit code, JSON schema, and example.
- Man page (`mihomoctl(1)`) — generated from CLI help, ships with the binary.

### Stability

mihomoctl follows a two-phase stability rule (see [reference § Stability and JSON contract](./reference.md#stability-and-json-contract); set by ADR-0002 per CEO direction 2026-05-07):

- **Pre-1.0 (0.x — current).** mihomoctl is experimental. The `--json` output shape, exit-code values, command tree, and flag names **may change between any two minor releases**. Every breaking change is listed under a `Breaking` heading in the affected release with a one-line migration note. **No major version bump is required during 0.x.** Pin to an exact 0.x.y in scripts and read this file before upgrading.
- **v1.0 onward.** The public surface above locks. Breaking changes after v1.0 require both a `Breaking` entry here and a major version bump.
- Adding optional JSON fields, new commands, new flags, or new exit codes that don't replace existing ones is non-breaking in either phase.
- Human-readable output is **never** part of the contract — don't script against it.

### Out of scope (v0.1 → tracked for v0.2+)

- `connections` management.
- `group delay` (latency testing).
- `rules list` / `providers list/update/healthcheck`.
- `dns query`, `cache flush`.
- Remote HTTPS controller / multi-profile config.
- TUI mode.

### Out of scope (permanent)

- `config reload`, `restart`, `upgrade`, `geo update` (high-impact mutations; if introduced, require dry-run + explicit confirmation + risk prompts).
- Service management (`start`/`stop`/`install` of mihomo itself).
- mihomoctl-side configuration files.

[0.3.0]: <TBD release URL>
[0.2.0]: <TBD release URL>
[0.1.3]: <TBD release URL>
[0.1.2]: <TBD release URL>
[0.1.1]: <TBD release URL>
[0.1.0]: <TBD release URL>

<!--
Release artifacts (until the public repo lands; tracked by Iris in docs-index):

- v0.2.0 source: Slock attachment `cca0a3f6-c277-40f5-a85e-3982c1a4c9c9` (r14, SHA256 `1979a71b0ae2d7be9b02d31019e932d16d33f6aba662cd3d6d038f5d986689b6`)
- v0.2.0 binaries:
  - linux amd64: `40ddf38b-7a19-4a1e-8ae3-f5b37b9a4bf5` (SHA256 `2aa0cff574ff62969bc4bf6ea29a62af00d89574a743f4688ae7f8ff61b8cd37`)
  - linux arm64: `e97691d1-ddef-4dec-b740-b53fa164991a` (SHA256 `0c0376e5aa24c387cdcb6c774eefda952855f920ef3bb425dd8b043b1e6a7f37`)
  - darwin amd64: `95e984e9-969e-4a3b-ac55-d476e247a4e4` (SHA256 `6687e663ff9b576f092f5ac9fa4afe96a82dff1df0dd0058aea39a2a90068f2e`)
- v0.2.0 source-sync polish (Otto r15, post-release docs-only): source `2f6cfb3e-500f-4c09-afe9-37a5a0c23888` (SHA256 `7a6c617e2dd6a41bea3cbef1d5a880f7534caa03e41e8ccd4d89b785718440ba`) — `connections list --filter` help-text wording corrected from "host, source, or rule" to "host, destination, source, or rule" to match the implementation's actual 4-field OR set; no behavior change. r14 binaries continue to apply unchanged.
- v0.1.2 source: Slock attachment `62e26867-ee5d-40be-8204-5cb74225ec48` (r9, SHA256 `e5fe502eb0fc651611de2c04a0a8e1f801ea44d5e78b7e083313aeb8f29bd99d`)
- v0.1.2 binaries (rebuilt for cobra refactor):
  - linux amd64: `c258837b-e4cb-424a-bf3e-52d108f2c2b0` (SHA256 `118c21be...`)
  - linux arm64: `5c8ed1e3-1bf9-41d8-930b-f7c8a27b1bad` (SHA256 `cad586ca...`)
  - darwin amd64: `6de16865-d271-450a-95a3-141784122926` (SHA256 `51e1526f...`)
- v0.1.1 source: Slock attachment `57233147-46ff-4b0a-ba95-429c85a5ca71` (r7, SHA256 `0253bac5...`)
- v0.1.0 source: Slock attachment `eeb62d71-2fdd-4337-9762-192123b0a978` (r6, SHA256 `da93c4db...`)
- v0.1.0 / v0.1.1 binaries (same — no behavior change in v0.1.1):
  - linux amd64: `5a104465-97b3-40b3-891b-a3244334c0a7` (SHA256 `42250f66...`)
  - linux arm64: `42cdd1ec-278f-40c4-a70e-90e099ef4c71` (SHA256 `aefa04a8...`)
  - darwin amd64: `f9b9cf3b-8a7b-4f1d-b206-032d7c2edaa9` (SHA256 `d2dd1877...`)

PR / release URL placeholders above will be backfilled when the public repo migration lands (tracked alongside ADR-0001 URL migration in adr-index).
-->
