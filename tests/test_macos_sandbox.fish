#!/usr/bin/env fish

# Tests for scripts/macos-sandbox.fish — sourced module.
# Help paths run on every OS. Profile generation runs only on Darwin (with
# sandbox-exec); on Linux we assert the platform check correctly refuses.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/macos-sandbox.fish"

function __invoke
    command fish -c "source '$SCRIPT'; macos-sandbox $argv" 2>&1
end

function test_help_subcommand
    set -l out (__invoke help)
    set -l rc $status
    assert_status "macos-sandbox help status" $rc 0
    assert_contains "macos-sandbox help mentions Usage" "$out" "Usage:"
    assert_contains "macos-sandbox help mentions print-profile" "$out" "print-profile"
end

function test_dash_dash_help
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "macos-sandbox --help status" $rc 0
    assert_contains "macos-sandbox --help mentions Usage" "$out" "Usage:"
end

function test_no_args_prints_usage
    set -l out (__invoke)
    set -l rc $status
    assert_status "macos-sandbox no-args status" $rc 0
    assert_contains "macos-sandbox no-args mentions Usage" "$out" "Usage:"
end

function test_platform_aware_behavior
    set -l uname (uname)
    if test "$uname" = "Darwin"
        # On macOS, print-profile should produce a sandbox-exec profile.
        set -l out (__invoke print-profile)
        set -l rc $status
        assert_status "macos-sandbox print-profile (darwin)" $rc 0
        assert_contains "macos-sandbox profile has version" "$out" "(version 1)"
        assert_contains "macos-sandbox profile imports system.sb" "$out" "system.sb"
        assert_contains "macos-sandbox default policy denies network" "$out" "(deny network*)"
    else
        # On non-Darwin (e.g. Linux CI), running an action should refuse.
        set -l out (__invoke print-profile)
        set -l rc $status
        assert_eq "macos-sandbox refuses non-darwin" $rc 1
        assert_contains "macos-sandbox refuses with macOS-only message" "$out" "macOS only"
    end
end

function test_invalid_network_policy_rejected_on_darwin
    if test (uname) != "Darwin"
        # Skipping: handled by the platform refusal test above.
        __test_record_pass "macos-sandbox invalid policy (skipped on non-Darwin)"
        return 0
    end
    set -l out (__invoke run --network-policy bogus -- /bin/true)
    set -l rc $status
    assert_eq "macos-sandbox invalid policy fails" $rc 1
    assert_contains "macos-sandbox invalid policy message" "$out" "Invalid --network-policy"
end

run_tests_in_file (basename (status filename))
