#!/usr/bin/env fish

# Tests for scripts/agent-sandbox-tools.fish — same surface as agent-sandbox.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/agent-sandbox-tools.fish"

function __invoke
    command fish -c "source '$SCRIPT'; agent-sandbox-tools $argv" 2>&1
end

function test_help_subcommand
    set -l out (__invoke help)
    set -l rc $status
    assert_status "agent-sandbox-tools help status" $rc 0
    assert_contains "agent-sandbox-tools help mentions Usage" "$out" "Usage:"
end

function test_no_args_prints_usage
    set -l out (__invoke)
    set -l rc $status
    assert_status "agent-sandbox-tools no-args status" $rc 0
    assert_contains "agent-sandbox-tools no-args mentions Usage" "$out" "Usage:"
end

function test_unknown_command_fails
    pushd "$REPO_ROOT" >/dev/null
    set -l out (__invoke not-a-real-command)
    set -l rc $status
    popd >/dev/null
    assert_eq "agent-sandbox-tools unknown cmd fails" $rc 1
    assert_contains "agent-sandbox-tools unknown cmd message" "$out" "Unknown command"
end

function test_invalid_network_policy_rejected
    pushd "$REPO_ROOT" >/dev/null
    set -l out (__invoke run --network-policy bogus)
    set -l rc $status
    popd >/dev/null
    assert_eq "agent-sandbox-tools invalid policy fails" $rc 1
    assert_contains "agent-sandbox-tools invalid policy message" "$out" "Invalid --network-policy"
end

function test_missing_compose_file_reported
    set -l tmp (mktemp -d)
    pushd $tmp >/dev/null
    set -l out (__invoke run)
    set -l rc $status
    popd >/dev/null
    rm -rf $tmp
    assert_eq "agent-sandbox-tools missing compose fails" $rc 1
    assert_contains "agent-sandbox-tools missing compose message" "$out" "docker-compose.yml"
end

run_tests_in_file (basename (status filename))
