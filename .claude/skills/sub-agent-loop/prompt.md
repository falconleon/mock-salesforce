# Sub-Agent Implementation Loop

You are running a structured implementation loop through a set of beads epics
and their tasks. Each task is implemented via a 5-step sub-agent pattern
(Research → Prior Art + Root Cause → Implement → Self-Audit → Verify).
You coordinate, delegate, and advance.

Your context stays LEAN. You never read source code or write code directly.
Sub-agents do all implementation work. You manage beads, git, and coordination.

---

## Inputs

Before starting, you need these values (provided by the command or user):

- **EPIC_IDS**: Ordered list of epic IDs to process (respects dependency order)
- **PLAN_DIR**: Directory containing plan files for each sub-project
- **DESIGN_DIR**: Directory containing design specs for each sub-project
- **PROJECT_ROOT**: Absolute path to the project root
- **BRANCH**: Git branch to work on (or `main` for direct work)
- **QUALITY_CMD**: Build/test command (default: `go build ./...`)

---

## Pre-flight Checks

### 1. Verify branch and working tree

```bash
git status
git --no-pager log --oneline -3
```

Confirm you're on the correct branch and it's clean.

### 2. Check epic status

For each epic in EPIC_IDS:

```bash
bd epic status {EPIC_ID}
```

Identify which epics are fully closed (skip them), which have open tasks.
Find the first epic with open, unblocked tasks — that's where you start.

### 3. Check for blocked tasks

```bash
bd blocked 2>/dev/null
```

Note any cross-epic blockers. Tasks blocked by tasks in earlier epics cannot
start until those epics complete.

### 4. Identify plan and design files

For the current epic, locate the corresponding plan file and design spec.
The plan file contains detailed implementation instructions per task. The design
spec has architecture decisions. Both are passed to sub-agents.

---

## The Outer Loop (Per Epic)

Process epics in the order given by EPIC_IDS. For each epic:

### Step 0: Check Epic Status

```bash
bd epic status {EPIC_ID}
```

- If all tasks are closed -> skip this epic, move to the next
- If tasks remain -> proceed to the inner loop

### Step 0.1: Read the Plan File (Once Per Epic)

Use an Explore agent to extract the key patterns and reference points from
the plan file. Do NOT read the plan file yourself.

```text
Agent(subagent_type="Explore",
  prompt="Read {PLAN_FILE} for epic {EPIC_ID}.
  Extract and return:
  1. Implementation order and parallelism notes
  2. Key reference files and line numbers mentioned
  3. Any implementation standards or constraints
  4. The dependency graph between tasks
  Keep your response under 2000 chars — summaries only, no full code blocks.")
```

Store the summary. It feeds into every task's sub-agent prompt for this epic.

---

## The Inner Loop (Per Task)

For each open, unblocked task in the current epic (in dependency order):

### Step 1: Claim the Task

```bash
bd update {TASK_ID} --claim
bd show {TASK_ID}
```

Read the task description. It contains:
- Files to create/modify
- Steps to implement
- Acceptance criteria

### Step 2: Research + Prior Art + Root Cause (Explore Agent)

Spawn an Explore agent to understand the target code AND check for existing
solutions and root causes. This is the CRITICAL step that prevents the agent
from reinventing existing code or fixing symptoms instead of causes.

```text
Agent(subagent_type="Explore",
  prompt="In {PROJECT_ROOT}:

  ## TARGET CODE
  1. Find and read the files listed in this task: {FILES_FROM_TASK}
  2. Identify existing patterns for {WHAT_THIS_TASK_DOES}
  3. Check the plan file section if available

  ## MANDATORY: PRIOR ART SEARCH
  Before ANY implementation, search for existing solutions:
  4. Search shared packages for functions/utilities that already solve this:
     - grep for similar function names in: internal/contracts/, internal/foundation/,
       pkg/, web/shared/, internal/foundation/httputil/
     - Check the Shared Code Catalog in CLAUDE.md
     - If you find an existing utility that solves this, report it prominently.
       The implement agent MUST use it instead of writing new code.

  ## MANDATORY: ROOT CAUSE ANALYSIS
  5. Is the task description addressing a symptom or the root cause?
     - What is the actual root cause of this bug/issue?
     - Would the proposed fix prevent recurrence, or just patch this instance?
     - If it's a symptom fix: describe what a root cause fix would look like.

  ## MANDATORY: OTHER INSTANCES
  6. Are there other places in the codebase with this same pattern/vulnerability?
     - grep for the vulnerable/broken pattern across the entire codebase
     - List ALL instances, not just the one in the task
     - The implement agent should fix ALL of them, not just one.

  ## Return Format
  (a) Current code structure summary
  (b) Exact patterns to follow
  (c) All files that need modification with current line counts
  (d) PRIOR ART: Existing utilities found (or 'none — new code justified')
  (e) ROOT CAUSE: Root cause vs symptom assessment
  (f) OTHER INSTANCES: List of all matching instances in codebase (or 'only one')
  ")
```

### Step 3: Implement (General-Purpose Agent)

Spawn an implementation agent with full context from the task AND the prior art
/ root cause findings.

```text
Agent(subagent_type="general-purpose",
  prompt="Implement the following task in {PROJECT_ROOT}.

  ## Task
  {PASTE FULL TASK DESCRIPTION FROM bd show}

  ## Context from Research
  {PASTE KEY FINDINGS from Step 2}

  ## Prior Art (from Research)
  {PASTE PRIOR ART FINDINGS — if existing utilities were found, USE THEM}

  ## Root Cause (from Research)
  {PASTE ROOT CAUSE ASSESSMENT — fix the root cause, not just the symptom}

  ## Other Instances (from Research)
  {PASTE OTHER INSTANCES — fix ALL of them, not just the one in the task}

  ## Plan File Details
  {PASTE RELEVANT SECTION from the plan file — the Explore agent extracted this}

  ## Implementation Standards
  - Follow existing code patterns exactly
  - USE existing utilities from shared packages — do NOT reinvent
  - Fix the ROOT CAUSE, not just the symptom described in the task
  - Fix ALL instances of the pattern, not just the one mentioned
  - Do NOT modify files not listed unless necessary for compilation or
    fixing other instances of the same pattern
  - Use the shared patterns from CLAUDE.md (TemplateFuncs, LoadTierTemplates, etc.)

  ## MANDATORY Security Checklist (read .claude/skills/sub-agent-loop/security-checklist.md)
  Before committing, verify your changes against these recurring patterns:
  1. ALL handlers in the file have RBAC checks (including Validate*/Check*/Test*/Apply* utilities)
  2. No raw err.Error() in API responses or rendered HTML — use generic messages
  3. No user-supplied input (names, IDs) reflected in error messages
  4. URL validation uses url.Parse, never string matching on raw URLs
  5. Proxy headers (X-Real-IP, X-Forwarded-For) only trusted behind isTrustedProxy()
  6. HTTP clients have CheckRedirect hooks for SSRF protection
  7. Content-Disposition filenames sanitized with regex, not HTMLEscapeString
  8. Cookie set/clear attributes match (SameSite, Secure, Path)
  9. Claims.Type always set to 'access' explicitly
  10. MaxBytesReader applied BEFORE any handler logic, not after session lookups
  11. DEV_MODE checks use == 'true', never != ''
  12. New features evaluated for attack surface (CLI flags with secrets, new endpoints need RBAC)
  13. Secrets (API keys, tokens, passwords, client secrets) NEVER rendered as plain text in templates —
      must use type="password" inputs or masked display (e.g. "sk-****...3bf2"). Even view-only fields
      must be masked. Rendering {{ .APIKey }} as a <p> or type="text" is a security vulnerability.

  ## Deliverables
  1. Implement the change
  2. Run: {QUALITY_CMD}
  3. Fix any build errors
  4. Commit: git add <specific-files> && git commit -m '<type>({EPIC_SHORT}): <description>'

  ## Quality
  - All acceptance criteria from the task must be met
  - Build must pass before committing
  - If tests are specified in the task, run them and confirm they pass")
```

### Step 4: Self-Audit (Background Agent)

Immediately after Implement commits, spawn a lightweight self-audit agent that
reviews ONLY the diff for common issues the implement agent might have
introduced. This runs in parallel with the build verification.

```text
Agent(subagent_type="general-purpose", model="sonnet", run_in_background=true,
  prompt="In {PROJECT_ROOT}, review ONLY the last commit's diff for issues
  the implementing agent may have introduced.

  Run: git --no-pager diff HEAD~1

  Check the diff for these specific failure patterns:

  ## 1. CONCURRENCY SAFETY
  Does the new code touch shared state (maps, slices, struct fields that could
  be accessed from multiple goroutines)? If yes, is it protected by
  mutex/atomic/channel? Flag any unprotected shared state. Look for:
  - New map[...] fields on structs without sync.Mutex
  - New global variables without sync protection
  - Goroutine launches that capture mutable variables

  ## 2. NEW VULNERABILITIES
  Could this diff be flagged by a security auditor? Check for:
  - Unvalidated user/external input used in URLs, SQL, commands, file paths
  - Missing bounds checks (array access, io.ReadAll without limit)
  - Error paths that leak internal details to clients (raw err.Error() in responses)
  - New string formatting in error responses (fmt.Sprintf with raw errors)
  - HMAC/password comparison without constant-time functions
  - User-supplied names/IDs reflected in error messages
  - Secrets (API keys, tokens, passwords) rendered as plain text in templates
    (must use type="password" or masked display — NEVER type="text" or raw {{ .Value }})
  - MANDATORY: If the diff touches a web handler file, check that ALL handlers
    in that file have getPagePermissions + CanAccess checks — especially
    Validate*, Check*, Test*, Apply* utility endpoints (recurring audit finding)

  ## 3. DUPLICATION CHECK
  Did the diff create a function that already exists in the codebase?
  For each new function/method defined in the diff:
  - grep for similar function names in internal/contracts/, internal/foundation/,
    pkg/, web/shared/
  - Flag if an existing utility could have been used instead

  ## 4. ERROR HANDLING
  Are all new error returns checked by callers? Look for:
  - _ = err patterns (silenced errors)
  - Functions that return error but callers ignore the return
  - Error messages that include raw internal state

  ## 5. RESOURCE MANAGEMENT
  - Any new io.ReadAll without size limit?
  - Any new http.Get/Post without timeout or URL validation?
  - Any new file/connection opens without corresponding Close/defer?

  Report: PASS if no issues found, or list specific issues with file:line.
  Be concise — only flag genuine issues with confidence >= 80%.")
```

### Step 5: Verify (Background Agent)

Also spawn the build verification agent in parallel with the self-audit.

```text
Agent(subagent_type="general-purpose", model="haiku", run_in_background=true,
  prompt="In {PROJECT_ROOT}:
  1. Run: {QUALITY_CMD}
  2. Run: git --no-pager diff HEAD~1 --stat
  3. Check the acceptance criteria for {TASK_ID}: {ACCEPTANCE_CRITERIA_SUMMARY}
  4. Report PASS or list specific issues found")
```

### Step 6: Handle Self-Audit + Verification Results

Wait for BOTH the self-audit and verification agents to return.

**If both PASS**: Close the task and move on.

**If self-audit finds issues**:
1. If CONCURRENCY or VULNERABILITY issues (HIGH severity): spawn a fix agent
   with the specific finding. Re-run self-audit after fixing.
2. If DUPLICATION issues (MEDIUM): log as a beads comment on the task for
   awareness but don't block — the refactoring loop at epic completion will
   catch cross-cutting DRY issues.
3. If ERROR HANDLING issues (MEDIUM): fix them in the same commit (amend or
   new commit).

**If verification fails**: Same as before — fix agent, max 3 attempts, revert.

```bash
bd close {TASK_ID}
```

### Step 7: Progress Report

After closing each task, log progress:

```bash
bd epic status {EPIC_ID}
```

Print: "Completed {TASK_ID}. {X}/{TOTAL} tasks done in {EPIC_ID}."

After every 3 tasks, do a fuller status update:

```bash
bd stats
```

### Step 8: Check for Parallelism

Before returning to Step 1, check if multiple tasks are now unblocked:

```bash
bd ready 2>/dev/null | grep {EPIC_ID}
```

If 2+ independent tasks are unblocked (different files, no shared state),
launch their Step 3 (Implement) agents in parallel. Verify each independently.

### Step 9: Loop Back

Return to Step 1 with the next open, unblocked task.

---

## Epic Completion Gate

When all tasks in an epic are closed:

### 1. Full build verification

```bash
go build ./...
```

### 2. Run scoped tests

```bash
# Run tests for packages modified by this epic
{EPIC_SCOPED_TEST_CMD}
```

### 3. Dual Code Review (MANDATORY)

Run BOTH reviews in parallel as sub-agents. Both must pass before the epic
can be closed. This catches shortcuts, spec deviations, and security issues
that individual task verification misses.

```text
# Launch BOTH in parallel (single message, two Agent calls):

Agent(subagent_type="feature-dev:code-reviewer",
  prompt="Review all files changed in epic {EPIC_ID} in {PROJECT_ROOT}.
  Scope: git diff {EPIC_START_COMMIT}..HEAD
  Focus: security (XSS, injection, auth bypass), correctness (nil panics,
  error handling, resource leaks), architecture (real services not hardcoded),
  Go idioms, test quality. Report as numbered list with file:line, severity
  (CRITICAL/HIGH/MEDIUM/LOW), issue, fix. Only genuine issues.")

Agent(subagent_type="general-purpose",
  prompt="Cross-reference implementation against plan in {PROJECT_ROOT}.
  Plan: {PLAN_FILE}. Design: {DESIGN_FILE}.
  Read ALL changed files: git diff {EPIC_START_COMMIT}..HEAD --name-only
  For each plan task, verify: endpoint paths, request/response shapes,
  error codes, pagination, test approach, constraints. Report as numbered
  list: expected (from plan), implemented, matches?, severity of gaps.")
```

When both return:
- **0 CRITICAL + 0 HIGH**: Proceed to close the epic.
- **Any CRITICAL or HIGH**: Fix them before closing. Spawn fix agents, re-run
  the failing review, then proceed. File MEDIUM/LOW issues as beads tasks
  (P3/P4) for later — don't block the loop.
- Consolidate findings into a single numbered list and log as a beads comment
  on the epic: `bd comments {EPIC_ID} add "Review: X findings (Y fixed, Z filed)"`

### 4. Refactoring Loop (MANDATORY)

After code review passes, run the refactoring loop against the epic's changes
to catch DRY violations, dependency graph issues, and reuse opportunities
across the entire epic (not just individual tasks).

```text
Agent(subagent_type="general-purpose",
  prompt="Run a refactoring analysis on the changes in epic {EPIC_ID} in {PROJECT_ROOT}.

  ## Scope
  Get the changed files: git diff {EPIC_START_COMMIT}..HEAD --name-only

  ## Checks

  ### 1. DRY Violations Across Tasks
  Did multiple tasks in this epic create similar code that should be consolidated?
  Look for: duplicated helper functions, repeated validation patterns, similar
  error handling, copy-pasted boilerplate across files changed in this epic.

  ### 2. Dependency Graph
  Do any new imports violate the layer rules?
  - internal/contracts/ can only import pkg/types/ and stdlib
  - internal/foundation/ can only import contracts, types, stdlib
  - internal/*/services/ can only import contracts, foundation, own localstore
  - web/*/handlers/ can only import own tier services, web/shared, contracts
  Check: grep -n 'import' in each changed .go file, verify against rules.

  ### 3. Existing Code Reuse
  Did any task create new code that duplicates existing shared utilities?
  Search for similar functions in: internal/contracts/, internal/foundation/,
  pkg/, web/shared/, internal/foundation/httputil/

  ### 4. Anti-Pattern Detection
  Flag any of these in the changed files:
  - fmt.Sprintf with raw error in HTTP responses (info disclosure)
  - io.ReadAll without size limit
  - http.Get/Post without URL validation
  - map access without mutex in concurrent context
  - _ = err (silenced errors)
  - Hardcoded model/provider names as string literals

  Report findings as: file:line, issue, severity (HIGH/MEDIUM/LOW), fix.
  If no issues found, report PASS.")
```

### 5. Security Spot-Check (MANDATORY)

Run a focused security review of the epic's diff, specifically checking for
the failure patterns that sub-agents commonly introduce.

```text
Agent(subagent_type="feature-dev:code-reviewer",
  prompt="SECURITY SPOT-CHECK on epic {EPIC_ID} changes in {PROJECT_ROOT}.
  Scope: git diff {EPIC_START_COMMIT}..HEAD

  Check ONLY for these specific high-value patterns:

  1. Any new sync.Map or map[] without mutex/atomic protection
  2. Any new io.ReadAll without io.LimitReader
  3. Any new http request construction without URL validation
  4. Any new fmt.Sprintf/fmt.Errorf in error messages returned to HTTP clients
  5. Any _ = err (silenced error returns)
  6. Any token/secret/HMAC comparison without constant-time function
  7. Any new shared state (global vars, struct fields) accessed from goroutines
  8. Any new template rendering without html/template (using text/template)

  For each match, report: file:line, pattern matched, severity, fix.
  If none found, report CLEAN.")
```

### 6. Handle Refactoring + Security Results

- **Refactoring HIGH findings**: Fix before closing the epic.
- **Security findings**: Fix CRITICAL/HIGH before closing. File MEDIUM/LOW.
- **Refactoring MEDIUM/LOW**: File as beads tasks for later.

### 7. Commit any final fixes

If the build, tests, reviews, or refactoring/security checks needed fixes:

```bash
git add <files>
git commit -m "fix({EPIC_SHORT}): post-completion review fixes"
```

### 8. Close the epic

```bash
bd close {EPIC_ID}
```

### 9. Check what's unblocked

```bash
bd close {EPIC_ID} --suggest-next
```

This shows tasks in later epics that were blocked by this epic and are now
unblocked. Log them for awareness.

### 10. Move to next epic

Return to the Outer Loop with the next epic in EPIC_IDS.

---

## Full Completion

When all epics are processed:

### 1. Final build gate

```bash
go build ./...
go test ./... -timeout 180s 2>&1 | tail -30
```

### 2. Summary report

Print:
- Epics completed: X/Y
- Tasks completed: X/Y
- Tasks failed/skipped: list them with reasons
- Self-audit issues caught: X (concurrency: N, vulnerability: N, duplication: N)
- Files changed (total): `git --no-pager diff {START_COMMIT}..HEAD --stat | tail -5`
- Commits made: `git --no-pager log {START_COMMIT}..HEAD --oneline | wc -l`

### 3. Push

```bash
git push
```

---

## Key Rules

1. **Never read source code in main context.** Delegate ALL file reading and
   code writing to sub-agents. Your context is for coordination only.

2. **Build must pass after every task.** Never leave the branch broken.

3. **One task at a time (unless parallelizable).** Complete and close each task
   before starting the next. Only parallelize when tasks touch different files
   and have no dependency.

4. **Respect the dependency DAG.** Never start a task whose blockers aren't
   closed. Check `bd blocked` when in doubt.

5. **Revert rather than leave broken.** If a task can't be completed in 3
   attempts, revert and move on. A clean build is worth more than a partial
   implementation.

6. **Epic order is strict.** Process epics in EPIC_IDS order. Don't skip ahead
   even if later epics have unblocked tasks — earlier epics provide the
   foundation.

7. **Context stays lean.** If you find yourself reading files or writing code,
   STOP. Spawn a sub-agent instead.

8. **Log progress to beads.** Use `bd comments` for notable decisions or issues
   encountered during a task. Future sessions can recover context from these.

9. **Push periodically.** After completing each epic (or every 5-6 tasks if an
   epic is large), push to remote so work is preserved.

10. **Dual review at every epic gate.** Run BOTH a code quality review
    (feature-dev:code-reviewer) AND a plan cross-reference (general-purpose)
    before closing any epic. This is NON-NEGOTIABLE. Without it, sub-agents
    take shortcuts and spec deviations compound across epics. Fix CRITICAL
    and HIGH findings before proceeding; file MEDIUM/LOW as beads tasks.

11. **Fix test failures you encounter.** If a test fails during your work —
    whether pre-existing or new — and you can fix it, fix it. Don't ignore
    failing tests. The only acceptable reason to skip a test failure is if
    it requires infrastructure you cannot provide (e.g., external service
    credentials). In that case, file a beads task for it and move on.

12. **Prior art before new code.** The Research step MUST search for existing
    utilities before the Implement agent writes new code. If an existing
    function in contracts/, foundation/, or shared/ solves the problem, USE IT.
    Creating duplicate utilities is a defect, not a feature.

13. **Root cause over symptom.** The Research step MUST identify whether the
    task addresses the root cause or a symptom. The Implement agent should fix
    the root cause whenever feasible. If the root cause fix is too large for
    one task, fix the symptom AND file a beads task for the root cause.

14. **All instances, not just one.** When fixing a pattern (e.g., missing RBAC
    check, timing oracle, unvalidated input), the Research step MUST search
    for ALL instances in the codebase. The Implement agent fixes ALL of them,
    not just the one mentioned in the task description.

15. **Self-audit catches what implement misses.** The self-audit step runs on
    every commit. Concurrency and vulnerability findings are blocking — they
    must be fixed before the task closes. Duplication findings are logged but
    deferred to the refactoring loop at epic completion.

16. **Refactoring loop + security spot-check at every epic gate.** After dual
    code review passes, run the refactoring analysis AND a focused security
    spot-check on the epic's diff. This catches DRY violations across tasks
    and the specific vulnerability patterns that sub-agents commonly introduce
    (data races, unbounded reads, SSRF, silenced errors). Fix HIGH+ before
    closing the epic.
