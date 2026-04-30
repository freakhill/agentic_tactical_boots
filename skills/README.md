# Local Skills

Repo-versioned skills live here and can be installed into `~/.claude/skills`.

## Contributor expectations

Before updating any skill, read:

1. `CONTRIBUTING.md`
2. `agents.md`
3. `scripts/CONVENTIONS.md`

When script behavior changes, keep skills and docs in sync in the same change:

- `README.md`
- affected `skills/*/SKILL.md`
- this file when install/usage guidance changes

Install:

```fish
scripts/install-local-skills.fish
```

Replace existing:

```fish
scripts/install-local-skills.fish --force
```

Dry run:

```fish
scripts/install-local-skills.fish --dry-run
```

Install fish tool shims (stow preferred, direct fallback):

```fish
scripts/install-fish-tools.fish install
```
