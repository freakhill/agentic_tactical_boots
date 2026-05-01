#!/usr/bin/env fish

# Tests for scripts/sandboxctl.fish

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/sandboxctl.fish"

function test_no_args_prints_usage
    set -l out (run_fish $SCRIPT 2>&1)
    set -l rc $status
    assert_status "sandboxctl no-args status" $rc 0
    assert_contains "sandboxctl no-args mentions Usage" "$out" "Usage:"
end

function test_help_subcommand
    set -l out (run_fish $SCRIPT help 2>&1)
    set -l rc $status
    assert_status "sandboxctl help status" $rc 0
    assert_contains "sandboxctl help mentions Usage" "$out" "Usage:"
    assert_contains "sandboxctl help lists docker" "$out" "docker"
    assert_contains "sandboxctl help lists tutorial topics" "$out" "Topics:"
end

function test_help_flag
    set -l out (run_fish $SCRIPT --help 2>&1)
    set -l rc $status
    assert_status "sandboxctl --help status" $rc 0
    assert_contains "sandboxctl --help mentions Usage" "$out" "Usage:"
end

function test_list_subcommand
    set -l out (run_fish $SCRIPT list 2>&1)
    set -l rc $status
    assert_status "sandboxctl list status" $rc 0
    assert_contains "sandboxctl list shows docker mapping" "$out" "agent-sandbox.fish"
    assert_contains "sandboxctl list shows pinning mapping" "$out" "check-pinning.fish"
end

function test_tutorial_known_topic
    for topic in docker local brew-vm github-keys forgejo-keys radicle-access network-limiting file-sharing
        set -l out (run_fish $SCRIPT tutorial $topic 2>&1)
        set -l rc $status
        assert_status "sandboxctl tutorial $topic status" $rc 0
        # All topic outputs are non-empty.
        if test -z "$out"
            __test_record_fail "sandboxctl tutorial $topic non-empty" "no output"
        else
            __test_record_pass "sandboxctl tutorial $topic non-empty"
        end
    end
end

function test_tutorial_unknown_topic_fails
    set -l out (run_fish $SCRIPT tutorial nonsense-topic 2>&1)
    set -l rc $status
    assert_eq "sandboxctl tutorial unknown fails" $rc 1
    assert_contains "sandboxctl tutorial unknown message" "$out" "Unknown tutorial topic"
end

function test_tutorial_missing_topic_fails
    set -l out (run_fish $SCRIPT tutorial 2>&1)
    set -l rc $status
    assert_eq "sandboxctl tutorial missing topic fails" $rc 1
end

function test_unknown_command_fails
    set -l out (run_fish $SCRIPT not-a-real-command 2>&1)
    set -l rc $status
    assert_eq "sandboxctl unknown cmd fails" $rc 1
    assert_contains "sandboxctl unknown cmd message" "$out" "Unknown command"
end

# Note: end-to-end dispatch (e.g. `sandboxctl pinning` actually running
# check-pinning.fish from the repo root) is intentionally not tested here.
# The dispatch-target scripts use `set -l script_dir (cd ...; pwd)`, which
# silently changes the script's cwd in fish — a separate issue tracked outside
# this test suite. Tests above already cover sandboxctl's own argv handling.

run_tests_in_file (basename (status filename))
