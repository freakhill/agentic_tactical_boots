# Contributing

Thanks for contributing.

## First Read

Before editing code or docs, read:

1. `agents.md`
2. `scripts/CONVENTIONS.md`
3. `README.md`

## Script Standards

- Keep script UX consistent (`help`/`--help`, predictable subcommands, stable flags).
- Prefer safe defaults (for sandbox and network-sensitive workflows this means `strict-egress`).
- Add comments that explain **why**, not obvious shell syntax.
- Include official documentation links in script headers when behavior is non-obvious.

## Skills and Docs Sync Policy

When changing any script under `scripts/` that affects behavior, flags, workflows, or defaults, update all relevant docs in the same change:

- `README.md`
- Related skill files under `skills/*/SKILL.md`
- `skills/README.md` if install/use guidance changes

Do not merge behavior changes where skills/docs are stale.

## Verification

Run at least:

```fish
fish -n scripts/*.fish
```

For command-surface changes, also verify help output paths still work.

## Network and File-Sharing Guardrails

- Keep deny-by-default egress and explicit allowlists.
- Do not broaden allowlist domains without rationale.
- For VM paths, prefer explicit `copy-in`/`copy-out`; avoid broad host mounts.
- Never introduce defaults that expose host credential directories.
