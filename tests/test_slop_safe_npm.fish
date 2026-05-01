#!/usr/bin/env fish

# Tests for scripts/slop-safe-npm.fish — sourced module.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/slop-safe-npm.fish"

function __invoke_in --argument-names dir
    set -e argv[1]
    pushd $dir >/dev/null
    set -l out (command fish -c "source '$SCRIPT'; slop-safe-npm $argv" 2>&1)
    set -l rc $status
    popd >/dev/null
    echo $rc
    for line in $out
        echo $line
    end
end

function test_help_path
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp --help)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_status "slop-safe-npm --help status" $rc 0
    assert_contains "slop-safe-npm --help mentions Usage" "$out" "Usage:"
end

function test_missing_lockfile_fails
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_eq "slop-safe-npm no lockfile fails" $rc 1
    assert_contains "slop-safe-npm no lockfile message" "$out" "package-lock.json is required"
end

function test_extra_args_rejected
    # Even with a lockfile present, extra args should be rejected before npm runs.
    set -l tmp (mk_tmpdir)
    touch "$tmp/package-lock.json"
    set -l result (__invoke_in $tmp some-package)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_eq "slop-safe-npm extra args fail" $rc 1
    assert_contains "slop-safe-npm extra args message" "$out" "does not accept package names"
end

function test_help_includes_enriched_sections
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp --help)
    set -l out $result[2..]
    assert_contains "slop-safe-npm help has Description" "$out" "Description:"
    assert_contains "slop-safe-npm help has Examples" "$out" "Examples:"
end

run_tests_in_file (basename (status filename))
