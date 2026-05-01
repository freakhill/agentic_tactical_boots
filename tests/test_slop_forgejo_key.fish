#!/usr/bin/env fish

# Tests for scripts/slop-forgejo-key.fish — sourced module.
# Forgejo API is not called; we exercise help and arg-validation paths.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/slop-forgejo-key.fish"

function __invoke
    command fish -c "source '$SCRIPT'; slop-forgejo-key $argv" 2>&1
end

function test_no_args_prints_usage_and_fails
    set -l out (__invoke)
    set -l rc $status
    assert_eq "slop-forgejo-key no-args fails" $rc 1
    assert_contains "slop-forgejo-key no-args mentions Usage" "$out" "Usage:"
end

function test_help_flag
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "slop-forgejo-key --help status" $rc 0
    assert_contains "slop-forgejo-key --help mentions Usage" "$out" "Usage:"
    assert_contains "slop-forgejo-key --help mentions instance-set" "$out" "instance-set"
    assert_contains "slop-forgejo-key --help mentions create-pair" "$out" "create-pair"
end

function test_unknown_argument_fails
    set -l out (__invoke list --bogus)
    set -l rc $status
    assert_eq "slop-forgejo-key unknown arg fails" $rc 1
    assert_contains "slop-forgejo-key unknown arg message" "$out" "Unknown argument"
end

function test_unknown_command_fails
    # Some commands run validation in the dispatch switch.
    set -l out (__invoke do-not-exist --repo a/b)
    set -l rc $status
    assert_eq "slop-forgejo-key unknown cmd fails" $rc 1
end

function test_invalid_repo_format_rejected
    # Use create with bogus repo to hit __llm_forgejo_validate_repo.
    # This requires require_tools to pass first; if curl/python3/ssh-keygen are
    # missing the rc will still be 1 with a clear message, which we accept.
    set -l out (__invoke create --repo bogus-no-slash --access ro)
    set -l rc $status
    assert_eq "slop-forgejo-key invalid repo fails" $rc 1
end

function test_help_advertises_here_and_tui
    set -l out (__invoke --help)
    assert_contains "slop-forgejo-key help mentions here" "$out" "here create-pair"
    assert_contains "slop-forgejo-key help mentions tui" "$out" "slop-forgejo-key tui"
end

function test_here_requires_subcommand
    set -l out (__invoke here)
    set -l rc $status
    assert_eq "slop-forgejo-key here no-sub fails" $rc 1
    assert_contains "slop-forgejo-key here no-sub message" "$out" "requires a subcommand"
end

function test_here_outside_git_repo_fails
    set -l tmp (mk_tmpdir)
    set -l body "
        cd '$tmp'
        source '$SCRIPT'
        slop-forgejo-key here list
    "
    set -l out (command fish -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop-forgejo-key here outside-repo fails" $rc 1
    assert_contains "slop-forgejo-key here outside-repo message" "$out" "could not infer repo"
end

function test_here_no_matching_instance_profile_fails_clearly
    # Set up a tmp repo + a tmp instance profile pointing at a DIFFERENT host
    # so the lookup-by-host branch fires. We override LLM_FORGEJO_INSTANCES_FILE
    # to keep the test hermetic and avoid touching ~/.config.
    set -l tmp (mk_tmpdir)
    set -l profile "$tmp/instances.json"
    echo '{"instances":{"main":{"url":"https://other.example.com","token_env":"FORGEJO_TOKEN"}}}' > "$profile"
    set -l body "
        cd '$tmp'
        command git init -q
        command git remote add origin git@forgejo.example.com:owner/repo.git
        source '$SCRIPT'
        set -g LLM_FORGEJO_INSTANCES_FILE '$profile'
        slop-forgejo-key here list
    "
    set -l out (command fish -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop-forgejo-key here no-instance fails" $rc 1
    assert_contains "slop-forgejo-key here no-instance message" "$out" "no Forgejo instance profile"
    assert_contains "slop-forgejo-key here suggests instance-set" "$out" "instance-set"
end

function test_here_repo_inference_url_forms
    for url in \
        "git@forgejo.example.com:owner/repo.git" \
        "https://forgejo.example.com/owner/repo.git" \
        "ssh://git@forgejo.example.com/owner/repo.git"
        set -l tmp (mk_tmpdir)
        set -l body "
            cd '$tmp'
            command git init -q
            command git remote add origin '$url'
            source '$SCRIPT'
            __llm_forgejo_repo_from_git
        "
        set -l out (command fish -c "$body" 2>&1)
        # Output is two lines: host then owner/repo. Join for the assertion.
        assert_contains "slop-forgejo-key infer host from $url" "$out" "forgejo.example.com"
        assert_contains "slop-forgejo-key infer repo from $url" "$out" "owner/repo"
    end
end

function test_tui_without_gum_prints_install_hint
    set -l tmp (mk_tmpdir)
    mkdir -p "$tmp/bin"
    set -l body "
        set -x PATH '$tmp/bin'
        source '$SCRIPT'
        slop-forgejo-key tui
    "
    set -l out (command fish -N -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop-forgejo-key tui no-gum fails" $rc 1
    assert_contains "slop-forgejo-key tui no-gum mentions gum" "$out" "gum"
    assert_contains "slop-forgejo-key tui no-gum suggests brew install" "$out" "brew install gum"
end

run_tests_in_file (basename (status filename))
