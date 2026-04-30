# CrewAI Sandbox Toolkit

Local scripts and docs for running agent workflows with stronger isolation defaults:

- container sandbox helpers
- VM-backed Homebrew evaluation
- strict installer wrappers
- ephemeral key/identity lifecycle helpers (GitHub, Forgejo, Radicle)

## Quick start

```fish
source .venv/bin/activate.fish
scripts/sandboxctl.fish help
```

## Contributor policy (important)

Before edits, read:

1. `CONTRIBUTING.md`
2. `agents.md`
3. `scripts/CONVENTIONS.md`

When changing `scripts/*.fish`, keep docs and skills synchronized in the same change:

- `llm-agent-sandboxing.md`
- affected `skills/*/SKILL.md`
- `skills/README.md` when usage/install guidance changes

CI enforces this via `.github/workflows/script-doc-sync-check.yml`.
