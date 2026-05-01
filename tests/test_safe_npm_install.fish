#!/usr/bin/env fish

# Tests for scripts/safe-npm-install.fish — sourced module.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/safe-npm-install.fish"

function __invoke_in --argument-names dir
    set -e argv[1]
    pushd $dir >/dev/null
    set -l out (command fish -c "source '$SCRIPT'; safe-npm-install $argv" 2>&1)
    set -l rc $status
    popd >/dev/null
    echo $rc
    for line in $out
        echo $line
    end
end

function test_help_path
    set -l tmp (mktemp -d)
    set -l result (__invoke_in $tmp --help)
    set -l rc $result[1]
    set -l out $result[2..]
    rm -rf $tmp
    assert_status "safe-npm-install --help status" $rc 0
    assert_contains "safe-npm-install --help mentions Usage" "$out" "Usage:"
end

function test_missing_lockfile_fails
    set -l tmp (mktemp -d)
    set -l result (__invoke_in $tmp)
    set -l rc $result[1]
    set -l out $result[2..]
    rm -rf $tmp
    assert_eq "safe-npm-install no lockfile fails" $rc 1
    assert_contains "safe-npm-install no lockfile message" "$out" "package-lock.json is required"
end

function test_extra_args_rejected
    # Even with a lockfile present, extra args should be rejected before npm runs.
    set -l tmp (mktemp -d)
    touch "$tmp/package-lock.json"
    set -l result (__invoke_in $tmp some-package)
    set -l rc $result[1]
    set -l out $result[2..]
    rm -rf $tmp
    assert_eq "safe-npm-install extra args fail" $rc 1
    assert_contains "safe-npm-install extra args message" "$out" "does not accept package names"
end

run_tests_in_file (basename (status filename))
