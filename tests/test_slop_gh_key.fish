#!/usr/bin/env fish

# Tests for scripts/slop-gh-key.fish — sourced module.
# We do not call the real GitHub API; we exercise help and arg-validation paths.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/slop-gh-key.fish"

function __invoke
    command fish -c "source '$SCRIPT'; slop-gh-key $argv" 2>&1
end

function test_no_args_prints_usage_and_fails
    set -l out (__invoke)
    set -l rc $status
    assert_eq "slop-gh-key no-args fails" $rc 1
    assert_contains "slop-gh-key no-args mentions Usage" "$out" "Usage:"
end

function test_help_flag
    set -l out (__invoke --help)
    set -l rc $status
    assert_status "slop-gh-key --help status" $rc 0
    assert_contains "slop-gh-key --help mentions Usage" "$out" "Usage:"
    assert_contains "slop-gh-key --help mentions create-pair" "$out" "create-pair"
end

function test_unknown_argument_fails
    set -l out (__invoke list --bogus)
    set -l rc $status
    assert_eq "slop-gh-key unknown arg fails" $rc 1
    assert_contains "slop-gh-key unknown arg message" "$out" "Unknown argument"
end

function test_unknown_command_fails
    set -l out (__invoke do-not-exist)
    set -l rc $status
    assert_eq "slop-gh-key unknown cmd fails" $rc 1
    assert_contains "slop-gh-key unknown cmd message" "$out" "Unknown command"
end

function test_create_requires_repo_and_access
    set -l out (__invoke create)
    set -l rc $status
    # Without gh installed the require_tools check may fire first; both are valid
    # validation paths. Either way, exit must be non-zero with a clear message.
    assert_eq "slop-gh-key create missing args fails" $rc 1
end

function test_print_ssh_config_validates
    set -l out (__invoke print-ssh-config)
    set -l rc $status
    assert_eq "slop-gh-key print-ssh-config no args fails" $rc 1
    assert_contains "slop-gh-key print-ssh-config error mentions ro-key" "$out" "--ro-key"
end

function test_print_ssh_config_renders_aliases
    set -l tmp (mk_tmpdir)
    set -l ro "$tmp/ro_key"
    set -l rw "$tmp/rw_key"
    touch $ro $rw
    set -l out (__invoke print-ssh-config --ro-key $ro --rw-key $rw)
    set -l rc $status
    assert_status "slop-gh-key print-ssh-config status" $rc 0
    assert_contains "slop-gh-key print-ssh-config ro alias" "$out" "github-llm-ro"
    assert_contains "slop-gh-key print-ssh-config rw alias" "$out" "github-llm-rw"
    assert_contains "slop-gh-key print-ssh-config has IdentitiesOnly" "$out" "IdentitiesOnly yes"
end

function test_uninstall_ssh_config_requires_marker_or_repo
    set -l out (__invoke uninstall-ssh-config)
    set -l rc $status
    assert_eq "slop-gh-key uninstall-ssh-config no args fails" $rc 1
    assert_contains "slop-gh-key uninstall-ssh-config message" "$out" "marker"
end

function test_help_advertises_here_and_tui
    set -l out (__invoke --help)
    assert_contains "slop-gh-key help mentions here" "$out" "here create-pair"
    assert_contains "slop-gh-key help mentions tui" "$out" "slop-gh-key tui"
end

function test_here_requires_subcommand
    set -l out (__invoke here)
    set -l rc $status
    assert_eq "slop-gh-key here no-sub fails" $rc 1
    assert_contains "slop-gh-key here no-sub message" "$out" "requires a subcommand"
end

function test_here_unknown_subcommand_fails
    # Run from a tmpdir that we initialize as a git repo with a github origin
    # so the inference step succeeds and the unknown-sub check is what fires.
    set -l tmp (mk_tmpdir)
    set -l body "
        cd '$tmp'
        command git init -q
        command git remote add origin git@github.com:owner/repo.git
        source '$SCRIPT'
        slop-gh-key here totally-not-a-thing
    "
    set -l out (command fish -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop-gh-key here unknown-sub fails" $rc 1
    assert_contains "slop-gh-key here unknown-sub message" "$out" "unknown 'here' subcommand"
end

function test_here_outside_git_repo_fails_clearly
    set -l tmp (mk_tmpdir)
    set -l body "
        cd '$tmp'
        source '$SCRIPT'
        slop-gh-key here list
    "
    set -l out (command fish -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop-gh-key here outside-repo fails" $rc 1
    assert_contains "slop-gh-key here outside-repo message" "$out" "could not infer GitHub repo"
end

function test_repo_inference_supports_url_forms
    # Each form should yield owner/repo via __llm_gh_repo_from_git.
    for url in \
        "git@github.com:owner/repo.git" \
        "git@github.com:owner/repo" \
        "https://github.com/owner/repo.git" \
        "https://github.com/owner/repo" \
        "ssh://git@github.com/owner/repo.git" \
        "git@github-llm-rw:owner/repo.git"
        set -l tmp (mk_tmpdir)
        set -l body "
            cd '$tmp'
            command git init -q
            command git remote add origin '$url'
            source '$SCRIPT'
            __llm_gh_repo_from_git
        "
        set -l out (command fish -c "$body" 2>&1)
        assert_eq "slop-gh-key infer from $url" "$out" "owner/repo"
    end
end

function test_repo_inference_rejects_non_github
    set -l tmp (mk_tmpdir)
    set -l body "
        cd '$tmp'
        command git init -q
        command git remote add origin git@gitlab.com:owner/repo.git
        source '$SCRIPT'
        __llm_gh_repo_from_git
        echo \"rc=\$status\"
    "
    set -l out (command fish -c "$body" 2>&1)
    assert_contains "slop-gh-key rejects gitlab origin" "$out" "rc=1"
end

function test_tui_without_gum_prints_install_hint
    # Force a PATH that excludes gum so the soft-dep check fires regardless of
    # whether the developer has gum installed.
    set -l tmp (mk_tmpdir)
    mkdir -p "$tmp/bin"
    # Provide minimal stand-ins for the few tools the function calls *before*
    # the gum check (none should be needed, but give it a clean stub PATH).
    set -l body "
        set -x PATH '$tmp/bin'
        source '$SCRIPT'
        slop-gh-key tui
    "
    set -l out (command fish -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop-gh-key tui no-gum fails" $rc 1
    assert_contains "slop-gh-key tui no-gum mentions gum" "$out" "gum"
    assert_contains "slop-gh-key tui no-gum suggests brew install" "$out" "brew install gum"
end

run_tests_in_file (basename (status filename))
