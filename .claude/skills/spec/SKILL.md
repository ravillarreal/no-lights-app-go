---
name: spec
description: Gather requirements and write a spec for a feature. Use when starting a feature of non-trivial scope. Interviews the user to surface edge cases and tradeoffs, then writes a self-contained SPEC.md.
disable-model-invocation: true
---

## Purpose

Turn a rough idea into a self-contained, reviewable spec before any code is written. The interview step is what catches the things the user hasn't thought of yet.

## Workflow

### 1. Interview (AskUserQuestion)

Ask the user questions, digging into the hard parts, not the obvious ones. Cover at minimum:

- **What**: the outcome, not the implementation. What does "done" look like?
- **Scope**: what is explicitly out of scope?
- **Technical**: which files/packages are involved? data model changes? new endpoints?
- **Edge cases**: empty input, errors, concurrency, failure modes.
- **Tradeoffs**: performance vs simplicity, coupling vs duplication.
- **Constraints**: existing patterns to follow, libraries already in use.

Keep interviewing until the answers stop revealing new information. Prefer a handful of high-value questions over a long list of obvious ones.

### 2. Write SPEC.md

A good spec is **self-contained** — a fresh session can execute it without re-asking. Structure:

```markdown
# [Feature name]

## Goal
One or two sentences on the outcome.

## Scope
- In scope: ...
- Out of scope: ...

## Files & interfaces
- Files that change and the interfaces they expose/consume.

## Data model
- New tables, columns, or Redis keys.

## Behavior / edge cases
- Bullet list of concrete cases and expected results.

## Performance budget (if applicable)
- Expected query count per endpoint (matches the X-Query-Count harness).

## Verification
- End-to-end step that proves it works: tests to run, commands, what output to expect.
```

### 3. Hand off

Tell the user to start a fresh session and point it at `SPEC.md`. A clean context executing a precise spec beats a long session with accumulated corrections.

## When NOT to use

Skip this for one-line changes or tasks whose diff you could describe in a single sentence. For those, just do it directly.
