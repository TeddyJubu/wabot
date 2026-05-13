---
name: coding-agents-workflows
description: Guides implementation-first workflows for coding agents across Codex/Cursor-style agent tasks, including repo inspection, scoped edits, verification, and safe git operations. Use when the user asks to implement code changes, run terminal-driven development tasks, or execute coding-agent workflows.
disable-model-invocation: true
---

# Coding Agents Workflows

## Goal

Deliver code tasks end-to-end with minimal back-and-forth: inspect, implement, verify, and report.

## Workflow

1. Clarify the request only when blocked; otherwise start implementation.
2. Inspect repository state first:
   - Current branch and dirty files
   - Relevant files, symbols, and entry points
   - Existing tests and validation commands
3. Implement the smallest complete change that satisfies the request.
4. Run targeted verification:
   - Prefer file- or package-scoped tests first
   - Run lint/typecheck/build only when relevant
5. Report what changed, how it was verified, and any remaining risk.

## Tooling Defaults

- Prefer direct file tools for reading/editing code.
- Use shell for git, builds, tests, package manager actions, and runtime checks.
- Avoid destructive git operations unless explicitly requested.
- Do not commit unless the user asks for a commit.

## Git Safety

- Never rewrite shared history unless explicitly requested.
- Before merge or release actions, confirm:
  - branch divergence from `main`
  - fast-forward possibility or merge strategy
  - local working tree cleanliness
- If push fails due to remote movement, re-sync (`fetch`/`pull --rebase`) and retry safely.

## Implementation Heuristics

- Prefer narrow, reversible edits over broad refactors.
- Keep naming and style consistent with nearby code.
- Add comments only for non-obvious logic.
- If requirements are ambiguous, choose a safe default and document it.

## Verification Checklist

Copy this checklist into your working notes for substantial tasks:

```markdown
Task Progress:
- [ ] Confirm scope and constraints
- [ ] Inspect repo + relevant files
- [ ] Implement code changes
- [ ] Run targeted verification
- [ ] Summarize results and risks
```

## Response Format

Use concise sections:

1. What changed
2. Why this approach
3. Verification run (commands + outcomes)
4. Optional next steps

## Out of Scope

- Product/project management workflows
- Framework-specific API documentation dumps
- Large architecture rewrites unless explicitly requested
