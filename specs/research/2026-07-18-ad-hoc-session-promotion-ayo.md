# Ad-hoc session promotion — ayo decision note

Date: 2026-07-18

## Prior art applied

- **AWS IAM Access Analyzer**: activity is evidence for a reviewable candidate, not authority to install automatically. Promotion therefore starts from a read-only ad-hoc-session snapshot and defaults to no selected grants.
- **Terraform saved plans**: preview/apply must bind exact source and target evidence; a stale plan is rejected rather than recomputed at apply. The promotion plan binds source identity/snapshot, complete grant revision/set, target policy state, selected IDs, target name, candidate hash, and renderer/schema version.
- **firewalld runtime/permanent split**: runtime grants and durable policy are separate authorities. Promotion maps only explicitly selected grants to `persistentEgress`; it neither bulk-copies runtime state nor changes the running session.

## Applied boundary

Promotion creates a new, ordinary untrusted project profile through a bounded plan/apply policy transaction. It preserves the recorded canonical workspace and normalized agent/environment/network. When the session has complete resolved package identity metadata, it emits that exact identity with `BareAgent=true`; absence retains the synthetic ad-hoc behavior. Partial or contradictory metadata fails closed.

No observation, acknowledgement, connection/request history, process/container state, credential/secret/ref, runtime authority, trust state, or unselected grant becomes durable. The CUE contains no promotion/session/grant/revision/timestamp marker.

## Approved operational choices

- Preview may read a running source, but apply accepts only `created` or `stopped`. A lifecycle transition `running -> stopped` does not stale the plan when all promotion-relevant evidence remains equal. Emacs guides the operator to stop, but never stops and promotes as one action.
- All grant checkboxes begin unchecked. Version 1 provides no Select All action.

## Remaining limits

The protocol model is bounded and separate from `formal/session`: it does not claim CUE correctness, filesystem crash durability, cryptographic-hash properties, arbitrary bounds, or package reproducibility.
