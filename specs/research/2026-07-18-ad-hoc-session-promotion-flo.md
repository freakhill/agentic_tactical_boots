# Ad-hoc session promotion — FLO decision note

Date: 2026-07-18

## Selected design

Use a snapshot-bound `profile promote preview` / `profile promote apply` transaction plus a dedicated pure promotion reducer and bounded TLA+/Go graph conformance.

The alternative one-shot `profile create --from-session` was rejected because it weakens review and source/target drift binding. An Emacs-only CUE composer prefill was rejected because it would duplicate authority decisions outside Go and create avoidable TOCTOU gaps.

## Locked safety properties

1. Empty selection creates no durable rule.
2. Each durable rule maps one-to-one to one selected current grant ID.
3. No promotion effect writes the session or runtime.
4. Replacement requires matching source and target snapshots.
5. Existing and builtin names are never replaced.
6. Complete candidate validation occurs before replacement.
7. Only a known commit is reported as applied success; an unknown commit is `commit-uncertain`.
8. Promotion emits no trust or launch effect.

## Transaction contract

Preview writes a versioned, bounded `0600` plan only after it renders and validates the full candidate. The plan is value-free and includes only binding hashes/identifiers, selected destinations, fidelity summary, and public state. Apply rereads authoritative session and policy state under a target-policy transaction lock; any stale evidence fails closed. The replacement primitive distinguishes known pre-commit failure from commit uncertainty and never reports the latter as success.

## User-interface verdict

The Emacs path fetches a fresh session/grant snapshot, asks for a new profile name, presents unchecked grants with the lifetime transition `session-only → profile-persistent / future sessions`, renders CLI preview data, requires a second confirmation, and applies the exact plan. It preserves the draft/review buffer on cancellation, stale/validation failure, or commit uncertainty; it routes known success to ordinary policy review/trust and never auto-trusts or launches.
