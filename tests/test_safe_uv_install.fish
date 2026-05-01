#!/usr/bin/env fish

# Tests for scripts/safe-uv-install.fish — sourced module.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/safe-uv-install.fish"

function __invoke_in
    set -l dir $argv[1]
    set -e argv[1]
    pushd $dir >/dev/null
    set -l out (command fish -c "source '$SCRIPT'; safe-uv $argv" 2>&1)
    set -l rc $status
    popd >/dev/null
    echo $rc
    for line in $out
        echo $line
    end
end

function test_no_args_prints_usage
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_status "safe-uv no-args status" $rc 0
    assert_contains "safe-uv no-args mentions Usage" "$out" "Usage:"
end

function test_help_subcommand
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp --help)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_status "safe-uv --help status" $rc 0
    assert_contains "safe-uv --help mentions Usage" "$out" "Usage:"
end

function test_unknown_subcommand_fails
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp not-a-real-cmd)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_eq "safe-uv unknown subcmd fails" $rc 1
    assert_contains "safe-uv unknown subcmd message" "$out" "Unknown command"
end

function test_sync_without_lockfile_fails
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp sync)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_eq "safe-uv sync no lock fails" $rc 1
    assert_contains "safe-uv sync no lock message" "$out" "uv.lock is required"
end

function test_pip_install_unpinned_rejected
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp pip-install requests)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_eq "safe-uv pip-install unpinned fails" $rc 1
    assert_contains "safe-uv pip-install unpinned message" "$out" "name==version"
end

function test_pip_install_no_args_rejected
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp pip-install)
    set -l rc $result[1]
    set -l out $result[2..]
    assert_eq "safe-uv pip-install no-args fails" $rc 1
    assert_contains "safe-uv pip-install no-args message" "$out" "<name==version>"
end

function test_help_includes_enriched_sections
    set -l tmp (mk_tmpdir)
    set -l result (__invoke_in $tmp --help)
    set -l out $result[2..]
    assert_contains "safe-uv help has Description" "$out" "Description:"
    assert_contains "safe-uv help has Examples" "$out" "Examples:"
end

run_tests_in_file (basename (status filename))
