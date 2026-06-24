# Migration: Run the town on a locally-built forked `gt` binary

**Bead:** hq-lls · **Status:** Complete · **Author:** polecat nux

## Goal

Stop depending on the Homebrew `gt` bottle and instead run the town on a `gt`
binary built from our own fork (`egroeg121/gastown`), while still pulling
improvements from upstream (`gastownhall/gastown`).

## Post-migration state

| Thing | Value |
|---|---|
| Installed binary | `~/.local/bin/gt` → fork build, shadowing Homebrew |
| `gt version --verbose` commit | shows fork commit hash (e.g. `51183512`) |
| `which gt` | `/Users/georgebarnett/.local/bin/gt` |
| Build source | `~/code/gastown` (standalone fork clone) |
| `rebuild-gt` plugin source | `~/code/gastown` (via `$GT_GASTOWN_SOURCE_DIR`) |
| Upstream remote in rig bare repo | ✅ `upstream = https://github.com/gastownhall/gastown.git` |
| Fallback | `rm ~/.local/bin/gt` reverts to Homebrew bottle instantly |

## Phase 0 — One-time bootstrap (COMPLETED)

Built and installed gt from the fork standalone clone:

```bash
cd ~/code/gastown
git fetch origin && git checkout main && git pull --ff-only origin main
make build
make install   # installs to ~/.local/bin/gt, restarts daemon, syncs plugins
```

Verified:
```bash
which gt        # → /Users/georgebarnett/.local/bin/gt
gt version --verbose  # → shows fork commit hash, NOT "Homebrew"
```

> Do **not** `brew uninstall gastown`. The bottle in place is a known-good
> fallback — `rm ~/.local/bin/gt` reverts to it instantly.

## Phase 1 — Fix rebuild-gt plugin path (COMPLETED)

The `rebuild-gt` plugin previously assumed source at `~/gt/gastown/mayor/rig`
(which never existed). Fixed to use `~/code/gastown` as the default build source,
overridable via `$GT_GASTOWN_SOURCE_DIR`.

Changed in `plugins/rebuild-gt/run.sh`:
```bash
# Before (wrong — ~/gt/... never existed):
RIG_ROOT="${TOWN_ROOT}/gastown/mayor/rig"

# After (correct):
RIG_ROOT="${GT_GASTOWN_SOURCE_DIR:-${HOME}/code/gastown}"
```

Also updated `plugin.md` examples to reference `~/code/gastown`.

## Phase 2 — Upstream remote in rig bare repo (COMPLETED)

The rig bare repo (`~/code/footy/gastown/.repo.git`) already had the upstream
remote added. Verify with:

```bash
git -C ~/code/footy/gastown/.repo.git remote -v
# Shows:
#   origin   https://github.com/egroeg121/gastown.git
#   upstream https://github.com/gastownhall/gastown.git
```

If it were missing, add it with:
```bash
git -C ~/code/footy/gastown/.repo.git remote add upstream \
    https://github.com/gastownhall/gastown.git
```

## Phase 3 — Rebase cadence (MANUAL, weekly or on-demand)

**Strategy:** rebase the fork's `main` onto `upstream/main`. Rebase (not merge)
keeps fork-specific commits as a clean linear set on top of upstream.

**Where:** the standalone `~/code/gastown` clone — outside the live rig so a
messy rebase never disturbs running sessions.

```bash
cd ~/code/gastown
git checkout main
git fetch upstream
git rebase upstream/main          # replay fork commits on top of upstream
# resolve conflicts if any, then:
git push origin main --force-with-lease   # update the fork
```

Then the rig picks it up on next `git pull` / next `rebuild-gt` run.

**Cadence:** weekly, or on-demand when a wanted upstream fix lands.

**NEVER automate the force-push.** A botched rebase that force-pushes the fork
is high blast radius. If a rebase gets ugly, `git rebase --abort` and escalate.

## Ongoing: rebuild-gt automation

The Deacon dispatches `rebuild-gt` on a 1h cooldown. It runs `gt stale --json`
and if the binary is behind `main` (and forward-safe + clean + on main), it runs:
```bash
cd ~/code/gastown && make build && make safe-install
```

`safe-install` replaces the binary WITHOUT restarting the daemon, so live
sessions are undisturbed and pick up the new binary on their next natural cycle.

## Fallback

```bash
rm ~/.local/bin/gt   # reverts to Homebrew bottle (/opt/homebrew/bin/gt)
```

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
