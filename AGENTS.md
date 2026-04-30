# Agent Operating Contract

This file defines repository-level behavior for human and LLM agents.

## Mandatory Read Order

Before making changes, agents must read:

1. `CONTRIBUTING.md`
2. `scripts/CONVENTIONS.md`
3. `llm-agent-sandboxing.md`
4. Relevant skill files in `skills/`

## Required Behaviors

- Keep command UX and safety defaults consistent across scripts.
- Treat network limiting as a first-class control; do not weaken defaults silently.
- Preserve explicit host file-sharing boundaries for VM/container workflows.
- Use comment best practices (why-focused, concise, linked to official references where needed).

## Skills and Docs Must Stay In Sync

Any script behavior/interface change requires matching updates in:

- `llm-agent-sandboxing.md`
- Affected skill files (`skills/*/SKILL.md`)
- `skills/README.md` when usage/install guidance changes

If updates are not synchronized, the task is incomplete.

## Done Checklist

1. Script help paths are updated (`help`/`--help`).
2. Documentation examples still match real commands.
3. Skill workflows reflect new command behavior/defaults.
4. `fish -n scripts/*.fish` passes.
