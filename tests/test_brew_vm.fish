#!/usr/bin/env fish

# Tests for scripts/brew-vm.fish — sourced module.
# tart/ssh/scp are not invoked in CI; we exercise help and validation paths only.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/brew-vm.fish"

function __invoke
    command fish -c "source '$SCRIPT'; brew-vm $argv" 2>&1
end

function test_help_subcommand
    set -l out (__invoke help)
    set -l rc $status
    assert_status "brew-vm help status" $rc 0
    assert_contains "brew-vm help mentions Usage" "$out" "Usage:"
    assert_contains "brew-vm help mentions copy-in" "$out" "copy-in"
    assert_contains "brew-vm help mentions copy-out" "$out" "copy-out"
end

function test_dash_dash_help
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "brew-vm --help status" $rc 0
    assert_contains "brew-vm --help mentions Usage" "$out" "Usage:"
end

function test_no_args_prints_usage
    set -l out (__invoke)
    set -l rc $status
    assert_status "brew-vm no-args status" $rc 0
    assert_contains "brew-vm no-args mentions Usage" "$out" "Usage:"
end

function test_unknown_command_fails
    set -l out (__invoke not-a-real-command)
    set -l rc $status
    assert_eq "brew-vm unknown cmd fails" $rc 1
    assert_contains "brew-vm unknown cmd message" "$out" "Unknown command"
end

run_tests_in_file (basename (status filename))
