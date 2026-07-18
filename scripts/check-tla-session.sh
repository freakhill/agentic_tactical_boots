#!/usr/bin/env bash
set -euo pipefail

export LC_ALL=C
export LANG=C
export TZ=UTC
umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCK_FILE="$ROOT/formal/tla2tools.lock"
CACHE_DIR="$ROOT/.build/tla"
ARTIFACT_DIR="$CACHE_DIR/session"

fail() {
  printf 'tla-session: %s\n' "$1" >&2
  exit 1
}

lock_version=""
lock_url=""
lock_sha256=""
verified_jar=""
read_lock() {
  [[ -f "$LOCK_FILE" ]] || fail "missing lock file: formal/tla2tools.lock"

  local line key value
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -n "$line" ]] || continue
    [[ "$line" == *=* ]] || fail "invalid lock file"
    key="${line%%=*}"
    value="${line#*=}"
    case "$key" in
      VERSION)
        [[ -z "$lock_version" ]] || fail "invalid lock file"
        lock_version="$value"
        ;;
      URL)
        [[ -z "$lock_url" ]] || fail "invalid lock file"
        lock_url="$value"
        ;;
      SHA256)
        [[ -z "$lock_sha256" ]] || fail "invalid lock file"
        lock_sha256="$value"
        ;;
      *) fail "invalid lock file" ;;
    esac
  done < "$LOCK_FILE"

  [[ "$lock_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "invalid lock version"
  [[ "$lock_url" == "https://github.com/tlaplus/tlaplus/releases/download/v${lock_version}/tla2tools.jar" ]] || fail "invalid lock URL"
  [[ "$lock_sha256" =~ ^[0-9a-f]{64}$ ]] || fail "invalid lock SHA256"
}

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
  else
    fail "no SHA-256 tool found (need sha256sum or shasum)"
  fi
}

verify_jar() {
  local path="$1" actual
  [[ -f "$path" ]] || fail "TLA+ Tools jar is not a regular file"
  actual="$(sha256_file "$path")"
  [[ "$actual" == "$lock_sha256" ]] || fail "TLA+ Tools SHA256 mismatch"
}

bootstrap() {
  read_lock
  mkdir -p "$CACHE_DIR" "$ARTIFACT_DIR"

  local jar tmp=""
  if [[ -n "${TLA2TOOLS_JAR:-}" ]]; then
    jar="$TLA2TOOLS_JAR"
    verify_jar "$jar"
  else
    jar="$CACHE_DIR/tla2tools-${lock_version}.jar"
    if [[ -e "$jar" ]]; then
      verify_jar "$jar"
    else
      [[ "${TLA_OFFLINE:-0}" != "1" ]] || fail "offline mode requires a verified cached jar or TLA2TOOLS_JAR"
      command -v curl >/dev/null 2>&1 || fail "curl is required to acquire TLA+ Tools"
      tmp="$(mktemp "$CACHE_DIR/.tla2tools.XXXXXX")"
      trap 'rm -f "${tmp:-}"' EXIT
      if ! curl --proto '=https' --tlsv1.2 --fail --silent --show-error --location --retry 3 --output "$tmp" "$lock_url"; then
        fail "failed to acquire TLA+ Tools"
      fi
      verify_jar "$tmp"
      chmod 0444 "$tmp"
      mv "$tmp" "$jar"
      tmp=""
      trap - EXIT
    fi
  fi

  # Keep the Java prerequisite separate from the signed runtime binary. The jar
  # has already been verified when this point-in-time development check runs.
  command -v java >/dev/null 2>&1 || fail "java is required for TLA+ checks"
  verify_jar "$jar"
  java -version >/dev/null 2>&1 || fail "java is not runnable"
  verified_jar="$jar"

  printf 'tla-session: verified TLA+ Tools v%s (%s)\n' "$lock_version" "$lock_sha256"
}

run_with_timeout() {
  local seconds="$1"
  shift
  "$@" &
  local command_pid=$!
  (
    sleep "$seconds"
    kill -TERM "$command_pid" 2>/dev/null || exit 0
    sleep 2
    kill -KILL "$command_pid" 2>/dev/null || true
  ) &
  local timer_pid=$!
  local rc
  if wait "$command_pid"; then rc=0; else rc=$?; fi
  kill "$timer_pid" 2>/dev/null || true
  wait "$timer_pid" 2>/dev/null || true
  return "$rc"
}

run_tlc() {
  local cfg="$1" output_dir="$2"
  rm -rf "$output_dir"
  mkdir -p "$output_dir/meta"
  verify_jar "$verified_jar"
  (
    cd "$ROOT/formal/session"
    run_with_timeout 120 java -XX:+UseParallelGC -cp "$verified_jar" tlc2.TLC \
      -workers 1 \
      -metadir "$output_dir/meta" \
      -dump dot,actionlabels "$output_dir/states.dot" \
      -config "$cfg" \
      SessionBoundary.tla
  ) >"$output_dir/tlc.log" 2>&1
}

check_model() {
  bootstrap >/dev/null
  local model_dir="$ARTIFACT_DIR/model"
  mkdir -p "$model_dir"

  local started elapsed states generated edges
  started="$(date +%s)"
  if ! run_tlc "SessionBoundary.cfg" "$model_dir/positive"; then
    tail -80 "$model_dir/positive/tlc.log" >&2 || true
    fail "positive model check failed"
  fi
  elapsed=$(( $(date +%s) - started ))
  read -r generated states < <(awk '
    /states generated, [0-9]+ distinct states found/ {
      generated = $1
      for (i = 1; i <= NF; i++) if ($i == "distinct") states = $(i - 1)
    }
    END { gsub(/,/, "", generated); gsub(/,/, "", states); print generated, states }
  ' "$model_dir/positive/tlc.log")
  edges="$(grep -c -- ' -> ' "$model_dir/positive/states.dot")"
  [[ "$states" =~ ^[0-9]+$ ]] || fail "could not read positive distinct-state count"
  [[ "$generated" =~ ^[0-9]+$ ]] || fail "could not read positive generated-state count"
  [[ "$edges" =~ ^[0-9]+$ ]] || fail "could not read positive edge count"
  (( states <= 100000 )) || fail "positive model exceeded 100000 distinct states"
  (( elapsed <= 120 )) || fail "positive model exceeded 120 seconds"
  printf 'tla-session: positive generated=%s distinct=%s edges=%s elapsed=%ss\n' "$generated" "$states" "$edges" "$elapsed"

  local mutant invariant action cfg out
  while IFS='|' read -r mutant invariant action; do
    cfg="mutants/${mutant}.cfg"
    out="$model_dir/mutant-${mutant}"
    if run_tlc "$cfg" "$out"; then
      fail "mutant ${mutant} unexpectedly passed"
    fi
    grep -F "Invariant ${invariant} is violated" "$out/tlc.log" >/dev/null || {
      tail -80 "$out/tlc.log" >&2 || true
      fail "mutant ${mutant} missed expected invariant ${invariant}"
    }
    grep -F "$action" "$out/tlc.log" >/dev/null || {
      tail -80 "$out/tlc.log" >&2 || true
      fail "mutant ${mutant} missed action anchor ${action}"
    }
    printf 'tla-session: mutant %s violated %s at %s\n' "$mutant" "$invariant" "$action"
  done <<'MUTANTS'
RuntimeBeforeDurable|RuntimeAuthoritySubsetOfDurableAuthority|MutantRuntimeBeforeDurable
SecondOwner|AtMostOneLiveOwner|MutantSecondOwner
StaleToken|ExactOwnerRequiredForHandoffReleaseSignal|MutantStaleToken
UnknownAsOld|CommitUnknownNeverAssumedOld|MutantUnknownAsOld
AckWithoutInspect|NormalStateHasGenerationAgreement|MutantAckWithoutInspect
StopWithoutProof|TeardownClaimRequiresProvenTeardown|MutantStopWithoutProof
MUTANTS
}

case "${1:-}" in
  bootstrap) bootstrap ;;
  model) check_model ;;
  "") fail "usage: scripts/check-tla-session.sh <bootstrap|model>" ;;
  *) fail "unknown command: $1" ;;
esac
