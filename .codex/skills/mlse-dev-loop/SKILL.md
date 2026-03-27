---
name: mlse-dev-loop
description: Use when changing code in the MLSE repository. Follow the default engineering loop for this repo: read AGENTS and relevant design docs, make focused edits, run targeted checks for the touched area, then run repository-wide tests and staticcheck, and finish with scripts/lint.sh. Sync repo docs and Obsidian mlse设计 notes when frontend, dialect, pass, test, lint, or build boundaries change.
---

# MLSE Dev Loop

Use this skill for normal implementation work in this repository.

## Before Editing

1. Read `AGENTS.md`.
2. Read the relevant repo docs under `docs/`.
3. If the change touches frontend boundaries, dialects, passes, runtime, testing, lint, or build behavior, also read the matching notes under `mlse设计`, especially `02-GoIR/`.

## Edit

1. Keep edits focused and consistent with the current formal-bridge-first direction.
2. Put generated intermediates only under `artifacts/` or `tmp/`.
3. If the change alters support boundaries or workflow defaults, plan to sync repo docs and Obsidian notes in the same turn.

## Validation Order

Run checks in this order after code changes:

1. Targeted checks for the touched area.
   Common examples:
   - `go test ./linters`
   - `go test ./internal/gofrontend`
   - `go build -o artifacts/bin/mlse-go ./cmd/mlse-go`
   - `python3 scripts/go-gobench-mlir-suite.py --skip-build`
2. Repository-wide Go checks:
   - `go test ./cmd/... ./internal/...`
   - `staticcheck ./cmd/... ./internal/...`
   - use `scripts/test-all.sh` when you need the repo-wide unified test entry, especially after changing test/build/bridge behavior
3. Finish with repo lint:
   - `scripts/lint.sh`

If one step fails, fix the code and rerun the affected checks before moving on.

## Close-Out

1. Report which checks were run and whether they passed.
2. Call out any checks that were skipped.
3. If behavior or support boundaries changed, sync:
   - `README.md`
   - relevant files in `docs/`
   - relevant notes under `mlse设计/`
