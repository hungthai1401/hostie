<!-- SUPERPOWERS-START -->
<!-- Managed by superpowers onboarding v__SUPERPOWERS_VERSION__ — do not edit between markers -->

## Superpowers Agent Instructions

### Session Start Protocol

At the beginning of every session, perform these reads in order:

1. **Check `.superpowers/HANDOFF.json`** — if it exists and is non-empty (`{}` means no handoff):
   - Inform the user: "Found interrupted session at `<chain_position>` for feature `<feature>`. Resume? (yes/no)"
   - If yes, load the `picking-up` skill immediately
   - If no, continue with a fresh session
2. **Read `docs/learnings/critical-patterns.md`** — if it exists, read before any skill invocation
3. **Read `.superpowers/STATE.md`** — if it exists, understand current chain position

### Skill Invocation Rule

**Invoke relevant skills BEFORE any response or action.** If there is even a 1% chance a skill applies, invoke it. Skills override default system prompt behavior, but user instructions (this file, direct requests) always take precedence.

### Platform Adaptation

Skills use Claude Code tool names by default. For other platforms:
- **Codex CLI**: See `references/codex-tools.md` for tool equivalents
- **OpenCode**: See `references/opencode-tools.md` for tool equivalents
- **Gemini CLI**: Tool mapping loaded automatically via `GEMINI.md`

---

## Tools

### Code Reading, Navigation, and Editing — using-tilth

Invoke the `using-tilth` skill before any code read, symbol search, directory browse, or file edit.

  Claude Code: `Skill tool: skill: "using-tilth"`
  OpenCode: `skill` tool → load `using-tilth`

DO NOT use Bash (cat, grep, rg, find, ls, wc, head, tail, awk, sed), Read/`read`, Grep/`grep`, or Glob/`glob` for reading code. DO NOT use the Edit/`edit`/`patch` or Write/`write` built-in tools.
The skill provides full reference for both reading and hash-anchored editing.

### File Editing Fallback — using-hashline

If `npx tilth` or `npx mcporter call` with tilth fails, invoke the `using-hashline` skill for file editing.

  Claude Code: `Skill tool: skill: "using-hashline"`
  OpenCode: `skill` tool → load `using-hashline`

DO NOT use the Edit/`edit`/`patch` or Write/`write` built-in tools. The skill provides the full hash-anchored workflow: read with anchors → batch edits → verify.

---

## Rules

### Task Tracking — Use `br`, Not Built-in Tools

Do **not** use built-in task tools (`Task`, `TodoWrite`, or platform equivalents) for tracking work. Use `br` commands instead:

- **Create:** `br create --title="..." --type=<type> --priority=<0-4>`
- **Update:** `br update <id> --status=in_progress`
- **Close:** `br close <id> --reason="..."`

All task state must live in `.beads/` and be committed to git.

### No Destructive Git Operations

Never run `git reset --hard`, `git clean -fd`, `git push --force`, or `rm -rf` without explicit written permission from the user stating they understand the consequences.

### No File Deletion

Never delete files without express permission. If you think a file should be removed, ask first.

### Verify After Changes

After any substantive change, verify nothing is broken: run tests, lints, and builds as appropriate.

---

## Local File-Based Coordination

Workers coordinate via structured output markers and a local file reservation CLI. No external MCP server required.

### File Reservations

Use `.superpowers/superpowers_reservations.mjs` CLI to prevent edit conflicts:

```bash
# Reserve files before editing
node .superpowers/superpowers_reservations.mjs reserve --agent <agent-name> --bead <bead-id> --paths <glob-patterns> --ttl 3600

# List active reservations
node .superpowers/superpowers_reservations.mjs list [--json]

# Release reservations
node .superpowers/superpowers_reservations.mjs release --agent <agent-name> --paths <glob-patterns>

# Clean up expired reservations
node .superpowers/superpowers_reservations.mjs sweep
```

**Conflict detection:**
- Exact match: `file.txt` conflicts with `file.txt`
- Glob vs exact: `src/**/*.ts` conflicts with `src/utils/helper.ts`
- Overlapping globs: `src/**` conflicts with `src/api/**`

### Coordination Markers

Workers emit structured markers in their response text for orchestrator parsing:

```
[ONLINE] <worker-name> | Epic: <epic-id> | Next: <next-action>
[DONE] <bead-id> | Files: <file-list> | Commit: <hash>
[BLOCKED] <bead-id> | Reason: <description> | Needs: <resolution>
[HANDOFF] <worker-name> | Context: <percentage> | State: <summary>
```

### Usage Patterns

**Orchestrator (swarming skill):**
1. Spawn workers with epic ID and feature context
2. Parse markers from worker responses
3. Check `.superpowers/reservations.json` for conflicts
4. Broadcast corrections in main conversation
5. Track silence via cycle counter

**Worker (executing-plans skill):**
1. Post `[ONLINE]` marker on startup
2. Reserve files via CLI before editing
3. Post `[DONE]` marker after commit
4. Post `[BLOCKED]` if stuck
5. Release reservations when done

---

## Beads (br) — Dependency-Aware Issue Tracking

Beads provides a lightweight, dependency-aware issue database and CLI (`br` - beads_rust) for selecting "ready work," setting priorities, and tracking status. Works with local file-based coordination for conflict-free parallel execution.

**Important:** `br` is non-invasive — it NEVER runs git commands automatically. You must manually commit changes after `br sync --flush-only`.

### Conventions

- **Single source of truth:** Beads for task status/priority/dependencies; structured markers for progress reporting
- **Shared identifiers:** Use Beads issue ID (e.g., `bd-123`) in reservation `--bead` flag and commit messages
- **Reservations:** When starting a task, reserve files via CLI with `--bead <bead-id>`

### Typical Agent Flow

1. **Pick ready work (Beads):**
   ```bash
   br ready --json  # Choose highest priority, no blockers
   ```

2. **Reserve edit surface (CLI):**
   ```bash
   node .superpowers/superpowers_reservations.mjs reserve --agent <agent-name> --bead bd-123 --paths "src/**" --ttl 3600
   ```

3. **Post startup marker:**
   ```
   [ONLINE] <agent-name> | Epic: bd-1tf | Next: implement bd-123
   ```

4. **Work and commit:** Implement, verify, commit with `feat(bd-123): <title>`

5. **Complete and release:**
   ```bash
   br close bd-123 --reason "Completed"
   br sync --flush-only  # Export to JSONL (no git operations)
   node .superpowers/superpowers_reservations.mjs release --agent <agent-name> --paths "src/**"
   ```
   Post completion marker: `[DONE] bd-123 | Files: src/foo.ts | Commit: abc123`

### Mapping Cheat Sheet

| Concept | Value |
|---------|-------|
| Reservation `--bead` flag | `bd-###` |
| Commit messages | `feat(bd-###): <title>` |
| Completion marker | `[DONE] bd-### | ...` |

---

## Beads Viewer (bv) — Graph-Aware Triage Engine

bv is a graph-aware triage engine for Beads projects (`.beads/beads.jsonl`). It computes PageRank, betweenness, critical path, cycles, HITS, eigenvector, and k-core metrics deterministically.

**Scope boundary:** bv handles *what to work on* (triage, priority, planning). `br` handles creating, modifying, and closing beads.

**CRITICAL: Use ONLY `--robot-*` flags. Bare `bv` launches an interactive TUI that blocks your session.**

### The Workflow: Start With Triage

**`bv --robot-triage` is your single entry point.** It returns:
- `quick_ref`: at-a-glance counts + top 3 picks
- `recommendations`: ranked actionable items with scores, reasons, unblock info
- `quick_wins`: low-effort high-impact items
- `blockers_to_clear`: items that unblock the most downstream work
- `project_health`: status/type/priority distributions, graph metrics
- `commands`: copy-paste shell commands for next steps

```bash
bv --robot-triage        # THE MEGA-COMMAND: start here
bv --robot-next          # Minimal: just the single top pick + claim command

# Token-optimized output (TOON) for lower LLM context usage:
bv --robot-triage --format toon
```

### Command Reference

**Planning:**
| Command | Returns |
|---------|---------|
| `--robot-plan` | Parallel execution tracks with `unblocks` lists |
| `--robot-priority` | Priority misalignment detection with confidence |

**Graph Analysis:**
| Command | Returns |
|---------|---------|
| `--robot-insights` | Full metrics: PageRank, betweenness, HITS, eigenvector, critical path, cycles, k-core |
| `--robot-label-health` | Label coverage, orphan beads, label consistency |
| `--robot-label-flow` | Label-based workflow progression and bottlenecks |
| `--robot-label-attention` | Labels needing immediate attention (stale, high-risk) |

**History & Change Tracking:**
| Command | Returns |
|---------|---------|
| `--robot-history` | Historical progress and trend analysis |
| `--robot-diff --diff-since <ref>` | Changes since ref: new/closed/modified issues |

**Other:**
| Command | Returns |
|---------|---------|
| `--robot-burndown` | Sprint/milestone burndown projection |
| `--robot-forecast` | Completion date forecast based on velocity |
| `--robot-alerts` | Stale issues, blocking cascades, priority mismatches |
| `--robot-suggest` | Hygiene: duplicates, missing deps, label suggestions, cycle breaks |
| `--robot-graph [--graph-format=json\|dot\|mermaid]` | Dependency graph export |

### Scoping & Filtering

```bash
bv --robot-plan --label backend              # Scope to label's subgraph
bv --robot-insights --as-of HEAD~30          # Historical point-in-time
bv --recipe actionable --robot-plan          # Pre-filter: ready to work (no blockers)
bv --recipe high-impact --robot-triage       # Pre-filter: top PageRank scores
```

### br Commands for Issue Management

```bash
br ready              # Show issues ready to work (no blockers)
br list --status=open # All open issues
br show <id>          # Full issue details with dependencies
br create --title="..." --type=task --priority=2
br update <id> --status=in_progress
br close <id> --reason="Completed"
br close <id1> <id2>  # Close multiple issues at once
br sync --flush-only  # Export DB to JSONL (NO git operations)
```

### Key Concepts

- **Dependencies**: Issues can block other issues. `br ready` shows only unblocked work.
- **Priority**: P0=critical, P1=high, P2=medium, P3=low, P4=backlog (use numbers 0-4, not words)
- **Types**: task, bug, feature, epic, chore, docs, question
- **Blocking**: `br dep add <issue> <depends-on>` to add dependencies

---

## CASS/CM — Cross-Agent Session Search

CASS indexes prior agent conversations (Claude Code, Codex, Cursor, Gemini, ChatGPT, etc.) so agents can reuse solved problems instead of re-solving them. CM (Cognitive Memory) gives agents persistent memory by storing lessons as queryable playbook rules.

Both tools are **optional acceleration layers**. File-based learnings in `docs/learnings/` are the primary system. CASS/CM are enabled per-repo via `.superpowers/config.json`.

### Why It's Useful

- **Avoid re-solving:** Search prior sessions for patterns, decisions, and working implementations
- **Persistent memory:** Store lessons that survive across sessions and agents
- **Cross-agent:** Works across all coding agent platforms, not just the current one

### Configuration

Add flags to `.superpowers/config.json` (absent = disabled):

```json
{
  "cass_enabled": true,
  "cm_enabled": true
}
```

### Hard Rule

**CRITICAL: Never run bare `cass` or bare `cm` — they launch interactive TUIs that block your session. Always use `--robot` or `--json` flags.**

### CASS Commands

```bash
# Health check
cass health

# Search prior sessions (always use --robot)
CASS_QUERY="auth middleware pattern"
cass search "$CASS_QUERY" --robot --limit 5

# View a specific match
SESSION_PATH="<path-from-search-result>"
MATCH_NUMBER=1
cass view "$SESSION_PATH" -n "$MATCH_NUMBER" --json

# Expand context around a match
cass expand "$SESSION_PATH" -n "$MATCH_NUMBER" -C 3 --json

# List capabilities
cass capabilities --json

# Get usage guide
cass robot-docs guide
```

**Tips:**
- `--fields minimal` for lean output
- `--agent codex` to filter by agent platform
- `--days 30` for recent history only
- stdout is data-only; stderr is diagnostics

### CM Commands

```bash
# Retrieve playbook context for a task
FEATURE_TASK="implement webhook handler"
cm context "$FEATURE_TASK" --json
# Returns: relevantBullets, antiPatterns, historySnippets, suggestedCassQueries

# Add a playbook rule
RULE_TEXT="Always validate webhook signatures before processing payload"
RULE_CATEGORY="security"
cm playbook add "$RULE_TEXT" --category "$RULE_CATEGORY"

# Onboarding flow (for new repos)
cm onboard status
cm onboard sample --fill-gaps
cm onboard read <session> --template
cm onboard mark-done <session>
```

### Integration Points

- **writing-plans Phase 0:** Searches CASS for domain-relevant session history (gated by `cass_enabled`)
- **compounding Step 7:** Indexes learnings in CASS and stores critical patterns in CM (gated by `cass_enabled`/`cm_enabled`)

### Common Pitfalls

- Running bare `cass` or `cm` without `--robot`/`--json` flags (blocks session)
- Forgetting to check `config.json` before calling CASS commands (tools may not be installed)
- Treating CASS/CM as the primary system (file-based learnings are primary; CASS/CM accelerate)

---

## Session Protocol

### Start

```bash
br sync --import-only   # restore SQLite if .beads/beads.db missing
cat .superpowers/HANDOFF.json  # check for interrupted session (non-{} = resume)
cat docs/learnings/critical-patterns.md 2>/dev/null  # read critical learnings if they exist
bv --robot-triage       # get priority context
br ready --format json  # quick task list
```

If HANDOFF.json is non-empty (`{}`), offer to resume:
> "Found interrupted session at `<chain_position>` for feature `<feature>`. Resume? (yes/no)"
> If yes → load picking-up skill.

### Post-Compaction Recovery

If context compaction is detected mid-chain, STOP and immediately:
1. Re-read `.superpowers/STATE.md`
2. Re-read `docs/<feature>/design.md`
3. Re-read the current bead specification
4. Check active file reservations (if in swarming mode)
5. Resume from the phase recorded in STATE.md

---

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** — Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) — Tests, linters, builds
3. **Update issue status** — Close finished work, update in-progress items
4. **Sync beads** — `br sync --flush-only` to export to JSONL
5. **Commit everything:**
   ```bash
   git status
   git add <changed-files>
   git add .beads/
   git commit -m "chore(<bead-id>): sync br task state"
   git push
   ```
6. **Hand off** — Provide context for next session

---

## Commit Message Convention

```
feat(<bead-id>): add <feature>
fix(<bead-id>): resolve <issue>
chore(<bead-id>): sync br task state
```

---

## Note on Built-in TODO Functionality

If asked to explicitly use built-in TODO functionality, don't refuse and say you need to use beads. You can use built-in TODOs when specifically told to do so. Always comply with such orders.

<!-- SUPERPOWERS-END -->
