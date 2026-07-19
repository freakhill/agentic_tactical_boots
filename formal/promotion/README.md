# Promotion protocol model

`Promotion.tla` is a development/CI-only finite safety model for the ad-hoc
session promotion transaction. It is intentionally separate from
`formal/session/SessionBoundary.tla`: the session theorem covers lifecycle,
process ownership, runtime egress agreement/recovery, and teardown, while this
model covers the value-free preview/apply promotion protocol for creating a new
profile from a read-only ad-hoc snapshot.

## Running

```sh
make bootstrap-tla-promotion
make check-tla-promotion-model
make check-tla-promotion
TLA_OFFLINE=1 make check-tla-promotion
```

The target reuses the existing SHA-256-pinned TLA+ Tools jar managed by
`formal/tla2tools.lock` and `scripts/check-tla-session.sh`. Java/TLA remain
local development and CI tools only; they do not enter the signed Go binary.

## Bounds

The reviewed finite domain contains one source session, one target policy, two
symbolic grant IDs (`A`, `B`) and every selected subset, promotion-relevant source
and policy evidence booleans, target-free versus present/builtin conflict,
valid/invalid candidate, source status `Created|Running|Stopped`, and known,
failed, or unknown commit outcomes. These bounds characterize the protocol shape;
they are not an unbounded proof for arbitrary sessions, grants, hashes, files, or
CUE programs.

## Safety laws

The positive model checks:

- empty selection yields no durable rule;
- applied durable rules equal the selected current grant IDs;
- replacement requires exact source, complete grant-revision/set, and policy hash
  evidence;
- replacement is create-only and cannot replace an existing/builtin name;
- validation precedes replacement;
- applied success requires a known commit;
- no trust or launch effect appears.

The expected-failure mutants remove the empty-selection guard, exact source
snapshot check, complete grant-revision check, policy hash check, create-only
guard, validation gate, source consistency before replacement, or treat unknown
commit as success. The checker requires each mutant to violate its named law at
its named action anchor.

## TLA/Go graph conformance

`internal/engine/promotion/tla_conformance_test.go` parses the TLC
`-dump dot,actionlabels` graph and compares normalized initial states, reachable
states, and labelled edges in both directions against an independently enumerated
Go graph over the same reviewed finite bounds. The Go graph is a conformance
harness for the pure promotion kernel; production still depends only on Go code.

## Limits

This model does not prove CUE correctness, filesystem crash durability,
cryptographic hash collision resistance, arbitrary finite bounds, policy rendering
semantics, or package reproducibility. It assumes drivers faithfully implement the
five data-only effects: read source, read policy, validate candidate, replace
exact candidate, and report.
