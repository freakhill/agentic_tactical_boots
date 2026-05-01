#!/usr/bin/env fish

# Tests for scripts/brew-sandbox.fish — sourced module.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/brew-sandbox.fish"

function __invoke
    command fish -c "source '$SCRIPT'; brew-sandbox $argv" 2>&1
end

function test_help_path
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "brew-sandbox --help status" $rc 0
    assert_contains "brew-sandbox --help mentions Usage" "$out" "Usage:"
    assert_contains "brew-sandbox --help warns about non-sandbox" "$out" "not a security sandbox"
end

function test_uninitialized_use_fails
    # When the prefix has not been initialised yet, brew-sandbox should warn and exit 1.
    set -l tmp (mktemp -d)
    set -l out (env HOME=$tmp command fish -c "source '$SCRIPT'; brew-sandbox info" 2>&1)
    set -l rc $status
    rm -rf $tmp
    assert_eq "brew-sandbox uninit fails" $rc 1
    assert_contains "brew-sandbox uninit suggests init" "$out" "brew-sandbox-init"
end

function test_warning_is_emitted
    set -l tmp (mktemp -d)
    set -l out (env HOME=$tmp command fish -c "source '$SCRIPT'; brew-sandbox info" 2>&1)
    rm -rf $tmp
    assert_contains "brew-sandbox emits warning" "$out" "Warning"
end

run_tests_in_file (basename (status filename))
