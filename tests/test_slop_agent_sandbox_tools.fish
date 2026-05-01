#!/usr/bin/env fish

# Tests for scripts/slop-agent-sandbox-tools.fish — same surface as slop-agent-sandbox.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/slop-agent-sandbox-tools.fish"

function __invoke
    command fish -c "source '$SCRIPT'; slop-agent-sandbox-tools $argv" 2>&1
end

function test_help_subcommand
    set -l out (__invoke help)
    set -l rc $status
    assert_status "slop-agent-sandbox-tools help status" $rc 0
    assert_contains "slop-agent-sandbox-tools help mentions Usage" "$out" "Usage:"
end

function test_no_args_prints_usage
    set -l out (__invoke)
    set -l rc $status
    assert_status "slop-agent-sandbox-tools no-args status" $rc 0
    assert_contains "slop-agent-sandbox-tools no-args mentions Usage" "$out" "Usage:"
end

function test_unknown_command_fails
    pushd "$REPO_ROOT" >/dev/null
    set -l out (__invoke not-a-real-command)
    set -l rc $status
    popd >/dev/null
    assert_eq "slop-agent-sandbox-tools unknown cmd fails" $rc 1
    assert_contains "slop-agent-sandbox-tools unknown cmd message" "$out" "Unknown command"
end

function test_invalid_network_policy_rejected
    pushd "$REPO_ROOT" >/dev/null
    set -l out (__invoke run --network-policy bogus)
    set -l rc $status
    popd >/dev/null
    assert_eq "slop-agent-sandbox-tools invalid policy fails" $rc 1
    assert_contains "slop-agent-sandbox-tools invalid policy message" "$out" "Invalid --network-policy"
end

function test_missing_compose_file_reported
    set -l tmp (mk_tmpdir)
    pushd $tmp >/dev/null
    set -l out (__invoke run)
    set -l rc $status
    popd >/dev/null
    assert_eq "slop-agent-sandbox-tools missing compose fails" $rc 1
    assert_contains "slop-agent-sandbox-tools missing compose message" "$out" "docker-compose.yml"
end

function test_help_advertises_tui_and_examples
    set -l out (__invoke help)
    assert_contains "slop-agent-sandbox-tools help mentions tui" "$out" "slop-agent-sandbox-tools tui"
    assert_contains "slop-agent-sandbox-tools help mentions Examples" "$out" "Examples"
end

function test_tui_without_gum_prints_install_hint
    set -l tmp (mk_tmpdir)
    mkdir -p "$tmp/bin"
    set -l body "
        set -x PATH '$tmp/bin'
        source '$SCRIPT'
        slop-agent-sandbox-tools tui
    "
    set -l out (command fish -N -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop-agent-sandbox-tools tui no-gum fails" $rc 1
    assert_contains "slop-agent-sandbox-tools tui no-gum mentions gum" "$out" "gum"
    assert_contains "slop-agent-sandbox-tools tui no-gum suggests brew install" "$out" "brew install gum"
end

run_tests_in_file (basename (status filename))
