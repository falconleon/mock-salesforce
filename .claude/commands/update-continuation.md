# Update Continuation Prompt

Update `dev-docs/Continuation_Prompt.md` with fresh project state so a new
session can pick up exactly where this one left off.

**This skill delegates to a subagent to avoid consuming main context.**

## Instructions

### Step 1: Compose Your Session Summary

From YOUR context (what you already know), write a brief summary covering:

- **Session number**: Increment from the current continuation prompt's session
  number (check `Session Context` section — just the number, e.g., "13")
- **What you worked on**: Key tasks, epics, beads progressed or closed
- **What changed**: Files modified, features shipped, bugs fixed
- **Key decisions**: Architecture choices, approach changes, investigations done
- **Current state**: What's in progress, what's blocked, what's next
- **Uncommitted files**: What's staged/modified and why (if applicable)

This is the most valuable input — only you have the session narrative. Keep it
concise but complete (10-30 lines). The subagent gathers all mechanical state
independently.

### Step 2: Spawn the Update Agent

Use the Task tool to delegate the full investigation and update. Replace
`SESSION_SUMMARY_HERE` with your composed summary from Step 1.

```
Task(
  subagent_type="general-purpose",
  description="Update continuation prompt",
  mode="bypassPermissions",
  prompt="""
Update the falcon-backend continuation prompt file using TARGETED EDITS.

## Session Summary (from coordinator)

SESSION_SUMMARY_HERE

## Task 1: Gather Current State

Run this ONE command to collect all fresh data at once:

```bash
cd /Users/leon/dev/falcondev/falcon-backend && \
echo "=== GIT LOG ===" && git --no-pager log --oneline -15 && \
echo "=== GIT STATUS ===" && git branch --show-current && git status --short && \
echo "=== BEADS STATS ===" && bd stats 2>&1 && \
echo "=== OPEN EPICS ===" && (bd list --status=open --type=epic 2>/dev/null || echo "(none)") && \
echo "=== FILE COUNTS ===" && \
echo "Go source files: $(find . -name '*.go' ! -name '*_test.go' ! -path './vendor/*' | wc -l | tr -d ' ')" && \
echo "Go test files: $(find . -name '*_test.go' ! -path './vendor/*' | wc -l | tr -d ' ')" && \
echo "Test packages: $(go list ./... 2>/dev/null | wc -l | tr -d ' ')"
```

## Task 2: Read Current File

Read `/Users/leon/dev/falcondev/falcon-backend/dev-docs/Continuation_Prompt.md`
in full (use the Read tool).

## Task 3: Edit the File In-Place

Use the **Edit tool** to make targeted updates. Do NOT rewrite the entire file.
Make one Edit call per section that needs updating. This is faster and safer
than regenerating the full file.

### Sections to Update (use Edit tool for each)

1. **Header line** — Update the `Last Updated` date and `Phase` status summary.

2. **Architecture Health table** — Add/update rows for work done this session.
   Only add rows for genuinely new capabilities, not per-task granularity.

3. **Recent Sessions** — The file should have AT MOST 3 detailed session
   sections. The new session goes at the top.
   - **If there are already 3 detailed sessions**: Consolidate the oldest into
     a summary paragraph under the "Previous Sessions" section and replace it
     with the new session.
   - **If there are more than 3**: Consolidate ALL sessions older than the 3
     most recent into grouped summaries under "Previous Sessions".

4. **Session History Management** (IMPORTANT):
   - **Last 3 sessions**: Keep full detail (10-20 lines each)
   - **Sessions N-10 to N-3**: Consolidate into 2-3 themed paragraphs (e.g.,
     "Sessions 63-66: CASE pipeline implementation and verification...")
   - **Sessions before N-10**: One-paragraph summary (like existing
     "Sessions 1-35" section)
   - Target: session history section should be ~80-100 lines total, not 400+

5. **Open Epics table** — Replace with current epic list from beads.

6. **What's Queued / Next Steps** — Update priorities based on current state.

7. **File counts** — Update Go source/test/package counts.

### Edit Rules

- Use the Edit tool's `old_string` / `new_string` to target specific sections
- Each Edit should replace a clearly bounded section (between headings)
- Do NOT touch sections that haven't changed (architecture files, key files, etc.)
- If an Edit's old_string is too large, break it into smaller targeted edits
- Preserve all markdown formatting and heading hierarchy

## Task 4: Return Summary

Return ONLY a brief summary (not the full file). Include:
- Which sections were edited
- What changed in each
- Final file line count
- Any issues encountered

## CRITICAL Rules

- Use EDIT tool, not Write tool — targeted changes only
- Do NOT touch stable sections (Key Architecture Files, etc.)
- Do NOT let session history grow unbounded — consolidate per the rules above
- Target total file size: ~300-400 lines (not 600+)
- The goal is a CONCISE, ACCURATE prompt — not a comprehensive history
- Use ABSOLUTE paths for all file operations
""")
```

### Step 3: Report Result

When the subagent returns, relay its summary to the user. That's it — the
investigation and file update all happened in the subagent's context.

## Why This Approach

The subagent handles targeted edits in its own context. Your main context only
consumes:
- This skill (~15 lines of instructions you actually follow)
- Your session summary (~15-30 lines)
- The Task call + result (~15 lines)

Using Edit instead of Write means the agent doesn't need to regenerate the
entire file — it makes 5-8 surgical edits on specific sections, which is
both faster and less error-prone.
