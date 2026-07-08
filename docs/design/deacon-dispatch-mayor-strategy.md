# Deacon Owns Auto-Dispatch, Mayor Owns Strategy

> Status: accepted (hq-5ws) · Owner: Mayor / Deacon roles

## Problem

The Mayor used to run the dispatch loop by hand: a user request came in, the
Mayor filed a bead and `gt sling`'d it to a rig. This burned the Mayor's
context on a mechanical chore — scanning `bd ready`, picking a rig, slinging —
that requires no judgment most of the time. The Mayor should wake for
*decisions*, not for the repetitive act of turning ready beads into running
polecats.

## Decision

Split dispatch responsibilities by **what they require**:

| | Deacon (operational) | Mayor (strategic) |
|---|---|---|
| Scan `bd ready` per rig each cycle | ✅ | |
| Sling unblocked, eligible work to polecats | ✅ | |
| Respect per-rig polecat capacity | ✅ | |
| Honor priority order | ✅ (reads it) | ✅ (sets it) |
| Set / change priorities | | ✅ |
| Defer, cancel, re-scope work | | ✅ |
| Cross-rig trade-offs | | ✅ |
| Dispatch `needs-*`/`hold`/`design-review` beads | | ✅ |
| Talk to the user | | ✅ |
| Handle escalations needing judgment | | ✅ |
| Restart stalled polecats / clean zombies | (detect only) | |

**The line:** the Deacon turns *standing intent* into running polecats. The
Mayor *sets the standing intent* (priorities, which rigs are awake, which beads
are held for judgment) and makes the calls the Deacon can't.

## How the Mayor steers dispatch without slinging

The Mayor shapes what the Deacon dispatches by shaping beads, not by issuing
`gt sling`:

- **Priority** — the Deacon dispatches high-priority ready work first.
- **Hold labels** — `needs-mayor`, `needs-human`, `hold`, `design-review`
  exclude a bead from auto-dispatch until the Mayor clears it.
- **Rig wake/sleep** — the Deacon only dispatches to operational (undocked,
  unparked) rigs.

The Mayor slings by hand only for urgent P0s that can't wait for the next
patrol cycle, or beads the Deacon flagged via `DISPATCH_REVIEW`.

## The dispatch ladder (Deacon patrol step)

Implemented as the `dispatch-ladder` step in `mol-deacon-patrol`. Each
**full-effort** patrol cycle (skipped on abbreviated/idle cycles):

1. Enumerate operational rigs (`gt rig list`); skip DOCKED/PARKED.
2. Per rig, `bd ready --repo <rig> --json` for unblocked, unhooked work.
3. Filter to **dispatch-eligible** beads — `open`, no active polecat, not
   labelled `needs-*`/`hold`/`design-review`, not coordination-only.
4. Check per-rig capacity (`gt polecat list`, `max_polecats`); don't exceed it.
5. `gt sling <bead> <rig> --merge=local` in priority order until capacity hits.
6. Flag ambiguous work to the Mayor (`DISPATCH_REVIEW`) instead of guessing.
7. Log the dispatch summary for the patrol digest.

### Safety rails

- Idle Town Principle: the ladder only runs on full-effort cycles, so an idle
  town stays quiet.
- Capacity-bounded: queued beads stay `open` and are picked up a later cycle —
  no spawn storms into a saturated pool.
- Docked/parked rigs are invisible — never undocked to create dispatch targets.
- Hold labels and coordination beads are left for the Mayor.

## Interaction with the Accountant (forward-looking)

When a capacity-tracking role (Accountant) signals `CAPACITY_AVAILABLE` for a
rig, the Deacon treats that rig as having a free slot this cycle and prefers it.
The Accountant role is not yet implemented; the ladder works today using
`gt polecat list` + `max_polecats` for capacity.

## Where this is documented

- **Deacon role** (`internal/templates/roles/deacon.md.tmpl`) — auto-dispatch
  listed under "You ARE responsible for"; strategy explicitly excluded.
- **Mayor role** (`internal/templates/roles/mayor.md.tmpl`) — "File It,
  Prioritize It — the Deacon Slings It"; strategic levers and decision tree.
- **Deacon formula** (`internal/formula/formulas/mol-deacon-patrol.formula.toml`)
  — the `dispatch-ladder` step.
