# Single instruction version bound across affected S0 inputs (#142)

**A separate private candidate packet now binds one permitted instruction
version consistently across every affected answer and grading input.** Work
stays in [issue #142](https://github.com/andikapradanaarif/ganesh/issues/142)
under [#124](https://github.com/andikapradanaarif/ganesh/issues/124). This is
consistency evidence on cached material, not task adoption, a baseline
adoption decision, grading acceptance or run authority. Original S0 remains
**0/108**; no model call, target-project read, preflight rerun or new source
inspection occurred.

## Background

The owner-authorized bounded review (#140 evidence) inspected one native
instruction file whose content had drifted after the frozen packet was
prepared. The recorded impact was 30 answer bindings (26 base, four
conditional) across nine tasks and one grading source record. Post-review
overlay work had already rebound the affected binding pins, the view file,
the regenerated source record and the catalog records, but two resolved
surfaces — the baseline-proposal snapshot entry and one task's oracle source
evidence — still pinned the superseded version. The effective input set was
therefore mixed-version, and the existing driver check verified pin
integrity without checking version uniformity.

## What the candidate changes

The private candidate materializes the four resolved input layers into one
flat packet (273 files + manifest). Exactly two files were regenerated:

| File | Change |
| --- | --- |
| `baseline.json` | The affected project's native snapshot entry: snapshot hash, source blob and working-tree flag rebound to the chosen version. Three designated fields only. |
| `tasks/<affected>/oracle-proposal.json` | The api source-evidence entry: section hash, end byte, blob and working-tree flag rebound. Four designated fields only. |

All **271 other files are byte-identical** to the independently re-resolved
input set; zero files were added, dropped or repinned. The chosen version is
the owner-inspected current snapshot (10,585 bytes); the superseded cached
snapshot (10,482 bytes) is recorded only as `previous` lineage in the
manifest's `version_binding` choice record. The candidate records
`adopted: false` — binding consistency does not silently adopt a baseline;
adoption remains an explicit owner decision.

## Verified properties

A private Go checker (`check.go`, offline, zero model calls) verifies:

- Manifest integrity for all 273 candidate files; file and directory modes.
- 30 affected answer bindings — 26 base + 4 conditional across exactly the
  nine recorded tasks — and the one affected grading source record.
- One chosen version across every affected surface: the view file, all
  native and allowed answer pins, the baseline entry, oracle source
  evidence, the grading source record (body and provenance), the
  current-baseline record, the source-index catalog and all grading joins.
- The frozen schedule unchanged: 108 base + 24 conditional rows, sequence
  order and per-row task/arm identity matching the bindings in both
  directions; 132 bindings, 30 tasks, role/stratum and arm counts intact.
- Zero occurrences of superseded-version markers anywhere in candidate
  inputs; they appear only as declared `previous_*` lineage in the manifest.

## Mixed-version rejection

Fourteen synthetic mutations are all rejected — eleven at manifest-integrity
level (stale pins) and three manifest-consistent mixes caught only by the
version-uniformity layer (oracle end-byte drift, a binding pin to the
superseded hash, a source-record body swapped to superseded text). A control
candidate uniformly bound to the *superseded* version with a matching
declared choice **passes**, confirming the checks enforce declared
uniformity rather than a hidden preferred version.

## Preservation

The original bound packet (261 files, manifest
`c22dbd8d04e5ac5ff8b60014b03d1f3cce9f5a6254dc3915dcf5587a53131d15`) and all
three overlay layers are untouched — verified by the unchanged private
packet checker and by the main-driver offline input check re-resolving to
the same input digest `c81650dbcf9ee907006d4f7aeca1fefddc0cf915bfffedce6b0abf06b82dfb69`.

Private candidate manifest SHA-256:
`b6eebd72d505ab8696f65cd6aae7e441e2adac34a44b6c694eee7fd8f9d6d18e`.

Private candidate resolved-input SHA-256:
`8b5f870ce9d3136b2db20840940e39d34de70d4ff2a1fe78c23fa2989f219268`.

Private check report SHA-256:
`34abe86403a90a842134b783dc9e7171a2d019a6debe216eb958c8dad5b95b68`.

Private tools, the candidate packet, instruction text, source paths and
held-out material stay outside git and GitHub. Public hashes bind custody;
readers without the private cache cannot reproduce the checks.

## Remaining

- The candidate is staged, not adopted. Adopting it as the execution
  baseline (or electing a different permitted version) is an owner decision.
- All other #124 readiness inputs are unchanged: final task/oracle adoption,
  natural-task grading acceptance, exact answering/grading profiles and
  complete request fit, processor disclosure, tool/permission inventories
  and the whole-run allowance. This work does not admit tasks, accept a
  grader, authorize the pilot or enable capture hooks.
