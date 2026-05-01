#!/usr/bin/env fish

# Tests for scripts/check-pinning.fish
# - help path
# - passes against real repo fixtures (current state must be pinned)
# - fails when an unpinned `latest` is introduced into a temp fixture

source (dirname (status filename))/helpers.fish

set -g CHECK "$SCRIPTS_DIR/check-pinning.fish"

function test_help_flag_works
    set -l out (run_fish $CHECK --help 2>&1)
    set -l rc $status
    assert_status "check-pinning --help status" $rc 0
    assert_contains "check-pinning --help output" "$out" "Usage:"
    assert_contains "check-pinning --help output mentions Checks" "$out" "Checks:"
end

function test_help_subcommand_works
    set -l out (run_fish $CHECK help 2>&1)
    set -l rc $status
    assert_status "check-pinning help status" $rc 0
    assert_contains "check-pinning help output" "$out" "Usage:"
end

function test_passes_against_repo_fixtures
    # examples/agent-tools.env is gitignored (it's a copy of .example for local
    # use). Make the happy path hermetic by staging all four required files in
    # a tmp dir, using the .example contents to seed the .env file.
    set -l tmp (mktemp -d)
    mkdir -p "$tmp/examples"
    cp "$REPO_ROOT/examples/agent-tools.env.example" "$tmp/examples/agent-tools.env.example"
    cp "$REPO_ROOT/examples/agent-tools.env.example" "$tmp/examples/agent-tools.env"
    cp "$REPO_ROOT/examples/Dockerfile.agent.tools"   "$tmp/examples/Dockerfile.agent.tools"
    cp "$REPO_ROOT/examples/docker-compose.yml"       "$tmp/examples/docker-compose.yml"

    pushd "$tmp" >/dev/null
    set -l out (run_fish $CHECK 2>&1)
    set -l rc $status
    popd >/dev/null
    rm -rf "$tmp"

    assert_status "check-pinning passes on staged fixtures" $rc 0
    assert_contains "check-pinning success message" "$out" "pinning check passed"
end

function test_detects_unpinned_latest_in_env
    set -l tmp (mktemp -d)
    mkdir -p "$tmp/examples"
    # Copy real reference files to keep the rest of the check satisfied.
    cp "$REPO_ROOT/examples/Dockerfile.agent.tools" "$tmp/examples/"
    cp "$REPO_ROOT/examples/docker-compose.yml" "$tmp/examples/"
    # Introduce an unpinned `latest` line in the env file fixture.
    echo "CLAUDE_CODE_VERSION=latest" > "$tmp/examples/agent-tools.env"
    echo "CLAUDE_CODE_VERSION=latest" > "$tmp/examples/agent-tools.env.example"

    pushd "$tmp" >/dev/null
    set -l out (run_fish $CHECK 2>&1)
    set -l rc $status
    popd >/dev/null
    rm -rf "$tmp"

    assert_eq "check-pinning fails on latest" $rc 1
    assert_contains "check-pinning reports failure reason" "$out" "unpinned"
end

run_tests_in_file (basename (status filename))
