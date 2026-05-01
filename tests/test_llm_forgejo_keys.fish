#!/usr/bin/env fish

# Tests for scripts/llm-forgejo-keys.fish — sourced module.
# Forgejo API is not called; we exercise help and arg-validation paths.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/llm-forgejo-keys.fish"

function __invoke
    command fish -c "source '$SCRIPT'; llm-forgejo-key $argv" 2>&1
end

function test_no_args_prints_usage_and_fails
    set -l out (__invoke)
    set -l rc $status
    assert_eq "llm-forgejo-key no-args fails" $rc 1
    assert_contains "llm-forgejo-key no-args mentions Usage" "$out" "Usage:"
end

function test_help_flag
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "llm-forgejo-key --help status" $rc 0
    assert_contains "llm-forgejo-key --help mentions Usage" "$out" "Usage:"
    assert_contains "llm-forgejo-key --help mentions instance-set" "$out" "instance-set"
    assert_contains "llm-forgejo-key --help mentions create-pair" "$out" "create-pair"
end

function test_unknown_argument_fails
    set -l out (__invoke list --bogus)
    set -l rc $status
    assert_eq "llm-forgejo-key unknown arg fails" $rc 1
    assert_contains "llm-forgejo-key unknown arg message" "$out" "Unknown argument"
end

function test_unknown_command_fails
    # Some commands run validation in the dispatch switch.
    set -l out (__invoke do-not-exist --repo a/b)
    set -l rc $status
    assert_eq "llm-forgejo-key unknown cmd fails" $rc 1
end

function test_invalid_repo_format_rejected
    # Use create with bogus repo to hit __llm_forgejo_validate_repo.
    # This requires require_tools to pass first; if curl/python3/ssh-keygen are
    # missing the rc will still be 1 with a clear message, which we accept.
    set -l out (__invoke create --repo bogus-no-slash --access ro)
    set -l rc $status
    assert_eq "llm-forgejo-key invalid repo fails" $rc 1
end

run_tests_in_file (basename (status filename))
