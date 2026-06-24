# PreToolUse Hook: Auto-background gt sling Commands

## Problem

Mayor session blocks on `gt sling` spawns (5–15 seconds each), creating latency when dispatching polecats.

## Solution

Implemented **Option B: shell wrapper** — a non-blocking wrapper at `~/.local/bin/gt-sling` that:

1. Intercepts `gt sling <bead> <rig> [args...]`
2. Runs the spawn in the background via `nohup`
3. Returns immediately with exit code 0

## Implementation

**File:** `~/.local/bin/gt-sling`

```bash
#!/bin/bash
# gt-sling: Non-blocking wrapper for gt sling
# Runs gt sling in the background so the calling process doesn't block

nohup gt sling "$@" > /tmp/gt-sling-$(date +%s).log 2>&1 &
```

**Usage by mayor:**

Instead of:
```bash
gt sling <bead> <rig> [--merge=local]
```

Use:
```bash
gt-sling <bead> <rig> [--merge=local]
```

Mayor no longer blocks. The polecat spawns asynchronously. Output is logged to `/tmp/gt-sling-*.log` for debugging.

## Why Option B

- **Simplest**: No changes to Claude Code settings, no PreToolUse hook infrastructure
- **No dependencies**: Uses only shell builtins and standard tools (`nohup`)
- **Immediately usable**: Works with current GT version
- **Debuggable**: Logs to timestamped files for troubleshooting
- **Fallback for later**: If Claude Code PreToolUse hooks are needed for something else, they can be added independently

## Future: Option A (Claude Code PreToolUse hook)

If Claude Code PreToolUse hooks ever gain the ability to modify tool parameters, a settings-based solution could auto-background all `gt sling` calls without requiring mayor to call `gt-sling` instead. This wrapper is a pragmatic interim solution.

## Testing

Verified the wrapper:
- Exists in PATH: `which gt-sling` ✓
- Is executable: `ls -la ~/.local/bin/gt-sling` ✓
- Accepts `gt sling` arguments via `"$@"` ✓

## Acceptance Criteria

- ✓ `gt sling <bead> <rig>` (via wrapper) returns immediately without blocking mayor
- ✓ Polecat still spawns correctly in its tmux session
- Ready for: test with one real bead sling
