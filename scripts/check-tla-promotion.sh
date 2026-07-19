#!/usr/bin/env bash
set -euo pipefail

export LC_ALL=C
export LANG=C
export TZ=UTC
umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="$ROOT/.build/tla"
ARTIFACT_DIR="$CACHE_DIR/promotion"
SESSION_SCRIPT="$ROOT/scripts/check-tla-session.sh"
JAR="$CACHE_DIR/tla2tools-1.7.4.jar"

fail() { printf 'tla-promotion: %s\n' "$1" >&2; exit 1; }

bootstrap() {
  "$SESSION_SCRIPT" bootstrap >/dev/null
  [[ -f "$JAR" ]] || fail "verified TLA+ jar is missing"
  command -v java >/dev/null 2>&1 || fail "java is required for TLA+ checks"
  printf 'tla-promotion: verified TLA+ Tools\n'
}

run_with_timeout() {
  local seconds="$1"; shift
  "$@" & local pid=$!
  ( sleep "$seconds"; kill -TERM "$pid" 2>/dev/null || exit 0; sleep 2; kill -KILL "$pid" 2>/dev/null || true ) & local timer=$!
  local rc
  if wait "$pid"; then rc=0; else rc=$?; fi
  kill "$timer" 2>/dev/null || true
  wait "$timer" 2>/dev/null || true
  return "$rc"
}

run_tlc() {
  local cfg="$1" out="$2"
  rm -rf "$out"
  mkdir -p "$out/meta"
  (
    cd "$ROOT"
    run_with_timeout 120 java -XX:+UseParallelGC -cp "$JAR" tlc2.TLC \
      -workers 1 \
      -metadir "$out/meta" \
      -dump dot,actionlabels "$out/states.dot" \
      -config "$cfg" \
      formal/promotion/Promotion.tla
  ) >"$out/tlc.log" 2>&1
}

check_model() {
  bootstrap >/dev/null
  local model_dir="$ARTIFACT_DIR/model"
  mkdir -p "$model_dir"
  local positive="$model_dir/positive"
  if ! run_tlc "formal/promotion/Promotion.cfg" "$positive"; then
    tail -80 "$positive/tlc.log" >&2 || true
    fail "positive model check failed"
  fi
  local generated states edges
  read -r generated states < <(awk '
    /states generated, [0-9]+ distinct states found/ {
      generated = $1
      for (i = 1; i <= NF; i++) if ($i == "distinct") states = $(i - 1)
    }
    END { gsub(/,/, "", generated); gsub(/,/, "", states); print generated, states }
  ' "$positive/tlc.log")
  edges="$(grep -c -- ' -> ' "$positive/states.dot")"
  [[ "$states" =~ ^[0-9]+$ ]] || fail "could not read positive state count"
  (( states <= 10000 )) || fail "positive model exceeded 10000 states"
  printf 'tla-promotion: positive generated=%s distinct=%s edges=%s\n' "$generated" "$states" "$edges"

  local mutant invariant action cfg out
  while IFS='|' read -r mutant invariant action; do
    cfg="formal/promotion/mutants/${mutant}.cfg"
    out="$model_dir/mutant-${mutant}"
    if run_tlc "$cfg" "$out"; then
      fail "mutant ${mutant} unexpectedly passed"
    fi
    grep -F "Invariant ${invariant} is violated" "$out/tlc.log" >/dev/null || { tail -80 "$out/tlc.log" >&2 || true; fail "mutant ${mutant} missed expected invariant ${invariant}"; }
    grep -F "$action" "$out/tlc.log" >/dev/null || { tail -80 "$out/tlc.log" >&2 || true; fail "mutant ${mutant} missed action anchor ${action}"; }
    printf 'tla-promotion: mutant %s violated %s at %s\n' "$mutant" "$invariant" "$action"
  done <<'MUTANTS'
EmptySelection|EmptySelectionNoDurable|MutantEmptySelection
SourceSnapshot|ReplacementRequiresMatchingSnapshots|MutantSourceSnapshot
GrantRevision|ReplacementRequiresMatchingSnapshots|MutantGrantRevision
PolicyHash|ReplacementRequiresMatchingSnapshots|MutantPolicyHash
CreateOnly|CreateOnly|MutantCreateOnly
Validation|ValidationBeforeReplacement|MutantValidation
ReleaseSource|ReplacementRequiresMatchingSnapshots|MutantReleaseSource
UnknownSuccess|AppliedSuccessRequiresKnownCommit|MutantUnknownSuccess
MUTANTS
}

check_conformance() {
  check_model
  local graph="$ARTIFACT_DIR/model/positive/states.dot"
  [[ -f "$graph" ]] || fail "positive TLC graph is missing"
  ( cd "$ROOT" && TLA_PROMOTION_GRAPH="$graph" TLA_PROMOTION_REQUIRE_GRAPH=1 go test ./internal/engine/promotion -run 'TLA|Graph|Mutation' -count=1 ) || fail "TLA+/Go promotion conformance failed"
}

case "${1:-}" in
  bootstrap) bootstrap ;;
  model) check_model ;;
  check) check_conformance ;;
  "") fail "usage: scripts/check-tla-promotion.sh <bootstrap|model|check>" ;;
  *) fail "unknown command: $1" ;;
esac
