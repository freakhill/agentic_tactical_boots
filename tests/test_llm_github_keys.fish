#!/usr/bin/env fish

# Tests for scripts/llm-github-keys.fish — sourced module.
# We do not call the real GitHub API; we exercise help and arg-validation paths.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/llm-github-keys.fish"

function __invoke
    command fish -c "source '$SCRIPT'; llm-gh-key $argv" 2>&1
end

function test_no_args_prints_usage_and_fails
    set -l out (__invoke)
    set -l rc $status
    assert_eq "llm-gh-key no-args fails" $rc 1
    assert_contains "llm-gh-key no-args mentions Usage" "$out" "Usage:"
end

function test_help_flag
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "llm-gh-key --help status" $rc 0
    assert_contains "llm-gh-key --help mentions Usage" "$out" "Usage:"
    assert_contains "llm-gh-key --help mentions create-pair" "$out" "create-pair"
end

function test_unknown_argument_fails
    set -l out (__invoke list --bogus)
    set -l rc $status
    assert_eq "llm-gh-key unknown arg fails" $rc 1
    assert_contains "llm-gh-key unknown arg message" "$out" "Unknown argument"
end

function test_unknown_command_fails
    set -l out (__invoke do-not-exist)
    set -l rc $status
    assert_eq "llm-gh-key unknown cmd fails" $rc 1
    assert_contains "llm-gh-key unknown cmd message" "$out" "Unknown command"
end

function test_create_requires_repo_and_access
    set -l out (__invoke create)
    set -l rc $status
    # Without gh installed the require_tools check may fire first; both are valid
    # validation paths. Either way, exit must be non-zero with a clear message.
    assert_eq "llm-gh-key create missing args fails" $rc 1
end

function test_print_ssh_config_validates
    set -l out (__invoke print-ssh-config)
    set -l rc $status
    assert_eq "llm-gh-key print-ssh-config no args fails" $rc 1
    assert_contains "llm-gh-key print-ssh-config error mentions ro-key" "$out" "--ro-key"
end

function test_print_ssh_config_renders_aliases
    set -l tmp (mk_tmpdir)
    set -l ro "$tmp/ro_key"
    set -l rw "$tmp/rw_key"
    touch $ro $rw
    set -l out (__invoke print-ssh-config --ro-key $ro --rw-key $rw)
    set -l rc $status
    assert_status "llm-gh-key print-ssh-config status" $rc 0
    assert_contains "llm-gh-key print-ssh-config ro alias" "$out" "github-llm-ro"
    assert_contains "llm-gh-key print-ssh-config rw alias" "$out" "github-llm-rw"
    assert_contains "llm-gh-key print-ssh-config has IdentitiesOnly" "$out" "IdentitiesOnly yes"
end

function test_uninstall_ssh_config_requires_marker_or_repo
    set -l out (__invoke uninstall-ssh-config)
    set -l rc $status
    assert_eq "llm-gh-key uninstall-ssh-config no args fails" $rc 1
    assert_contains "llm-gh-key uninstall-ssh-config message" "$out" "marker"
end

run_tests_in_file (basename (status filename))
