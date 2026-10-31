# S0 local capture primitive

Task: [#22](https://github.com/andikapradanaarif/ganesh/issues/22), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). The [roadmap capture contract](personal-second-memory-roadmap.md#build-41-narrative-origin-file-provenance-and-capture-completeness) and [execution sheet](s0-execution-sheet.md) remain authoritative.

`ganesh capture --spool DIR` accepts a normalized metadata object on stdin and returns a local receipt after storage checks succeed. It is an S0 development primitive, not a working client integration, HTTP/MCP endpoint, full hook adapter or transcript reader. A separate prototype [Crush tool-hook command](s0-crush-hooks.md) reduces a narrow native input subset into this core. No hooks are installed and discovery still reports every capture profile unsupported.

## Input

Only five fields are admitted:

| Field | Contract |
| --- | --- |
| `schema_version` | Integer `1`. |
| `event_id` | Stable lowercase UUID-shaped identity with a UUID version/variant. An adapter must create and preserve this identity before retrying. |
| `session_id` | Lowercase UUID-shaped local session identity; actual client mapping is a later adapter requirement. |
| `event` | `session_start`, `session_end`, `turn_stop`, `tool_attempt`, `tool_result`, `subagent_start`, `subagent_stop`, `pre_compact`, `post_compact`, or `instructions_observed`. |
| `outcome` | `observed`, `succeeded`, `failed`, `denied`, `interrupted`, or `unknown`. |

Unknown/missing/duplicate keys, malformed identities, unsupported values, invalid encoding, extra JSON objects and input over 64 KiB fail with static errors. There is no place for raw text, paths, commands, tool results, credentials, owner declarations or successful-validation claims. Values are preserved exactly rather than normalized into another identity.

This structural admission is not a general-purpose secret scanner or proof of legitimate UUID provenance. A future client adapter must reduce and scrub its raw inputs before this boundary and pass the installed schema/configuration fixtures. **Do not install this command directly as a raw client hook.** Its receipts are development output, not agent context or hook approval/decision output.

The following uses synthetic demonstration IDs only:

```sh
go build -o bin/ganesh ./cmd/ganesh
printf '%s\n' '{"schema_version":1,"event_id":"00000000-0000-4000-8000-000000000001","session_id":"00000000-0000-4000-8000-000000000002","event":"tool_result","outcome":"failed"}' \
  | bin/ganesh capture --spool .local/s0/day-one/capture-smoke
```

The spool must be a dedicated directory with an existing parent. Existing private directories can be used, but the directory itself, lock and record files must not be symlinks or group/other-accessible. Storage/locking is currently implemented for macOS/Linux; other platforms fail closed.

## Receipt, retries and storage

Every record/receipt is `state: captured_local`, `origin: unknown`, `coverage: incomplete`. Success means process-restart durability with that local volume intact. It does not mean server acceptance, remote protection, complete capture, clean origin or availability for reuse.

Writes use a private 0700 directory, 0600 files, a process lock, a synced temporary file and non-overwriting hard-link publication. The spool directory and its existing parent are synced before a receipt is returned. A retry of the exact metadata recovers the original timestamp and returns `duplicate: true`. Reusing the event ID with changed metadata fails; it never overwrites the first record.

A missing receipt is ambiguous: publication may already have happened. Retry the same envelope/ID, including after restart. A corrupt existing record fails rather than being silently replaced. Reads have byte limits. Directory enumeration uses batches of at most 64 entries and checks the context per batch/entry; there is no total file-count cap. A large directory can therefore consume the collector deadline and fail closed. A full spool rejects new records. The initial cap is 64 MiB, counting stored records and orphaned temporary files; it is conservative about duplicate hard links. Exact retry recovery remains possible at capacity.

Process-exit and cancellation fixtures test write/file-sync/publication/directory-sync/parent-sync/receipt boundaries. These are not power-loss certification or a guarantee for every filesystem. Use local storage supporting the required locking, link and sync operations. Same-user filesystem compromise is not prevented or remotely attested.

## Deadline and measured scope

On the observed macOS/CGO build, the command obtains its own creation time through the OS process-info API. The 750 ms budget therefore includes loader/Go runtime startup, parsing, lock wait and local storage. It is not restarted in handler logic. A clock failure or unsupported process-clock build fails closed; the rest of the CLI continues to work with CGO disabled.

Stdin waiting is deadline-bounded. Storage checks cancellation between operations and before issuing a receipt. A filesystem syscall may overrun or be interrupted by the host; the command cannot guarantee real-time scheduling or complete an fsync under every failure. Ambiguous or failed attempts do not produce a successful receipt. Network, reconciliation, upload and model work are absent from the command.

The short-lived CLI arms a process watchdog around receipt output using the remainder of the same process-start budget. If stdout is blocked, it exits with status **124** without attempting another potentially blocked diagnostic write. It does not alter shared stdout descriptor flags. The record may already be durable; missing or partial receipt bytes are ambiguous and require exact retry. This process-exit guard is specific to the CLI, not a reusable in-process writer cancellation API. OS scheduling remains outside a hard real-time guarantee.

`process_start_to_receipt_ms` is sampled after durable storage and before serialization/output; it is a receipt-ready measurement, not proof of write completion or consumer receipt. Parent-observed elapsed time includes stdout delivery and process exit.

The client host's one-second hook timeout, overall/shared SessionEnd budget and competing hooks are **not** configured or tested here. The required installed-client smoke matrix, exact read/file-version binding, ancestry/non-tool inventories, closure/reconciliation and provenance are not implemented by these enum events. Even `session_end` is only a declared event, not a closure certificate.

## Lifecycle and next integration

This prototype has no uploader, automated orphan cleanup/expiry, installation control synchronization, device purge acknowledgment or source mapping. Interrupted attempts can leave private metadata temporary files that consume capacity. It is not ready for unattended production capture or real client hook enrollment. Those contracts remain explicit later integration gates; the cap is not an offline-duration guarantee.

Record prototype latency separately from actual per-installation evidence. Fresh-process synthetic measurements do not establish cold-cache startup time, real hook coverage, client honesty, the full P01/P03-D suites or automatic discovery-to-reuse. Keep failures/timeouts and unmeasured paths visible. Project and client selection remain pending; no real project/transcript content is admitted by this implementation task.

### Prototype measurement, 2026-10-31

On macOS amd64 with CGO enabled, the compiled collector from `9c86aaf` completed 50 synthetic captures in fresh processes with zero failures. Nearest-rank process-start-to-durable-receipt latency was p95 **73.201 ms**, p99 **390.825 ms**. Parent-observed elapsed latency over all attempts was p95 **74.567 ms**, p99 **392.075 ms**. The measurement watchdog was two seconds; it did not simulate a client host timeout.

Page-cache state was uncontrolled and cold-start time was not independently measured. No installed hook, SessionEnd budget or competing hook was exercised. Detailed non-content attempt metrics are retained outside git in `.local/local-capture-22-latency.json`. This small prototype sample is not a supported-profile result or performance SLO.

The owner-provided PR review reported a separate fresh-process smoke around **465 ms**. That observation is not part of the 50-sample dataset and has not been reproduced here. Slower or cold hosts may exhaust the unchanged budget; real installed-profile measurements remain required before enabling capture. Event/session identity relationships and real client mappings remain adapter responsibilities; UUID shape alone does not establish them. A blocked stdin reader may remain until this short-lived process exits, so the command is not an in-process capture service.
