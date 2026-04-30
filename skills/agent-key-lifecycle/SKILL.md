---
name: agent-key-lifecycle
description: >
  Manage ephemeral GitHub/Forgejo deploy keys and Radicle local identities
  with short TTLs, scoped access, and clean revocation.
---

# Agent Key Lifecycle Skill

Use this skill when creating, listing, rotating, or revoking keys/identities for automation agents.

## Required pre-read

Before executing this skill, read:

1. `CONTRIBUTING.md`
2. `agents.md`
3. `scripts/CONVENTIONS.md`
4. `README.md`

## Command map

- GitHub: `scripts/llm-github-keys.fish` (`llm-gh-key ...`)
- Forgejo: `scripts/llm-forgejo-keys.fish` (`llm-forgejo-key ...`)
- Radicle: `scripts/llm-radicle-access.fish` (`llm-radicle-access ...`)

## Defaults

1. Use separate RO and RW credentials.
2. Prefer short TTLs (default `24h`) and revoke aggressively.
3. Install SSH config aliases for explicit remote intent.

## Workflows

### GitHub key pair

1. `source scripts/llm-github-keys.fish`
2. `llm-gh-key create-pair --repo <owner>/<repo> --name session-1 --ttl 24h --install-ssh-config`
3. Use `git@github-llm-ro:<owner>/<repo>.git` for read-only operations.
4. Revoke with `llm-gh-key revoke-expired --repo <owner>/<repo> --yes`.

### Forgejo key pair

1. `source scripts/llm-forgejo-keys.fish`
2. `llm-forgejo-key bootstrap-config`
3. `llm-forgejo-key create-pair --instance main --repo <owner>/<repo> --name session-1 --ttl 24h --install-ssh-config`
4. Revoke by id/title/expiration as needed.

### Radicle identities across multiple repos

1. `source scripts/llm-radicle-access.fish`
2. `llm-radicle-access create-identity --name session-1 --ttl 24h`
3. `llm-radicle-access bind-repo --rid <rad:...> --identity-id <id> --access ro|rw`
4. `llm-radicle-access retire-expired --yes`

## Safety checklist

- Never use long-lived RW keys unless required.
- Keep branch protections/rulesets active for RW deploy keys.
- Remove stale SSH alias blocks after revocation.

## Sync requirements after changes

If you change key/identity script behavior, update in the same task:

- `README.md`
- this skill file
- any other affected skill under `skills/*/SKILL.md`
- `skills/README.md` when installation/usage guidance changes
