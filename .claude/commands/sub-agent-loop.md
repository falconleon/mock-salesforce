---
description: "Loop through implementation plan epics using sub-agent strategy — Research, Implement, Verify per task"
argument-hint: "[--epic EPIC_ID] [--all] [--from EPIC_ID]"
---

Starting the Sub-Agent Implementation Loop.

**First, check for existing progress.** Run `bd stats` and `bd list --type=epic`
to see the overall project state.

## Configuration

Determine which epics to process:

- If `--epic EPIC_ID` was provided: process only that single epic
- If `--all` was provided: process ALL open epics in dependency order
- If `--from EPIC_ID` was provided: start from that epic and continue through remaining
- If no argument: show the list of open epics and ask which to process

### CASE Pipeline Epic Order (Default)

The CASE pipeline has 5 sub-projects with this dependency chain:

```
A (Engine Foundation) --+
                        +--> E (Pipeline Definition)
B (Advanced Execution) -+
                        |
C (Pipeline Infra) -----+
                        |
D (Mock Data) ----------+
```

A and B can run in parallel (different files). C depends on A (schema v12).
D depends on C (prompt templates). E depends on all of A-D.

### Plan and Design Files

Each sub-project has a plan file and design spec in `docs/plans/`:

| Sub-project | Plan File | Design Spec |
|-------------|-----------|-------------|
| A | `docs/plans/2026-03-25-execution-engine-foundation-plan.md` | `docs/plans/2026-03-25-execution-engine-foundation-design.md` |
| B | `docs/plans/2026-03-25-advanced-execution-plan.md` | `docs/plans/2026-03-25-advanced-execution-design.md` |
| C | `docs/plans/2026-03-25-pipeline-infrastructure-plan.md` | `docs/plans/2026-03-25-pipeline-infrastructure-design.md` |
| D | `docs/plans/2026-03-25-mock-data-completeness-plan.md` | `docs/plans/2026-03-25-mock-data-completeness-design.md` |
| E | `docs/plans/2026-03-25-case-pipeline-definition-plan.md` | `docs/plans/2026-03-25-case-pipeline-definition-design.md` |

## Execution

Read the loop prompt from `.claude/skills/sub-agent-loop/prompt.md` and execute
it. The prompt will:

1. Check epic status to find where to start
2. For each epic (in dependency order):
   a. Read the plan file summary via Explore agent
   b. For each open task:
      - Claim it via beads
      - Research target code via Explore agent
      - Implement via general-purpose agent (with full task + plan context)
      - Verify via background agent (build + acceptance criteria)
      - Fix regressions (max 3 attempts, then revert)
      - Close the task
   c. Run epic completion gate (full build + scoped tests)
   d. Close the epic
3. Push and print summary

Use `Ctrl+C` to stop the loop at any time. Progress is tracked in beads — the
loop resumes cleanly from wherever it stopped.

Now read the prompt file and begin executing it:

Read the file `.claude/skills/sub-agent-loop/prompt.md` and follow its instructions exactly.
