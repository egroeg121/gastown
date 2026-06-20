#!/usr/bin/env bash
# upstream-drift/run.sh — File a bead when the gastown fork lags upstream.
#
# SAFETY: Read-only. Never rebases, force-pushes, or modifies any branch.
# Files a bead + escalation only; the human performs the rebase manually.

set -euo pipefail

TOWN_ROOT="${GT_TOWN_ROOT:-$(gt town root 2>/dev/null)}"
BARE_REPO="${TOWN_ROOT}/gastown/.repo.git"
THRESHOLD="${UPSTREAM_DRIFT_THRESHOLD:-5}"

log() { echo "[upstream-drift] $*"; }

# --- Verify bare repo exists --------------------------------------------------

if [ ! -d "$BARE_REPO" ]; then
  log "SKIP: bare repo $BARE_REPO not found"
  exit 0
fi

if ! git -C "$BARE_REPO" rev-parse --git-dir >/dev/null 2>&1; then
  log "SKIP: $BARE_REPO is not a git repo"
  exit 0
fi

# --- Verify upstream remote exists -------------------------------------------

if ! git -C "$BARE_REPO" remote get-url upstream >/dev/null 2>&1; then
  ERROR="No 'upstream' remote in $BARE_REPO. Add it: git -C $BARE_REPO remote add upstream https://github.com/gastownhall/gastown.git"
  log "SKIP: $ERROR"
  gt plugin record-run --plugin upstream-drift --result skipped --rig gastown \
    --title "upstream-drift: skipped (no upstream remote)" \
    --description "$ERROR" >/dev/null 2>&1 || true
  exit 0
fi

# --- Fetch upstream -----------------------------------------------------------

log "Fetching upstream..."
if ! git -C "$BARE_REPO" fetch upstream --quiet 2>&1; then
  ERROR="git fetch upstream failed"
  log "FAILED: $ERROR"
  gt plugin record-run --plugin upstream-drift --result failure --rig gastown \
    --title "upstream-drift: FAILED" \
    --description "$ERROR" >/dev/null 2>&1 || true
  gt escalate --severity=medium \
    --subject="Plugin FAILED: upstream-drift" \
    --body="$ERROR" \
    --source="plugin:upstream-drift" 2>/dev/null || true
  exit 1
fi

# --- Count drift --------------------------------------------------------------

BEHIND=$(git -C "$BARE_REPO" rev-list --count upstream/main ^origin/main 2>/dev/null || echo 0)
log "Fork is $BEHIND commit(s) behind upstream/main"

# --- Threshold check ----------------------------------------------------------

if [ "$BEHIND" -le "$THRESHOLD" ]; then
  log "Within threshold ($THRESHOLD). No action needed."
  gt plugin record-run --plugin upstream-drift --result success --rig gastown \
    --title "upstream-drift: $BEHIND commit(s) behind (OK, threshold=$THRESHOLD)" >/dev/null 2>&1 || true
  exit 0
fi

# --- Drift exceeds threshold — file bead + escalate --------------------------

log "Drift ($BEHIND) exceeds threshold ($THRESHOLD). Filing bead..."

BEAD_ID=$(bd create --repo hq \
  --title "Rebase gastown fork onto upstream/main ($BEHIND commits behind)" \
  --type task --priority 2 \
  --description "upstream-drift plugin detected the fork is $BEHIND commits behind gastownhall/gastown upstream/main.

Manual rebase required:
  cd ~/code/gastown
  git fetch upstream
  git rebase upstream/main
  git push --force-with-lease origin main

Do NOT auto-rebase or force-push without review. Confirm CI is green after push." \
  --json 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")

if [ -n "$BEAD_ID" ]; then
  log "Filed bead: $BEAD_ID"
else
  log "WARNING: bd create did not return an ID (bead may still have been created)"
fi

gt escalate --severity=medium \
  --subject="upstream-drift: fork is $BEHIND commits behind upstream" \
  --body="gastownhall/gastown has $BEHIND commits not yet in egroeg121/gastown fork (threshold=$THRESHOLD). Bead filed${BEAD_ID:+: $BEAD_ID}. To fix: cd ~/code/gastown && git fetch upstream && git rebase upstream/main && git push --force-with-lease origin main" \
  2>/dev/null || true

gt plugin record-run --plugin upstream-drift --result success --rig gastown \
  --title "upstream-drift: $BEHIND commit(s) behind — bead filed${BEAD_ID:+ ($BEAD_ID)}" \
  --description "Fork is $BEHIND commits behind upstream (threshold=$THRESHOLD). Bead filed${BEAD_ID:+: $BEAD_ID}." >/dev/null 2>&1 || true

log "Done."
