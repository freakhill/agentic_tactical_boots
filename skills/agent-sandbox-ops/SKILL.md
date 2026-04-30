---
name: agent-sandbox-ops
description: >
  Operate the local sandbox toolchain safely: Docker sandbox, Tart brew VM,
  network-limiting checks, and explicit host file sharing.
---

# Agent Sandbox Ops Skill

Use this skill whenever tasks involve runtime isolation, network limiting, or file transfer between host and sandboxed runtimes.

## Required pre-read

Before executing this skill, read:

1. `CONTRIBUTING.md`
2. `agents.md`
3. `scripts/CONVENTIONS.md`
4. `README.md`

## Command map

- Hub: `scripts/sandboxctl.fish`
- Docker runtime: `scripts/agent-sandbox.fish`, `scripts/agent-sandbox-tools.fish`
- VM runtime: `scripts/brew-vm.fish`

## Default policy

1. Prefer `strict-egress` network policy.
2. Keep domain access allowlisted via `examples/allowlist.domains`.
3. Use explicit file transfer for VM (`copy-in`, `copy-out`) and avoid secret transfer.

## Workflows

### Docker sandbox workflow

1. `scripts/sandboxctl.fish docker up`
2. `scripts/sandboxctl.fish docker shell`
3. Verify non-allowlisted egress is blocked from agent runtime.
4. `scripts/sandboxctl.fish docker down`

### Brew VM workflow

1. `source scripts/brew-vm.fish`
2. `set -x BREW_VM_PROXY_URL http://<proxy-host>:3128`
3. `brew-vm create-base`
4. `brew-vm install --network-policy strict-egress <formula>`
5. `brew-vm verify-network`

### File sharing workflow

- Docker: use `/workspace` mount.
- VM: use `brew-vm copy-in <host-path> <guest-path>` and `brew-vm copy-out <guest-path> <host-path>`.
- Recommended guest temp path: `/tmp/llm-share`.

## Safety checklist

- Do not mount host credential directories into containers/VMs.
- Do not disable strict egress for untrusted installs.
- Document every allowlist expansion with reason.

## Sync requirements after changes

If you change sandbox scripts or defaults, update in the same task:

- `README.md`
- this skill file
- any other affected skill under `skills/*/SKILL.md`
- `skills/README.md` when installation/usage guidance changes
