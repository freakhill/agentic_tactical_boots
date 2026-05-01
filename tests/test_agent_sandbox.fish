#!/usr/bin/env fish

# Tests for scripts/agent-sandbox.fish — sourced module.
# Docker is not invoked in CI; we exercise help and validation paths only.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/agent-sandbox.fish"

function __invoke
    # Run agent-sandbox in a fresh fish so the function definition is fresh.
    command fish -c "source '$SCRIPT'; agent-sandbox $argv" 2>&1
end

function test_help_subcommand
    set -l out (__invoke help)
    set -l rc $status
    assert_status "agent-sandbox help status" $rc 0
    assert_contains "agent-sandbox help mentions Usage" "$out" "Usage:"
    assert_contains "agent-sandbox help mentions network-policy" "$out" "--network-policy"
end

function test_dash_dash_help
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "agent-sandbox --help status" $rc 0
    assert_contains "agent-sandbox --help mentions Usage" "$out" "Usage:"
end

function test_no_args_prints_usage
    set -l out (__invoke)
    set -l rc $status
    assert_status "agent-sandbox no-args status" $rc 0
    assert_contains "agent-sandbox no-args mentions Usage" "$out" "Usage:"
end

function test_unknown_command_fails
    # Run from repo root so compose file check passes and we exercise the unknown
    # branch rather than the pre-check failure.
    pushd "$REPO_ROOT" >/dev/null
    set -l out (__invoke not-a-real-command)
    set -l rc $status
    popd >/dev/null
    assert_eq "agent-sandbox unknown cmd fails" $rc 1
    assert_contains "agent-sandbox unknown cmd message" "$out" "Unknown command"
end

function test_invalid_network_policy_rejected
    pushd "$REPO_ROOT" >/dev/null
    set -l out (__invoke run --network-policy bogus)
    set -l rc $status
    popd >/dev/null
    assert_eq "agent-sandbox invalid policy fails" $rc 1
    assert_contains "agent-sandbox invalid policy message" "$out" "Invalid --network-policy"
end

function test_missing_compose_file_reported
    set -l tmp (mk_tmpdir)
    pushd $tmp >/dev/null
    set -l out (__invoke run)
    set -l rc $status
    popd >/dev/null
    assert_eq "agent-sandbox missing compose fails" $rc 1
    assert_contains "agent-sandbox missing compose message" "$out" "docker-compose.yml"
end

function test_help_advertises_tui_and_examples
    set -l out (__invoke help)
    assert_contains "agent-sandbox help mentions tui" "$out" "agent-sandbox tui"
    assert_contains "agent-sandbox help mentions Examples" "$out" "Examples"
end

function test_tui_without_gum_prints_install_hint
    set -l tmp (mk_tmpdir)
    mkdir -p "$tmp/bin"
    set -l body "
        set -x PATH '$tmp/bin'
        source '$SCRIPT'
        agent-sandbox tui
    "
    set -l out (command fish -N -c "$body" 2>&1)
    set -l rc $status
    assert_eq "agent-sandbox tui no-gum fails" $rc 1
    assert_contains "agent-sandbox tui no-gum mentions gum" "$out" "gum"
    assert_contains "agent-sandbox tui no-gum suggests brew install" "$out" "brew install gum"
end

run_tests_in_file (basename (status filename))
