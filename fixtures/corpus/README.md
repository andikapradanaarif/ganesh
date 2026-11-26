# Corpus fixture provenance

The snapshots, passage enumerations, selection record, and manifest in this
directory are frozen evidence inputs. They must not be regenerated as part of
the benchmark.

`fetch.py`, `sample.py`, and `segment.py` are the historical scripts that
produced the frozen v1 fixtures. They are retained only as provenance for how
those bytes and passage enumerations were created. They are not part of the
Rust workspace, are not invoked by the build, importer, service, adapters, or
benchmark harness, and must not be used to repair or alter frozen fixtures.

The supported implementation and replay path is Rust/Cargo. If a future
fixture requires regeneration or extension, write that tooling in Rust and
freeze a new manifest rather than editing this frozen corpus in place.
