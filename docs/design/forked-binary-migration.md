# Migration: Run the town on a locally-built forked `gt` binary

**Bead:** hq-65x · **Status:** Plan (design doc) · **Author:** polecat furiosa

## Goal

Stop depending on the Homebrew `gt` bottle and instead run the town on a `gt`
binary built from our own fork (`egroeg121/gastown`), while still pulling
improvements from upstream (`gastownhall/gastown`).

The end state:

1. Polecats make changes in the **gastown rig** (worktrees of the
   `egroeg121/gastown` fork).
2. Work lands on the fork's `main` via the rig's normal merge queue / PRs.
3. The local `gt` binary is **rebuilt from the fork** and shadows the Homebrew
   bottle in `PATH`.
4. The fork periodically **rebases onto upstream** so we keep getting upstream
   fixes.

## Verified current state (2026-06-20)

| Thing | Value |
|---|---|
| Installed binary | `/opt/homebrew/bin/gt` → Homebrew bottle `1.1.0` |
| `gt version --verbose` commit | reports `Homebrew` (no source commit) |
| `gt stale --json` | `stale=false, safe_to_rebuild=false, binary_commit="Homebrew"` |
| Town root | `/Users/georgebarnett/code/footy` |
| gastown rig root | `/Users/georgebarnett/code/footy/gastown` |
| Rig bare repo | `…/gastown/.repo.git` → `origin = egroeg121/gastown` (fork) ✅ |
| Rig worktrees (witness/refinery/polecats) | origin = fork ✅ |
| `upstream` remote in rig | **MISSING** (only `origin` exists) ⚠️ |
| Standalone clone | `~/code/gastown` (origin=fork, upstream=gastownhall/gastown) ✅ |
| `~/.local/bin` in PATH | **first**, ahead of `/opt/homebrew/bin` ✅ |
| `~/.local/bin/gt` | does not exist yet |
| Go toolchain | go1.26.4 (go.mod wants 1.26.2 ✅) |
| icu4c (CGo dep) | `/opt/homebrew/opt/icu4c@78` present ✅ |
| `rebuild-gt` plugin | exists, but assumes source at `~/gt/gastown/mayor/rig` ⚠️ |

Two assets already exist that make this easy, and two gaps that must be fixed.

**Assets**

- The fork's `Makefile` already has `build`, `install`, `safe-install`,
  `check-up-to-date`, `check-forward-only`, and `check-version-tag` targets.
  `INSTALL_DIR = $(HOME)/.local/bin`, which is already first in `PATH` — so a
  build will shadow the Homebrew bottle with **no `brew uninstall` required**.
- The `rebuild-gt` plugin (`plugins/rebuild-gt/`) already automates
  "detect stale binary → `make build && make safe-install`", with a forward-only
  safety guard that prevented a past crash-loop.

**Gaps**

- **G1 — wrong source path in `rebuild-gt`.** The plugin builds from
  `~/gt/gastown/mayor/rig`, which does not exist in this town. The real rig
  source is `~/code/footy/gastown/<role>/rig`.
- **G2 — no `upstream` remote in the rig.** The fork bare repo has only
  `origin`. The rebase-from-upstream loop has nowhere to pull from. Today only
  the standalone `~/code/gastown` clone has `upstream`.

## Migration plan

### Phase 0 — One-time bootstrap (build & install from fork)

Pick a canonical build source. **Recommendation:** `mayor/rig`
(`~/code/footy/gastown/mayor/rig`) — it is long-lived, always tracks `main`, and
is not churned by polecat merge-queue activity the way `refinery/rig` is.

```bash
cd ~/code/footy/gastown/mayor/rig
git fetch origin && git checkout main && git pull --ff-only origin main
make build          # produces ./gt (+ proxy client/server)
make install        # copies to ~/.local/bin/gt, restarts daemon, syncs plugins
```

`make install` will:
- write `~/.local/bin/gt` (now shadows Homebrew bottle, since `.local/bin` is
  first in PATH),
- nuke stale `~/go/bin/gt` / `~/bin/gt` shadows,
- restart the daemon to pick up the new binary,
- `gt plugin sync` from the source tree.

**Verify:**

```bash
which gt                 # → /Users/.../.local/bin/gt
gt version --verbose     # → shows a real commit hash, NOT "Homebrew"
gt stale --json          # → binary_commit is a sha; safe_to_rebuild logic now works
```

> Do **not** `brew uninstall gastown`. Leaving the bottle in place is harmless
> (it's shadowed) and gives a known-good fallback: if the source build is ever
> broken, `rm ~/.local/bin/gt` instantly reverts to the bottle.

### Phase 1 — Fix the rebuild automation (G1)

The `rebuild-gt` plugin must point at the real source dir. Make the path
town-relative instead of hard-coded.

In `plugins/rebuild-gt/run.sh`, change:

```bash
RIG_ROOT="${TOWN_ROOT}/gastown/mayor/rig"
```

to derive `TOWN_ROOT` from `gt town root` (already done) — the bug is the
literal `~/gt/...` assumption baked into `plugin.md` examples. After Phase 0,
`${TOWN_ROOT}/gastown/mayor/rig` resolves to
`~/code/footy/gastown/mayor/rig`, which is correct. **Action:** update
`plugin.md` examples from `~/gt/gastown/mayor/rig` to
`$(gt town root)/gastown/mayor/rig` so docs and script agree.

This is a small code change and should be filed as its own bead (see Follow-ups).

With that fixed, ongoing rebuilds are automatic: the Deacon dispatches
`rebuild-gt` on a 1h cooldown; it runs `gt stale --json`, and if the binary is
behind `main` (and forward-safe + clean + on main), it runs
`make build && make safe-install`. `safe-install` deliberately does **not**
restart the daemon, so live sessions are undisturbed and pick up the new binary
on their next natural cycle.

### Phase 2 — Wire up upstream for rebasing (G2)

Add `upstream` to the rig bare repo so the fork can track upstream:

```bash
git -C ~/code/footy/gastown/.repo.git remote add upstream \
    https://github.com/gastownhall/gastown.git
git -C ~/code/footy/gastown/.repo.git fetch upstream
```

(The standalone `~/code/gastown` clone already has this and can serve as the
rebase workspace if you'd rather not rebase inside the rig — see below.)

### Phase 3 — Rebase cadence (pull upstream into the fork)

**Strategy: rebase the fork's `main` onto `upstream/main`.** Rebase (not merge)
keeps fork-specific commits as a clean linear set on top of upstream, which makes
"what have we changed vs upstream" trivially inspectable and keeps
`check-forward-only` semantics sane.

**Where to do it:** the standalone `~/code/gastown` clone is the safest place —
it is outside the live rig, so a messy rebase never disturbs running sessions.

```bash
cd ~/code/gastown
git checkout main
git fetch upstream
git rebase upstream/main          # replay fork commits on top of upstream
# resolve conflicts if any, then:
git push origin main --force-with-lease   # update the fork
```

Then the rig picks it up on its next `git pull` / next `rebuild-gt` run.

**Cadence recommendation:**
- **Weekly** scheduled rebase (low-friction, fork stays close to upstream so
  conflicts stay small), OR
- **On-demand** when a wanted upstream fix lands.

Automate the *check* (not the conflict resolution) with a scheduled routine that
fetches upstream and opens a bead if the fork is >N commits behind, so a human/
polecat does the rebase deliberately. **Do not auto-force-push rebases** — a
botched rebase that force-pushes the fork is high-blast-radius.

**Conflict handling:** because fork commits sit on top of upstream, conflicts
surface only on the fork's own changes. Resolve in the standalone clone, run
`make build && make test`, and only then `push --force-with-lease`. If a rebase
gets ugly, `git rebase --abort` and escalate rather than forcing through.

### Phase 4 — Polecat workflow in the gastown rig (already correct)

No change needed — verified working:
- The rig bare repo `.repo.git` already points at the fork.
- Polecat worktrees (e.g. this one) are spawned from that bare repo, so
  `origin = egroeg121/gastown` automatically.
- Polecats branch, `gt done`, and the Refinery merges to the fork's `main` via
  the merge queue.

The **only** thing that closes the loop is Phase 1's automated `rebuild-gt`:
after polecat work merges to fork `main`, the binary is rebuilt so the town runs
its own freshly-merged code.

## End-to-end flow (steady state)

```
polecat edits fork worktree
        │  gt done
        ▼
Refinery merges → fork main (egroeg121/gastown)
        │  Deacon dispatches rebuild-gt (1h cooldown)
        ▼
gt stale detects binary behind main → make build && make safe-install
        │
        ▼
~/.local/bin/gt updated (shadows Homebrew bottle) → sessions pick it up on cycle

   ── weekly / on-demand ──
upstream/main ──rebase──> fork main ──force-with-lease──> origin
```

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Bad build bricks the town (past crash-loop) | `check-forward-only` guard + `safe-install` (no daemon restart). Fallback: `rm ~/.local/bin/gt` reverts to Homebrew bottle. |
| Force-push rebase corrupts fork | Rebase in standalone clone, `--force-with-lease`, build+test before push, never automate the force-push. |
| `rebuild-gt` points at non-existent path (G1) | Fix path before relying on automation (Phase 1). |
| Version-tag drift | `check-version-tag` already guards release tags. |
| CGo/icu4c build failure on a fresh machine | Makefile auto-detects `brew --prefix icu4c`; documented in INSTALLING.md. |

## Follow-up beads to file

1. **Fix `rebuild-gt` source path (G1)** — update `plugins/rebuild-gt/plugin.md`
   examples and confirm `run.sh` resolves `${TOWN_ROOT}/gastown/mayor/rig`.
   Type: bug, P1.
2. **Add `upstream` remote to rig bare repo (G2)** — one-time setup command.
   Type: task, P2.
3. **Scheduled upstream-drift check** — routine that fetches `upstream`, files a
   bead when fork is >N commits behind. Type: task, P2.
4. **Bootstrap doc in INSTALLING.md** — document the Phase 0 build-from-source
   install for new machines. Type: task, P3.
