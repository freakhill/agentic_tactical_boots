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

The fish wrappers above delegate JSON / state / datetime work to small Python
helpers under `scripts/_py/llm_*.py`. Each helper carries PEP-723 inline
metadata pinning the interpreter, and is invoked via `uv run --script`.

## Required tools

- `ssh-keygen` (everywhere)
- `gh` (GitHub workflow only)
- `curl` (Forgejo workflow only)
- `uv` (everywhere — runs the pinned Python helpers; replaces bare `python3`)

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

### GitHub key pair — repo-aware shortcuts

When invoked from inside the target repo's working tree, `llm-gh-key here ...`
infers `--repo` from the cwd's `origin` remote (handles HTTPS, SSH, and
`github-*` ssh-config aliases) and supplies sensible defaults:

- `llm-gh-key here create-pair` — RO+RW pair, 24h TTL, auto name
  (`auto-<short-sha>-<utc-date>`), `--install-ssh-config` enabled by default
  (override with `--no-install-config`).
- `llm-gh-key here list` — list deploy keys for the current repo.
- `llm-gh-key here revoke <id>` — revoke a single key by id.
- `llm-gh-key here cleanup` — `revoke-expired --yes` for the current repo.
- `llm-gh-key here revoke-all` — `revoke-by-title '^llm-agent:' --yes` for the
  current repo (destructive; confirm explicitly).

Falls back to a clear error with the underlying CLI flag to use if the cwd is
not a git repo or the origin is not a recognized GitHub URL.

### Interactive flows

Two TUIs are available, both teachable (each action prints its equivalent CLI
before executing):

- `slop` — global launcher across every tool in this repo. Hard-deps on
  [`gum`](https://github.com/charmbracelet/gum). Install with `brew install gum`.
- `llm-gh-key tui` — focused per-tool launcher for the current repo's deploy
  keys. Soft-deps on `gum` (graceful install hint if missing).

### Forgejo key pair

1. `source scripts/llm-forgejo-keys.fish`
2. `llm-forgejo-key bootstrap-config`
3. `llm-forgejo-key create-pair --instance main --repo <owner>/<repo> --name session-1 --ttl 24h --install-ssh-config`
4. Revoke by id/title/expiration as needed.

### Forgejo — repo-aware shortcuts and TUI

When invoked from a Forgejo-tracked repo's working tree:

- `llm-forgejo-key here create-pair` — RO+RW pair, infers `--instance` (looked
  up by host in the saved profiles file) and `--repo` from the cwd's origin.
  Auto name and 24h TTL by default; ssh-config installed.
- `llm-forgejo-key here list` / `here revoke <id>` / `here cleanup` /
  `here revoke-all`.
- `llm-forgejo-key tui` — focused per-tool TUI (soft-deps on gum).

If the host has no matching profile, the error message tells you to run
`bootstrap-config` and `instance-set --name <label> --url https://<host> --token-env <ENV>`.

### Radicle identities across multiple repos

1. `source scripts/llm-radicle-access.fish`
2. `llm-radicle-access create-identity --name session-1 --ttl 24h`
3. `llm-radicle-access bind-repo --rid <rad:...> --identity-id <id> --access ro|rw`
4. `llm-radicle-access retire-expired --yes`

### Radicle — repo-aware shortcuts and TUI

When invoked from a Radicle-tracked repo (one where `git config rad.id` is set
or `rad inspect` returns a RID):

- `llm-radicle-access here info` — print the inferred RID.
- `llm-radicle-access here bind --identity-id <id> --access ro|rw [--note text]`
- `llm-radicle-access here unbind [--identity-id <id>] [--yes]`
- `llm-radicle-access here list-bindings`
- `llm-radicle-access tui` — focused per-tool TUI (soft-deps on gum).

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
- `tests/test_llm_*.fish` and `tests/test_py_helpers.fish` for changed argv or error paths
- `scripts/_py/llm_*.py` if the JSON / state / datetime contract changes (and never reintroduce bare `python3` — keep things uv-managed)
