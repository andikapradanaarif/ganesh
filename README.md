# Ganesh M1 — reproducible source-cited passage retrieval

Rust implementation of the M1 milestone of the
[personal second memory roadmap](docs/personal-second-memory-roadmap.md):
given frozen public articles and a question, return up to three passages that
collectively support the answer — with citations the caller can open to the
exact source revision — or explicit `insufficient_context` when the corpus
lacks the evidence. Fully offline, zero model-provider calls.

## Layout

- `crates/ganesh-core` — shared library: snapshot importer (SQLite), passage
  segmentation, lexical retrieval (SQLite FTS5/BM25 over stemmed tokens),
  abstention, citation issuance and resolution, access check.
- `crates/ganesh-server` — REST adapter: `POST /v1/search`,
  `GET /v1/citations/:token`, `GET /v1/health`.
- `crates/ganesh-mcp` — thin local MCP bridge (stdio JSON-RPC) exposing
  `search_context` and `open_citation`, forwarding to the REST service.
- `crates/ganesh-bench` — offline benchmark harness and dev coverage probe.
- `fixtures/corpus` — frozen public corpus: 8 Wikipedia article snapshots
  (CC BY-SA 4.0, see `fixtures/corpus/NOTICE.md`), manifest with publishers,
  URLs, revisions, retrieval dates, permissions and SHA-256 digests.
- `fixtures/questions` — frozen question keys, scoring rules and the freeze
  record (`FREEZE.md`): development set `dev.v1.json` (14 questions) and
  held-out set `heldout.v1.json` (30 questions).

## Replaying the benchmark (clean checkout)

```
cargo build --release --workspace
cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions/heldout.v1.json \
  --report out/heldout-report.md
```

One invocation creates a fresh temporary store, imports the pinned corpus with
digest verification, rebuilds the search index, starts the REST service on a
loopback port, executes every question through the real REST endpoint and the
real MCP stdio bridge, opens every returned citation through both adapters,
scores against the frozen key and writes the report with measured costs.
Everything runs offline against the frozen inputs.

### Executable gate

`run` defaults to held-out gate mode. It reads `scoring-spec.json` beside the
questions file (or `--scoring-spec`), enforces the frozen denominators and the
top-three passage limit, prints a final machine-readable JSON result, and uses
these exit codes:

- `0` — PASS
- `1` — FAIL
- `3` — INCOMPLETE (empty/partial roster, malformed adapter response, transport
  interruption, missing or inconsistent scoring specification)
- `2` — command-line usage error

An incomplete or malformed run can never produce PASS. Development mode keeps
the same diagnostics without enforcing the held-out roster:

```
cargo run --release -p ganesh-bench -- run --mode dev \
  --questions fixtures/questions/dev.v1.json \
  --report out/dev-report.md
```

The controlled gate fixtures demonstrate all three exit paths offline:

```
for f in pass fail incomplete; do
  cargo run --release -p ganesh-bench -- run \
    --questions fixtures/gate/$f.questions.json \
    --report out/gate-$f.md
  echo "$f exit=$?"
done
```

Expected: PASS exits 0, FAIL exits 1, INCOMPLETE exits 3. The PASS fixture is
also the quick end-to-end example: d01 is a successful cited lookup (Java's
originators and original name) and d12 is an explicit `insufficient_context`
result (the corpus does not state a default Java maximum heap size).

## Manual use

```
# import the corpus into a store (once)
cargo run --release -p ganesh-bench -- run --questions fixtures/questions/dev.v1.json --report /tmp/x.md  # see above; or use your own importer call
```

The service reads a pre-built SQLite store:

```
target/release/ganesh-server --db /path/to/ganesh.db --bind 127.0.0.1:4315
curl -s -X POST localhost:4315/v1/search -H 'content-type: application/json' \
  -d '{"query": "Who created Node.js?"}'
```

The MCP bridge attaches to the running service:

```
GANESH_REST_URL=http://127.0.0.1:4315 target/release/ganesh-mcp   # stdio JSON-RPC
```

## Verification

```
cargo test --release --workspace
```

Key properties under test: the segmenter reproduces the frozen reference
passage enumeration byte-for-byte; FTS5 availability; importer idempotency
(reimport same revision/bytes is a no-op, reused revision ID with different
bytes is an error — both also exercised by the harness, which reimports every
article and requires an already-present result).
