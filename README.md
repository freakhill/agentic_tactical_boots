# Appropriate footwear for Agentic workflows

[![Tests](https://github.com/freakhill/agentic_tactical_boots/actions/workflows/tests.yml/badge.svg?branch=main)](https://github.com/freakhill/agentic_tactical_boots/actions/workflows/tests.yml?query=branch%3Amain+is%3Asuccess)

The badge shows the current state of the test suite on `main`. Click it to see
the list of successful runs on `main` — the topmost entry is the last commit
that passed CI.

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

## Install fish command shims

Install command shims into `~/.local/bin` (default target is `$HOME`):

```fish
scripts/install-fish-tools.fish install
```

Use a custom target directory:

```fish
scripts/install-fish-tools.fish install --target /path/to/target
```

How mode selection works:

- If `stow` is available, install stows the `fish-tools` package into `~/.local` (coexists with other tools in shared `~/.local`).
- If `stow` is unavailable or stow install fails, install falls back to managed direct wrapper files.
- If wrappers were installed directly and `stow` is installed later, first tool run auto-migrates to stow mode.
- Install also provides fish integration assets in `~/.local/share/fish`:
  - `vendor_conf.d/agentic_tactical_boots.fish` to auto-load PATH setup in new fish sessions
  - `vendor_completions.d/*.fish` for command autocompletion

Re-running `install` is safe and idempotent. The cleanup phase that removes stale managed files explicitly skips paths whose parent directory is a symlink (typical of a previous stow install with tree folding), so it cannot follow the symlink and delete files in the repo's stow source tree.

If `~/.local/bin` is not on `PATH`, installer output includes a fish snippet to add it.

Uninstall shims:

```fish
scripts/install-fish-tools.fish uninstall
```

## Contributor policy (important)

Before edits, read:

1. `CONTRIBUTING.md`
2. `agents.md`
3. `scripts/CONVENTIONS.md`

When changing `scripts/*.fish`, keep docs, skills, **and tests** synchronized in the same change:

- `README.md`
- affected `skills/*/SKILL.md`
- `skills/README.md` when usage/install guidance changes
- `tests/test_<script>.fish` for new/changed subcommands, flags, or error paths
- `scripts/_py/<helper>.py` and `tests/test_py_helpers.fish` when the Python helper contract changes

CI enforces this via `.github/workflows/script-doc-sync-check.yml`.

Run the test suite locally with:

```fish
fish tests/run.fish
```

### Python helpers run via `uv`

The `scripts/llm-*.fish` wrappers delegate JSON, datetime, and state-file work
to small Python helpers in `scripts/_py/llm_*.py`. Each helper carries
PEP-723 inline metadata (`requires-python`, `dependencies`) and is invoked as
`uv run --script "$HELPER_PY" <subcommand> ...` from fish. This keeps the
Python interpreter version pinned per helper and avoids relying on whatever
`python3` happens to be on `$PATH`. **`uv` is therefore a hard dependency of
the `llm-*` workflows; `python3` is not.** Any new Python work in this repo
must follow the same pattern (no bare `python3 -c '...'`).

## LLM Agent Sandboxing on macOS (fish shell)

This guide is written for macOS and fish users. It follows the Diataxis model:

- Explanation: why these controls matter and what macOS can/cannot do
- Reference: capability matrix and copy/paste config snippets
- How-to: task-oriented procedures
- Tutorials: end-to-end walkthroughs

---

## Explanation

### Threat model for coding agents

When an LLM can call tools, the main risks are:

- Prompt injection: hostile text in docs/issues/tests tricks the agent into dangerous commands
- Data exfiltration: reading secrets (`~/.ssh`, cloud creds, `.env`) then sending them over the network
- Supply chain compromise: package installs run untrusted scripts
- Persistence: shell startup files or git hooks get modified to survive beyond one session

For your use case, enforce three boundaries at all times:

1. File boundary: only mount/expose a dedicated workspace
2. Network boundary: explicit URL/domain allowlist
3. Installer boundary: strict package install policy (`npm`, `uv/pip`, `brew`)

### macOS isolation reality in 2026

- `sandbox-exec` is deprecated and not a future-proof foundation
- macOS does not provide Linux-like namespaces/cgroups for arbitrary CLI processes
- The most reliable modern boundary is virtualization/containerization (VZ.framework via OrbStack/Lima/Tart)
- Per-process outbound controls are best done with a Network Extension firewall (Little Snitch or LuLu)

Practical consequence: for untrusted agent actions, run them in containers/VMs and keep host-level network controls as defense-in-depth.

Optional exception: if you need a lighter local control layer, you can use `scripts/macos-sandbox.fish` (`sandboxctl local ...`) on systems that still provide `sandbox-exec`. Treat it as defense-in-depth only, not as a substitute for container/VM isolation.

### Package installer threat model

- `npm`: lifecycle scripts (`preinstall`, `install`, `postinstall`) can execute arbitrary shell commands
- `uv/pip`: source builds or malicious build backends can execute arbitrary code at install time
- `brew`: formula Ruby can execute arbitrary commands during source build; bottles are prebuilt binaries you still trust by provenance

Default stance: no installer network except approved registries, no installer scripts unless necessary, immutable lockfiles/hashes.

---

## Reference

### Capability matrix (macOS)

Columns are ordered from "what the agent can see/touch on disk" through to
"what enforces the policy". Two columns are deliberately distinct:

- **URL restrictions** — HTTP/HTTPS-layer allowlist applied to the agent's
  fetch tools (webfetch/websearch/etc.).
- **Network restrictions** — broader socket-/DNS-/firewall-level egress
  control. URL allowlists do not stop a `bash -c "curl ..."` or a raw
  socket; only network-layer controls (sandbox-exec `(deny network*)`,
  Docker network namespace + proxy, host firewall like LuLu/Little Snitch)
  do.
- **Process visibility limits** — whether the framework prevents the
  agent from enumerating, inspecting, or signaling other processes on the
  host (`ps`, `/proc/*/cmdline`, `lsof`, `kill`, etc.). Agents that can
  read other processes can scrape secrets out of `argv`, environment
  variables, or open file handles.

| Framework | File restrictions | SSH key restrictions | URL restrictions | Network restrictions | Process visibility limits | Installer restrictions | Enforcement level |
|---|---|---|---|---|---|---|---|
| Claude Code | Yes (`/sandbox` filesystem policy) | Yes (`denyRead` on `~/.ssh`) | Yes (managed domain filtering/proxy) | Yes when `/sandbox` profile uses `(deny network*)`; otherwise relies on app-layer allowlist | Configurable via sandbox-exec (`(deny process-info*)`, `(deny mach-lookup)`); not enforced by default profile | Indirect via command policy + environment | OS-level sandbox + app policy |
| OpenCode | Yes (permission rules, app-level) | Yes (pattern deny, app-level) | Partial (`webfetch/websearch`; bash needs external controls) | Not built-in (bash escape route); rely on Docker netns + proxy | Not built-in; rely on Docker PID namespace | Via command allow/deny + external sandbox | App policy (plus Docker if added) |
| CrewAI | Not built-in | Not built-in | Not built-in | Not built-in | Not built-in | Not built-in | External controls required |
| PydanticAI | Strong in Code Mode (Monty); otherwise not built-in | Strong in Code Mode; otherwise not built-in | Strong in Code Mode; otherwise not built-in | Strong in Code Mode (Monty isolates network); otherwise not built-in | Strong in Code Mode (Monty has no host process access); otherwise not built-in | Policy in your tool wrappers | Rust sandbox (Monty) + your controls |
| AG2 | Yes with Docker executor (`work_dir` mount) | Yes if keys never mounted | Via Docker networking/proxy | Via Docker network policy + proxy ACL | Yes via Docker PID namespace (default); broken if `--pid=host` is set | Via container policy/wrappers | Container boundary |

For frameworks marked "Not built-in" or "Configurable", the practical
defense remains the container/VM boundary plus a host firewall:

- **Network**: route the agent through a proxy (`examples/squid.conf`)
  inside a Docker network with no direct internet route, then keep a
  host-level deny-by-default firewall (LuLu / Little Snitch / `pf`).
- **Process visibility**: prefer Docker / Tart so the agent runs in its
  own PID namespace. Avoid `--pid=host`, `docker run --privileged`, or
  mounting `/proc`. On macOS `sandbox-exec`, add `(deny process-info*)`
  and `(deny mach-lookup)` to the profile.

### Default best-practice recommendations per framework

These are the defaults this repo recommends. They turn each row of the
matrix into concrete configuration. Treat them as the floor: weaken
only with a written reason and a compensating control.

**All frameworks (cross-cutting):**

- Run the agent inside a container (OrbStack / Lima / Docker Desktop) or
  a disposable VM (Tart for macOS). No host home mount; mount only the
  project directory at a fixed path (e.g. `/workspace`).
- Force outbound traffic through a proxy with a deny-by-default
  allowlist. Start from `examples/allowlist.domains` and
  `examples/squid.conf`. Add a host-level firewall (LuLu / Little Snitch
  / `pf`) as defense-in-depth.
- Never mount `~/.ssh`, `~/.aws`, `~/.config/gcloud`, or other
  credential directories. Use ephemeral, scope-limited credentials
  generated by the helpers under `scripts/llm-*.fish`.
- Pin all installed tools to exact versions (`examples/agent-tools.env`,
  lockfiles checked in, `npm ci` / `uv sync --frozen`). CI gates this
  via `scripts/check-pinning.fish`.
- Default network policy: `strict-egress`. Document any exceptions.
- Do not pass `--pid=host`, `--network=host`, `--privileged`, or mount
  `/var/run/docker.sock` into the agent container.

**Claude Code:** start from `examples/claude-code.settings.json`.
Enable `/sandbox`, deny-list `~/.ssh`, `~/.aws`, `~/.config/gcloud`,
and shell rc files. Sandbox profile must include `(deny network*)`,
`(deny process-info*)`, and `(deny mach-lookup)`; allow only the
specific subpaths the session needs to write.

**OpenCode:** load `examples/opencode.restrictive.json` via
`OPENCODE_CONFIG`, then run OpenCode inside the `agent` container from
`examples/docker-compose.yml`. App-level URL allowlist is not enough on
its own — bash escapes it; the container network namespace + proxy is
what enforces network policy.

**CrewAI:** treat the framework as having no built-in controls. Run
the crew runtime inside a container, expose only the project mount,
keep credentials in short-lived env vars or secret mounts, and route
all egress through the proxy. For tools that execute generated code,
prefer external sandbox services (E2B / Modal) over local execution.

**PydanticAI:** for any LLM-written code, use Code Mode with Monty so
execution happens in the Rust sandbox (no host process access, no host
filesystem, no host network). For ordinary tools, enforce path/network
allowlists in each tool wrapper, set `requires_approval=True` on tools
that mutate state or can exfiltrate, and cap runs with `UsageLimits`
(`request_limit`, `tool_calls_limit`, token caps).

**AG2:** use `DockerCommandLineCodeExecutor`, never the local executor.
Mount only a per-session `work_dir`; keep the container's root
filesystem read-only where possible; set `network_mode` to use the
proxy network, not `host`. Destroy the execution container after each
session.

**macOS host (defense-in-depth):** for risky one-off package installs,
prefer `scripts/brew-vm.fish` over the host. `scripts/macos-sandbox.fish`
(`sandboxctl local ...`) is acceptable as a lighter local layer for
trusted tasks but not as a substitute for container/VM isolation.

### OpenCode deep dive (requested focus)

OpenCode's permission model is useful but not sufficient by itself for untrusted execution. Use it with a container boundary.

Recommended layering:

1. Run OpenCode in Docker (no host home mount)
2. Mount only project directory
3. Route all egress through proxy allowlist
4. Use restrictive `opencode.json`

Reference config: `examples/opencode.restrictive.json`

### Claude Code sandbox reference

Use `/sandbox` in Claude Code and keep filesystem/network policy strict. Example file: `examples/claude-code.settings.json`.

Key points:

- Explicitly deny `~/.ssh`, cloud credentials, and shell rc files
- Restrict write access to project/work directories
- Keep domain allowlist narrow (registries + source control only)

### CrewAI reference

CrewAI has no native process sandbox for arbitrary tools. Use:

- Docker wrapper for crew runtime
- Proxy-enforced egress allowlist
- Tool wrappers for sensitive actions
- Optional external sandbox services for code execution (E2B/Modal)

Minimal hardening checklist:

- run CrewAI process in container/VM
- expose only a project mount (never host home)
- keep credentials in short-lived env vars or secret mounts
- route outbound network through allowlist proxy

### PydanticAI reference

- For LLM-written code: prefer Code Mode + Monty
- For normal tools: each tool is your code, so enforce path/network rules in tool wrappers
- Add `requires_approval=True` to high-risk tools

Minimal hardening checklist:

- use Monty for generated code execution
- guard filesystem/network tools with explicit allowlists
- enforce `UsageLimits` and approval for sensitive tools

### AG2 reference

Use `DockerCommandLineCodeExecutor` for untrusted code. Keep:

- `work_dir` as only mounted path
- read-only root fs where possible
- no host credential mounts
- proxy-enforced egress rules

Minimal hardening checklist:

- prefer `DockerCommandLineCodeExecutor` over local executor
- isolate `work_dir` per session
- avoid mounting host sockets (`/var/run/docker.sock`) unless unavoidable

### Homebrew reference (sandboxing suspicious installs)

Facts that matter:

- Homebrew still uses sandboxing for source builds on macOS, but bottles are prebuilt and skip build sandboxing
- Separate Homebrew prefixes can coexist but are not a security boundary
- Strongest option for suspicious brew installs is a disposable macOS VM (Tart)

Use `scripts/brew-vm.fish` for VM-backed isolation. `scripts/brew-sandbox.fish` is only prefix separation and should not be treated as sandboxing.

### Package manager hardening reference

`npm`

- Prefer `npm ci`
- Default to `--ignore-scripts`
- Use lockfile only, no ad hoc install in agent runs
- Pin CLI packages to exact versions (no `latest` in production)

`uv/pip`

- Prefer wheels only: `--only-binary :all:`
- Pin exact versions/hashes where possible
- Use `uv sync --frozen` for project sync
- Keep pinned framework versions in `examples/agent-tools.env`

`brew`

- Audit formula first (`brew cat`, `brew info`, `brew install --dry-run`)
- Prefer official taps only
- For unknown packages, install in disposable VM first

Recommended registry/domain allowlist baseline:

- npm: `registry.npmjs.org`
- Python: `pypi.org`, `files.pythonhosted.org`
- Git source: `github.com`, `raw.githubusercontent.com`

### Artifact pinning and attestation reference

Use pinned versions by default:

- `examples/agent-tools.env`
- `examples/agent-tools.env.example`

Useful verification commands:

```fish
npm view @anthropic-ai/claude-code@2.1.121 dist.integrity
npm view opencode-ai@1.14.28 dist.integrity
"/opt/homebrew/bin/python3" -m pip index versions crewai
"/opt/homebrew/bin/python3" -m pip index versions pydantic-ai
"/opt/homebrew/bin/python3" -m pip index versions ag2
./scripts/check-pinning.fish
```

For project dependencies:

- npm: commit `package-lock.json` and use `npm ci`
- uv: commit `uv.lock` and use `uv sync --frozen`
- Homebrew: prefer explicit formula names, official taps, and dry-run/audit before install

---

## How-to

### How to run any agent behind Docker + URL allowlist proxy

1. Create stack files from this repo:
   - `examples/Dockerfile.agent`
   - `examples/Dockerfile.agent.tools`
   - `examples/docker-compose.yml`
   - `examples/squid.conf`
   - `examples/agent-tools.env.example`
2. Start the proxy:

```fish
docker compose -f examples/docker-compose.yml build agent
docker compose -f examples/docker-compose.yml up -d proxy
```

3. Run agent container through proxy:

```fish
docker compose -f examples/docker-compose.yml run --rm agent
```

4. Verify blocking:

```fish
docker compose -f examples/docker-compose.yml run --rm agent sh -lc 'curl -I https://example.com'
```

Expected: denied unless domain is allowlisted.

### How to run with preinstalled CLIs/frameworks

1. Copy env template and pin versions:

```fish
cp examples/agent-tools.env.example examples/agent-tools.env
```

2. Edit `examples/agent-tools.env` and enable only the stacks you need
3. Keep versions pinned; avoid `latest` in automation
4. Build and run the tools image:

```fish
docker compose --env-file examples/agent-tools.env -f examples/docker-compose.yml build agent-tools
docker compose --env-file examples/agent-tools.env -f examples/docker-compose.yml run --rm agent-tools
```

5. Optional convenience wrapper:

```fish
source scripts/agent-sandbox-tools.fish
agent-sandbox-tools shell
```

or use hub command:

```fish
scripts/sandboxctl.fish docker-tools shell
```

### How to lock down OpenCode on macOS

1. Use restrictive config:

```fish
set -x OPENCODE_CONFIG (pwd)/examples/opencode.restrictive.json
```

2. Run OpenCode inside the `agent` container from `examples/docker-compose.yml`
3. Do not mount host home, only mount repo workspace
4. Keep proxy allowlist minimal and add domains only when required

### How to use optional local `sandbox-exec` layer on macOS

Use this only when full container/VM flows are not practical.

1. Load helper:

```fish
source scripts/macos-sandbox.fish
```

2. Run command with default `cwd` scope and strict egress deny:

```fish
macos-sandbox run -- /bin/pwd
```

3. Run command with repository-root scope (alternative to default `cwd` scope):

```fish
macos-sandbox run --repo-root-access -- /usr/bin/env ls
macos-sandbox run --path-scope repo-root -- /usr/bin/env ls
```

4. Use through the unified hub:

```fish
scripts/sandboxctl.fish local run --repo-root-access -- /bin/pwd
```

5. Add explicit additional paths only when needed:

```fish
macos-sandbox run --allow-read ~/.config --allow-write ./tmp -- /usr/bin/env ls
```

Notes:

- `--repo-root-access` is an alias for `--path-scope repo-root`
- `--network-policy strict-egress` (default) denies outbound network in profile
- Prefer Docker/VM workflows for untrusted execution

### How to lock down Claude Code

1. Enable `/sandbox`
2. Apply settings similar to `examples/claude-code.settings.json`
3. Add deny rules for sensitive paths (`~/.ssh`, `~/.aws`, `~/.config/gcloud`)
4. Keep network allowlist to registries + git hosts only

### How to run CrewAI with container boundaries

1. Run your CrewAI app inside the `agent` service (or a custom image)
2. Mount only workspace paths needed by tasks
3. Keep outbound traffic proxy-only (`HTTP_PROXY`/`HTTPS_PROXY` set)
4. For code execution tools, use external sandbox providers where possible

### How to run PydanticAI safely

1. Use Code Mode + Monty for generated code
2. Wrap filesystem/network tools in allowlist checks
3. Add approval gates to mutation/exfiltration-capable tools
4. Enforce run limits (`request_limit`, `tool_calls_limit`, token limits)

### How to run AG2 safely

1. Use Docker executor classes, not local execution
2. Mount only a session directory as `work_dir`
3. Keep egress constrained by proxy ACLs
4. Destroy execution container after each session

### How to add host-level process egress controls (LuLu/Little Snitch)

1. Install LuLu (free) or Little Snitch (paid)
2. Create deny-by-default outbound policy for agent binaries
3. Allow only explicit domains/ports needed for registries and git
4. Keep this as defense-in-depth even when using containers

### How to sandbox `npm` and `uv` installs

Use helper scripts:

- `scripts/safe-npm-install.fish`
- `scripts/safe-uv-install.fish`

These enforce strict defaults and are designed to run inside containerized agent sessions.

### How to sandbox `brew` with disposable Tart VMs

1. Load VM helper:

```fish
source scripts/brew-vm.fish
```

2. Create base template once:

```fish
brew-vm create-base
```

3. Install formula in disposable VM session:

```fish
set -x BREW_VM_PROXY_URL http://<proxy-host>:3128
brew-vm install --network-policy strict-egress <formula>
```

4. Optional: inspect manually in VM shell:

```fish
set -x BREW_VM_KEEP_SESSION true
brew-vm install <formula>
brew-vm shell
brew-vm destroy
```

5. Share files explicitly with host:

```fish
brew-vm copy-in ./local-file.txt /tmp/llm-share/local-file.txt
brew-vm copy-out /tmp/llm-share/result.txt ./result.txt
```

6. Verify policy enforcement:

```fish
brew-vm verify-network
```

Reference: `examples/tart-brew-sandbox.md`

### How to strengthen network limiting

1. Keep default script mode as `strict-egress`
2. Maintain outbound allowlist in `examples/allowlist.domains`
3. Use internal Docker network path (`agent`/`agent-tools` -> `proxy` only)
4. For VM sessions, set `BREW_VM_PROXY_URL` and run `brew-vm verify-network`
5. Keep host firewall egress rules (LuLu/Little Snitch/pf) as defense in depth

### How to manage ephemeral GitHub SSH deploy keys

1. Load helper:

```fish
source scripts/llm-github-keys.fish
```

2. Create RO + RW key pair for one repo (default TTL 24h):

```fish
llm-gh-key create-pair --repo <owner>/<repo> --name session-1 --ttl 24h
```

Optional: append SSH aliases to `~/.ssh/config` while creating:

```fish
llm-gh-key create-pair --repo <owner>/<repo> --name session-1 --ttl 24h --install-ssh-config --host-prefix github-llm
```

Then use:

- RO remote: `git@github-llm-ro:<owner>/<repo>.git`
- RW remote: `git@github-llm-rw:<owner>/<repo>.git`

3. List deploy keys on repo:

```fish
llm-gh-key list --repo <owner>/<repo>
```

4. Revoke one key by id:

```fish
llm-gh-key revoke --repo <owner>/<repo> --id <key-id>
```

5. Revoke keys by title pattern or expiration:

```fish
llm-gh-key revoke-by-title --repo <owner>/<repo> --match '^llm-agent:'
llm-gh-key revoke-expired --repo <owner>/<repo>
```

6. Generate or install SSH config aliases manually:

```fish
llm-gh-key print-ssh-config --ro-key ~/.ssh/llm_agent_github_ro_<stamp> --rw-key ~/.ssh/llm_agent_github_rw_<stamp>
llm-gh-key install-ssh-config --repo <owner>/<repo> --name session-1 --ro-key ~/.ssh/llm_agent_github_ro_<stamp> --rw-key ~/.ssh/llm_agent_github_rw_<stamp>
```

7. Remove old alias blocks from `~/.ssh/config`:

```fish
llm-gh-key uninstall-ssh-config --repo <owner>/<repo> --name session-1 --yes
llm-gh-key uninstall-ssh-config --marker '^llm-gh-key:<owner>-<repo>:' --yes
```

Notes:

- Prefer deploy keys (repo-scoped) over account-level SSH keys for agent identities
- Keep RO and RW keys separate and short-lived
- Enforce branch protections/rulesets for RW keys

### How to manage ephemeral Forgejo deploy keys (multi-instance)

1. Load helper:

```fish
source scripts/llm-forgejo-keys.fish
```

Optional bootstrap to copy starter config locally:

```fish
llm-forgejo-key bootstrap-config
```

2. Save Forgejo instance profile once:

```fish
llm-forgejo-key instance-set --name main --url https://forgejo.example.com --token-env FORGEJO_TOKEN_MAIN
set -x FORGEJO_TOKEN_MAIN <token-with-repo-admin>
```

Reference template: `examples/forgejo-instances.example.json`

3. Create RO + RW deploy key pair for one repository:

```fish
llm-forgejo-key create-pair --instance main --repo <owner>/<repo> --name session-1 --ttl 24h --install-ssh-config
```

4. List and revoke:

```fish
llm-forgejo-key list --instance main --repo <owner>/<repo>
llm-forgejo-key revoke --instance main --repo <owner>/<repo> --id <key-id>
llm-forgejo-key revoke-expired --instance main --repo <owner>/<repo> --yes
```

5. Remove old SSH alias blocks:

```fish
llm-forgejo-key uninstall-ssh-config --repo <owner>/<repo> --name session-1 --yes
```

### How to manage ephemeral Radicle identities across many repos

1. Load helper:

```fish
source scripts/llm-radicle-access.fish
```

Optional bootstrap to copy starter policy file locally:

```fish
llm-radicle-access bootstrap-config
```

2. Create short-lived identity:

```fish
llm-radicle-access create-identity --name session-1 --ttl 24h
```

3. Bind identity to current/future repositories by RID:

```fish
llm-radicle-access bind-repo --rid <rad:...> --identity-id <identity-id> --access ro
llm-radicle-access bind-repo --rid <rad:...> --identity-id <identity-id> --access rw --note "maintainer tasks"
```

4. Inspect and retire:

```fish
llm-radicle-access list-identities
llm-radicle-access list-bindings --all
llm-radicle-access retire-expired --yes
llm-radicle-access unbind-repo --rid <rad:...> --yes
```

5. Print shell export for active identity key:

```fish
llm-radicle-access print-env --identity-id <identity-id>
```

Reference state format: `examples/radicle-access-policy.example.json`

---

## Tutorials

### Tutorial: first sandboxed OpenCode session

Goal: run OpenCode with file, SSH, and URL constraints in <10 minutes.

1. Start proxy and agent container:

```fish
docker compose -f examples/docker-compose.yml build agent
docker compose -f examples/docker-compose.yml up -d proxy
docker compose -f examples/docker-compose.yml run --rm agent
```

2. Inside container, set OpenCode config and start:

```fish
set -x OPENCODE_CONFIG /workspace/examples/opencode.restrictive.json
# Install your OpenCode binary/package in this image first,
# then run it with the restrictive config.
```

3. Validate file isolation:
   - Attempt read of `/root/.ssh/id_rsa` (should fail or not exist)
   - Attempt write outside `/workspace` (should fail)

4. Validate URL isolation:
   - `curl https://registry.npmjs.org` should succeed
   - `curl https://example.com` should fail by proxy ACL

5. Tear down:

```fish
docker compose -f examples/docker-compose.yml down
```

### Tutorial: evaluate a suspicious formula safely

1. Load VM helper and create base template:

```fish
source scripts/brew-vm.fish
brew-vm create-base
```

2. Review formula first:

```fish
set -x BREW_VM_PROXY_URL http://<proxy-host>:3128
brew-vm run --network-policy strict-egress brew cat <formula>
brew-vm run --network-policy strict-egress brew info <formula>
brew-vm run --network-policy strict-egress brew install --dry-run <formula>
```

3. Install in disposable VM:

```fish
brew-vm install --network-policy strict-egress <formula>
```

4. Verify teardown:

```fish
brew-vm destroy
```

Host remains unchanged after VM deletion.

---

## Example scripts and configs

- `scripts/agent-sandbox.fish`: convenience runner for Docker sandbox
- `scripts/agent-sandbox-tools.fish`: runner for tool-preinstalled sandbox image
- `scripts/macos-sandbox.fish`: optional local `sandbox-exec` wrapper (defense-in-depth)
- `scripts/sandboxctl.fish`: unified command hub for sandbox scripts and tutorials
- `scripts/brew-vm.fish`: disposable Tart VM wrapper for Homebrew installs
- `examples/tart-brew-sandbox.md`: VM template assumptions for `brew-vm`
- `scripts/brew-sandbox.fish`: legacy isolated-prefix helper (not a sandbox)
- `scripts/llm-github-keys.fish`: generate/revoke ephemeral GitHub deploy keys
- `scripts/llm-forgejo-keys.fish`: generate/revoke ephemeral Forgejo deploy keys (multi-instance)
- `scripts/llm-radicle-access.fish`: manage ephemeral Radicle identities and RID bindings
- `scripts/_py/llm_*.py`: pinned-Python helpers for the three `llm-*.fish` scripts (run via `uv run --script`, PEP-723 inline metadata)
- `scripts/install-local-skills.fish`: install repo-versioned skills into local runtime
- `scripts/install-fish-tools.fish`: install fish command shims (stow preferred, direct fallback)
- `stow/fish-tools`: stow package for tool command shims under `.local/{bin,lib}`
- `skills/agent-sandbox-ops/SKILL.md`: operating workflow for sandbox + network controls
- `skills/agent-key-lifecycle/SKILL.md`: operating workflow for key and identity lifecycle
- `examples/forgejo-instances.example.json`: sample multi-instance Forgejo profile file
- `examples/radicle-access-policy.example.json`: sample Radicle identity/binding state format
- `scripts/check-pinning.fish`: CI/local gate for pinned tool versions
- `scripts/safe-npm-install.fish`: strict npm install wrapper
- `scripts/safe-uv-install.fish`: strict uv/pip install wrapper
- `scripts/CONVENTIONS.md`: script UX/comment/safety standards for maintainers
- `scripts/script-template.fish`: starter template for new fish scripts
- `CONTRIBUTING.md`: contributor workflow and sync requirements
- `agents.md`: agent operating contract and mandatory read order
- `examples/docker-compose.yml`: reusable agent + proxy stack
- `examples/allowlist.domains`: central outbound domain allowlist used by proxy
- `examples/Dockerfile.agent`: custom agent image with fish + uv + Python + Node
- `examples/Dockerfile.agent.tools`: optional layer with preinstalled agent stacks
- `examples/agent-tools.env.example`: pinned package/version template
- `examples/squid.conf`: URL allowlist rules
- `examples/opencode.restrictive.json`: restrictive OpenCode permissions
- `examples/claude-code.settings.json`: restrictive Claude Code sandbox settings

---

## Recommended baseline (short version)

If you want one default setup that works well on macOS:

1. Run agent in container (OrbStack/Lima/Docker Desktop)
2. Mount only the project directory
3. Never mount host credential directories (`~/.ssh`, cloud configs)
4. Force egress through allowlist proxy
5. Keep allowlist in `examples/allowlist.domains` and verify denials regularly
6. For risky brew installs, use disposable Tart VM first

---

## Verification checklist

Use these checks after setup to prove controls are active.

1. Build and start components:

```fish
docker compose -f examples/docker-compose.yml build agent
docker compose -f examples/docker-compose.yml up -d proxy
```

Optional tools image verification:

```fish
cp examples/agent-tools.env.example examples/agent-tools.env
docker compose --env-file examples/agent-tools.env -f examples/docker-compose.yml build agent-tools
docker compose --env-file examples/agent-tools.env -f examples/docker-compose.yml run --rm agent-tools sh -lc 'python3 --version && node --version && uv --version'
docker compose --env-file examples/agent-tools.env -f examples/docker-compose.yml run --rm agent-tools sh -lc 'python3 -m pip show crewai pydantic-ai pydantic-ai-harness ag2'
```

2. Confirm proxy policy blocks non-allowlisted URLs:

```fish
docker compose -f examples/docker-compose.yml run --rm agent \
    sh -lc 'curl -I https://example.com || true'
```

3. Confirm allowlisted registries still work:

```fish
docker compose -f examples/docker-compose.yml run --rm agent \
    sh -lc 'curl -I https://registry.npmjs.org'
docker compose -f examples/docker-compose.yml run --rm agent \
    sh -lc 'curl -I https://pypi.org/simple/'
```

4. Confirm SSH keys are not present in container:

```fish
docker compose -f examples/docker-compose.yml run --rm agent \
    sh -lc 'ls -la /root/.ssh || true'
```

5. Confirm `npm` strict mode wrapper behavior:

```fish
source scripts/safe-npm-install.fish
safe-npm-install
```

6. Confirm `uv` strict mode wrapper behavior:

```fish
source scripts/safe-uv-install.fish
safe-uv sync
safe-uv pip-install requests==2.32.3
```

7. Confirm pinning gate passes:

```fish
./scripts/check-pinning.fish
```

8. Confirm `brew` VM workflow is disposable:

```fish
source scripts/brew-vm.fish
set -x BREW_VM_PROXY_URL http://<proxy-host>:3128
brew-vm create-base
brew-vm install --network-policy strict-egress wget
tart list | grep brew-sandbox-session
```

Expected: no `brew-sandbox-session` VM remains unless `BREW_VM_KEEP_SESSION=true`.

9. Confirm CI enforces pinning on PRs:

- Workflow file: `.github/workflows/pinning-check.yml`
- Trigger: pull requests and pushes to `main`

10. Confirm CI can build sandbox images:

- Workflow file: `.github/workflows/sandbox-images-check.yml`
- Builds both `agent` and `agent-tools` services

11. Confirm CI enforces script/docs/skills/tests synchronization:

- Workflow file: `.github/workflows/script-doc-sync-check.yml`
- Rule: when `scripts/*.fish` **or** `scripts/_py/*.py` changes, corresponding updates must include:
  - `README.md`
  - `skills/*/SKILL.md` or `skills/README.md`
  - `tests/*.fish`

12. Confirm CI runs the test suite:

- Workflow file: `.github/workflows/tests.yml`
- Runs `fish tests/run.fish` on Ubuntu for every PR and push to `main`
