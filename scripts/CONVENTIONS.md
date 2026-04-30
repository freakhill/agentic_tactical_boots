# Script Conventions

This folder uses a single operational style to keep scripts easy to read, modify, and debug by hand.

## Required interface

- Every user-facing script supports `--help` and `help`.
- Help output includes: `Usage`, key commands/options, and at least one short safety note.
- Prefer subcommand style over positional ambiguity (for example: `tool run ...`, `tool list ...`).

## Comment best practices

- Add a short top-of-file block describing:
  - purpose
  - key safety/model assumptions
  - official documentation links
- Add function-level comments only when behavior is non-obvious or safety-critical.
- Explain **why** a pattern exists, not what obvious shell syntax does.
- Keep comments stable: avoid version-pinned claims unless necessary.

## Safety defaults

- Default network policy should be `strict-egress` for untrusted execution paths.
- Keep deny-by-default egress and explicit domain allowlists.
- For VM workflows, prefer explicit file transfer (`copy-in`/`copy-out`) over broad host mounts.
- For key lifecycle workflows, prefer short-lived credentials and clear revocation paths.

## Parameter naming standards

Use these names consistently where applicable:

- `--name`: human-readable label/session
- `--id`: object identifier (key id, identity id)
- `--repo`: `owner/repo`
- `--access`: `ro|rw`
- `--ttl`: duration (`30m`, `24h`, `7d`)
- `--yes`: non-interactive confirmation
- `--force`: overwrite/bootstrap replacement
- `--network-policy`: `strict-egress|proxy-only|off`

If a domain requires custom identifiers (for example `--rid`), keep aliases where possible and document clearly in help.

## Validation checklist before merge

1. `fish -n scripts/*.fish`
2. Run each script's help path (`<script> --help` or equivalent)
3. For network-related scripts, run at least one allow and one block verification path
4. Confirm docs reference any new commands/options
