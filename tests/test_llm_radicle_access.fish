#!/usr/bin/env fish

# Tests for scripts/llm-radicle-access.fish — sourced module.
# We do not generate real keys; we exercise help and arg-validation paths.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/llm-radicle-access.fish"

function __invoke
    command fish -c "source '$SCRIPT'; llm-radicle-access $argv" 2>&1
end

function test_no_args_prints_usage_and_fails
    set -l out (__invoke)
    set -l rc $status
    assert_eq "llm-radicle-access no-args fails" $rc 1
    assert_contains "llm-radicle-access no-args mentions Usage" "$out" "Usage:"
end

function test_help_flag
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "llm-radicle-access --help status" $rc 0
    assert_contains "llm-radicle-access --help mentions Usage" "$out" "Usage:"
    assert_contains "llm-radicle-access --help mentions create-identity" "$out" "create-identity"
    assert_contains "llm-radicle-access --help mentions bind-repo" "$out" "bind-repo"
end

function test_unknown_argument_fails
    set -l out (__invoke list-identities --bogus)
    set -l rc $status
    assert_eq "llm-radicle-access unknown arg fails" $rc 1
    assert_contains "llm-radicle-access unknown arg message" "$out" "Unknown argument"
end

function test_unknown_command_fails
    set -l out (__invoke do-not-exist)
    set -l rc $status
    assert_eq "llm-radicle-access unknown cmd fails" $rc 1
    assert_contains "llm-radicle-access unknown cmd message" "$out" "Unknown command"
end

function test_invalid_rid_rejected
    # bind-repo with an obviously bad RID should be rejected. require_tools may
    # fire first depending on env, so we only require non-zero exit.
    set -l out (__invoke bind-repo --rid not-a-rad --identity-id rid-x --access ro)
    set -l rc $status
    assert_eq "llm-radicle-access invalid rid fails" $rc 1
end

function test_invalid_access_rejected
    set -l out (__invoke bind-repo --rid rad:z3abcDEF --identity-id rid-x --access bogus)
    set -l rc $status
    assert_eq "llm-radicle-access invalid access fails" $rc 1
end

function test_print_env_requires_id
    set -l out (__invoke print-env)
    set -l rc $status
    assert_eq "llm-radicle-access print-env no id fails" $rc 1
    assert_contains "llm-radicle-access print-env message" "$out" "--identity-id"
end

run_tests_in_file (basename (status filename))
