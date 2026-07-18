# GitHub App repository discovery — decision (FLO)

Date: 2026-07-18 · Status: accepted for spec 0119.

## Verdict

Add one explicit, account-scoped, read-only operation:

```text
safeslop creds repositories <host>/<owner> --output json
```

V1 supports a linked public-GitHub App installation (`github.com/<owner>`) only. It resolves the existing host-only private-key ref, verifies the installation, mints one installation-wide token attenuated to exactly `metadata:read`, completely enumerates a bounded `/installation/repositories` snapshot, and synchronously attempts revocation before emitting any candidates.

Emacs invokes it only inside `C-c s K` → `R`, after the operator chooses GitHub + explicit repositories + one visible linked account + Fetch. The searchable list is selector assistance. It never authorizes a profile, saves CUE, trusts policy, launches, or replaces launch-time repo/permission intersection.

## Deterministic laws

1. **Exact authority:** the discovery mint sends exactly `permissions: {metadata: read}`, omits `repositories` and `repository_ids`, and requests no other permission. No Contents, Administration, mutation, user token, PAT, ambient `gh`, or session-token fallback.
2. **Host-memory-only:** resolved key bytes, App JWT, installation token, request/response bodies, and raw provider errors never enter argv, env, files, account/policy state, stage dirs, sandbox, public JSON, Emacs, or logs.
3. **Explicit and separate:** `creds status` remains non-minting. Discovery never runs from refresh, polling, agent traffic, save, trust, or launch.
4. **Cleanup survives cancellation:** after mint, every return path attempts exactly one revoke with a fresh bounded context independent of the request context. Command-local signal handling turns a first `SIGINT`/`SIGTERM` into request cancellation so cleanup can run. Cleanup uncertainty is never success.
5. **Snapshot, not authorization:** presence/absence and the capability hint are current UI metadata only. Whole-profile replacement confirmation, exact-byte trust, and launch-time downscoped mint/intersection remain authoritative.
6. **Minimum value-free contract:** output contains only the selected account key, a bounded Contents-capability class, validated repository names, and fixed outcome classes. No refs, app/installation ids, URLs, cursors, visibility, roles, raw GitHub objects, values, paths, or secret-derived detail.
7. **Hermetic/no dependency:** production reuses the Go `ForgeHTTP`, secret resolver, account store, and Emacs async seams; tests use fakes/local HTTP; no new runtime dependency.

## CLI and shared JSON contract

The positional account is exact and mandatory. There is no current/default account, wildcard, repetition, `--all`, or hidden multi-link aggregation. Malformed, missing, non-GitHub, or non-public-GitHub links fail before secret resolution/minting. The installation probe must match the selected owner case-insensitively.

Success uses the existing shared envelope—not a new top-level protocol:

```json
{
  "schema_version": 1,
  "ok": true,
  "data": {
    "account": "github.com/acme",
    "contents_maximum": "write",
    "repositories": ["acme/api", "acme/web"]
  },
  "warnings": [],
  "errors": []
}
```

`contents_maximum` is `none`, `read`, or `write`, derived from the installation probe's top-level Contents permission. It is a coarse current maximum, not a per-repo grant or readiness promise.

Every failure uses the existing shared error envelope and withholds `data.repositories`. Stable mappings use the current append-only registry: malformed account/argv → `INVALID_ARGUMENT`; missing/wrong link → `NOT_FOUND`; unresolved link credential → `AUTH_REQUIRED`; provider/transport/cancellation → `IO_ERROR` (or `TIMEOUT` for the operation deadline); malformed/inconsistent/over-limit provider data → `SCHEMA_VIOLATION`; unconfirmed post-mint revoke → `CREDENTIAL_REVOKE_FAILED`. Messages/details are fixed and value-free. Success exits 0; every error exits nonzero.

A successful list with a failed revoke is therefore **not** returned with a warning. Candidates remain buffered until cleanup succeeds; cleanup uncertainty emits `CREDENTIAL_REVOKE_FAILED`, with the fixed explanation that no repositories were imported and a metadata-only token may remain usable for at most one hour.

## Token lifecycle

- Resolve the selected link's private-key ref just in time on the host.
- Use a bounded request context (60 seconds) for installation probe, mint, and listing.
- Mint with exactly metadata read and no repo selector. Reject a provider expiry beyond GitHub's one-hour contract.
- Arm cleanup immediately after mint and buffer all prospective output.
- On every post-mint path, use a fresh ten-second cleanup context rooted independently of the cancelled request and issue exactly one revoke.
- If cleanup succeeds, return the primary success/error. If cleanup fails/times out/is ambiguous, override the primary result with `CREDENTIAL_REVOKE_FAILED` and withhold candidates.
- Install command-local `signal.NotifyContext` handling for the first interrupt/termination; a second forced signal, `SIGKILL`, process/host loss, or power loss can still prevent cleanup.

The hard residual bound is therefore private repository metadata for at most GitHub's one-hour token expiry after an unclean process death or ambiguous revoke. Expiry is the safety guarantee; revoke is the attempted early cleanup.

## Pagination and validation

Use only the selected public GitHub API origin and fixed requests:

```text
GET /installation/repositories?per_page=100&page=N
```

Never follow or emit provider-supplied next URLs/cursors. Bound the operation to 100 pages / 10,000 repositories, a transport-level response-body ceiling, a total retained-name ceiling, and the request deadline. Page 1 supplies `total_count`; zero still requires one valid empty page. Every page repeats the same count and has the expected size. Any short/overfull page, changed/final count, body/name overflow, malformed type, duplicate, or bound breach fails with no partial-success shape.

Retain only `full_name`. Every selector must match unchanged `[A-Za-z0-9._-]+/[A-Za-z0-9._-]+`, and the owner must match the probed installation account case-insensitively. Reject case-folded duplicates; sort case-insensitively with raw spelling as tie-breaker. Ignore unknown fields without logging them.

A complete result claims only an internally consistent traversal. Installation selection can change immediately afterward.

## Emacs flow

1. `R` keeps its existing project-profile, provider, mode, current-scope prefill, complete-replacement confirmation, failure-draft, and re-trust semantics.
2. For GitHub + explicit repos, show the linked App account(s); one can default, multiple require visible choice. Then offer **Fetch from linked App** (default) or **Enter manually**. Only Fetch invokes discovery asynchronously via argv, never a shell.
3. A valid success opens searchable `completing-read-multiple` prompts using installation repository names, with current read/write entries prefilled even when absent from the snapshot. Manual validated `owner/repo` input remains allowed because discovery is staleable and non-authoritative.
4. Show `contents_maximum` as a capability hint. If the operator requests write while the current hint is not `write`, require an explicit fixed warning; do not silently drop existing/manual entries.
5. Selection still reaches the existing before/after full replacement confirmation, then `profile credentials set`; policy review/re-trust remains required.
6. Cancellation, stale callback, dead/reused Credentials buffer, discovery error, or malformed envelope changes no profile/account/trust state and imports no candidates. Preserve the value-free draft so the operator can retry or choose manual entry.
7. Candidate lists exist only for the active flow and are not written to account state, CUE, customization, cache, or disk. Selected repos persist only through the existing confirmed policy mutation.

Direct CLI cancellation is signal-aware as above. Emacs does not need to kill the subprocess on ordinary buffer navigation; it ignores late results with a request token while allowing CLI cleanup to finish.

## Rejected alternatives

- **All-link aggregation:** hidden multiple mints, cleanup fan-out, and lost provenance. One explicit account per invocation is legible; multi-owner profiles retain manual entry.
- **Discovery inside status/refresh/save/trust/launch:** violates explicit mint consent and merges diagnostics with capability creation.
- **Candidates after cleanup uncertainty:** callers can ignore warnings; fail and withhold instead.
- **Partial success:** completion cannot safely make an incomplete set look ordinary. Fail, retain draft, and offer manual entry.
- **Names-only contract:** the bounded top-level Contents hint prevents misleading write UX without exposing rich repo metadata.
- **Rich GitHub objects, caching, ambient `gh`, user tokens, or Contents-scoped discovery:** unnecessary authority/metadata and stale-state risk.
- **Only local origins or opening GitHub settings:** useful manual fallbacks but does not meet the approved searchable installation list.

## Verification authority

Go TDD must pin exact mint JSON; no-selector behavior; one-account lookup; signal/request/cleanup contexts; success/error shared envelopes; no value/ref/body leakage; exact pagination for 0/1/100/101/10,000 rows; body/count/duplicate/grammar/owner adversaries; and exactly one revoke for every post-mint path. All provider tests use fakes or `httptest`.

ERT must pin exact argv, explicit-only invocation, account choice, async/stale/dead-buffer handling, strict success-contract parsing, searchable read/write inputs, capability warning, off-snapshot manual fallback, draft/current-scope preservation, and no profile mutation before existing confirmation. UI-matrix coverage must retain raw/Evil/Doom `R` dispatch.

## FLO record

Locked rubric: boundary custody/lifecycle 35%; authorization/enumeration truth 25%; operator UX/control 20%; public-contract/implementation fit 20%. One isolated worker produced the candidate; one cross-family Kimi evaluator scored 10.0/10.0/10.0/9.5 (nominal 99/100) with no fatal law violation. Host deterministic review lowered the unpatched artifact because it invented a top-level JSON protocol instead of the repository's shared envelope and assumed signal cancellation not present in the current command path. Both were evidence-forced fixes, not new hard choices: this note uses `jsoncontract.Envelope` and requires command-local signal handling. Evaluator-noted failure-example and zero-count ambiguities are likewise corrected here. No re-evaluation was needed.
