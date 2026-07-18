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

  printf 'tla-session: verified TLA+ Tools v%s (%s)\n' "$lock_version" "$lock_sha256"
}

case "${1:-}" in
  bootstrap) bootstrap ;;
  "") fail "usage: scripts/check-tla-session.sh bootstrap" ;;
  *) fail "unknown command: $1" ;;
esac
