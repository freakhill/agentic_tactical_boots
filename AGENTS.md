# Agent Operating Contract

This file defines repository-level behavior for human and LLM agents.

## Mandatory Read Order

Before making changes, agents must read:

1. `CONTRIBUTING.md`
2. `scripts/CONVENTIONS.md`
3. `README.md`
4. Relevant skill files in `skills/`

## Required Behaviors

- Keep command UX and safety defaults consistent across scripts.
- Treat network limiting as a first-class control; do not weaken defaults silently.
- Preserve explicit host file-sharing boundaries for VM/container workflows.
- Use comment best practices (why-focused, concise, linked to official references where needed).
- All Python work goes through `uv` for isolation and repeatability. Helpers live in `scripts/_py/*.py` with PEP-723 inline metadata; fish wrappers invoke them as `uv run --script <file> <subcommand> ...`. Never reintroduce bare `python3 -c '...'` calls or `python3` as a `__require_tools` dependency.

## Skills, Docs, and Tests Must Stay In Sync

Any script behavior/interface change requires matching updates in:

- `README.md`
- Affected skill files (`skills/*/SKILL.md`)
- `skills/README.md` when usage/install guidance changes
- `tests/test_<script>.fish` for changed subcommands, flags, or error paths
- `scripts/_py/<helper>.py` (and `tests/test_py_helpers.fish`) when the Python helper contract for `llm-*.fish` scripts changes

If updates are not synchronized, the task is incomplete. CI enforces this via
`.github/workflows/script-doc-sync-check.yml`.

## Done Checklist

1. Script help paths are updated (`help`/`--help`).
2. Documentation examples still match real commands.
3. Skill workflows reflect new command behavior/defaults.
4. Test cases reflect new/changed argv handling.
5. `fish -n scripts/*.fish` passes.
6. `fish tests/run.fish` passes.
