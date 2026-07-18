# TLA+ session-boundary prior-art lessons

Date: 2026-07-18  
Project state: `main@ab08404`

## Frame

The target is a development/CI-only TLA+ safety model for safeslop's transactional session boundary: lifecycle ownership, old/new/uncertain record commits, egress widen/narrow generation acknowledgement, recovery, and fail-closed teardown. The model must remain aligned mechanically with production Go while preserving the single-binary runtime and all public behavior.

Method: two blind from-knowledge lanes plus a host lane; no web retrieval. GPT and DeepSeek lanes completed. Gemini was unavailable due provider quota. The host also inspected the current safeslop implementation, verified local TLC v1.7.4 behavior, and compared two local bounded-model precedents.

## High lessons

1. **Model stable semantic boundaries, not CLI control flow.** One CLI mutation spans durable commits and runtime effects; each interruptible commit/apply/inspect/teardown boundary must remain visible. Helpers and retries may refine to stuttering steps.
2. **Keep truth and evidence distinct.** Durable authority, effective runtime authority, recorded applied generation, inspected generation, and old/new knowledge after uncertainty are separate concepts. Hashes are symbolic `Generation(authority, revision)` values in the model.
3. **Treat unknown commits as old-or-new.** A commit-uncertain result cannot be interpreted as definite failure; recovery or proven teardown must precede another normal authority mutation.
4. **Model exact owner identity, not PID belief.** Two bounded owners share a PID and differ by process token. Liveness/token observations are nondeterministic environment evidence; the model does not claim a kernel proof.
5. **Use a pure production transition reducer.** Deterministic choices should move from `internal/cli/session.go` into the session engine; filesystem, runtime, process, and time effects remain behind existing dependency seams.
6. **Compare complete bounded graphs bidirectionally.** TLC v1.7.4 was locally verified to emit named edges with `-dump dot,actionlabels`. Compare normalized initial states, reachable states, and labelled edges against a Go BFS over production `Reduce`/`Enabled` in both directions.
7. **Make vacuity fail.** Positive invariants need expected-failure mutants, action/outcome coverage, strict parser failures, finite state/runtime budgets, and useful shortest-witness diagnostics.
8. **Pin tooling as supply-chain input.** Use official TLA+ Tools v1.7.4, SHA-256 `936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`, verify every invocation, cache under ignored `.build/tla`, and permit only a byte-identical override.

## Medium/deferred lessons

- A sampled production trace validator can become a diagnostic later, but is weaker than graph equality and would add instrumentation pressure now.
- History variables should be added only when current-state invariants cannot close; unnecessary history enlarges the state space.
- TLC counterexamples should be converted manually into focused Go regressions initially. Automatic Go-source generation is deferred.
- A two-session model is deferred until a cross-session invariant exists. One record plus two competing owners already explores per-record serialization.

## Rejected applications

- Model-only prose mapping: insufficient anti-drift evidence.
- Generated production Go/TLA or a new protocol DSL: excessive irreversible machinery and reduced independent checking.
- Production trace logging: unnecessary side effects and value-free/logging risk.
- Modeling Docker, Squid, POSIX durability, or process liveness internals: outside safeslop's controlled protocol boundary.
- Treating model-discovered missing guards as refactoring: any behavior defect requires a separate RED→GREEN decision and proof.
