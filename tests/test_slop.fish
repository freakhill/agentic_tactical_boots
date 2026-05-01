#!/usr/bin/env fish

# Tests for scripts/slop.fish — global TUI launcher.
# We never actually start the interactive menu in CI (would require a TTY and
# the gum binary). All assertions exercise non-interactive paths: help/version
# print without gum, and the gum hard-dep gate fires with a clear message.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/slop.fish"

function test_help_subcommand_works_without_gum
    set -l out (run_fish $SCRIPT help 2>&1)
    set -l rc $status
    assert_status "slop help status" $rc 0
    assert_contains "slop help mentions Usage" "$out" "Usage:"
    assert_contains "slop help mentions Examples" "$out" "Examples:"
    assert_contains "slop help mentions Notes" "$out" "Notes:"
    assert_contains "slop help mentions per-tool TUI" "$out" "llm-gh-key tui"
end

function test_dash_dash_help
    set -l out (run_fish $SCRIPT --help 2>&1)
    set -l rc $status
    assert_status "slop --help status" $rc 0
    assert_contains "slop --help mentions Usage" "$out" "Usage:"
end

function test_version_flag
    set -l out (run_fish $SCRIPT --version 2>&1)
    set -l rc $status
    assert_status "slop --version status" $rc 0
    assert_contains "slop --version prints version" "$out" "slop"
end

function test_unknown_arg_fails_with_help
    set -l out (run_fish $SCRIPT bogus 2>&1)
    set -l rc $status
    assert_eq "slop unknown arg fails" $rc 1
    assert_contains "slop unknown arg mentions Usage" "$out" "Usage:"
    assert_contains "slop unknown arg shows error" "$out" "unknown argument"
end

function test_no_args_without_gum_prints_install_hint
    # Force a PATH that excludes gum so the hard-dep gate fires.
    # We use `fish -N` (no-config) and `source` so the user's fish config
    # cannot re-extend PATH and accidentally re-add /opt/homebrew/bin where
    # gum typically lives. We accept that sourced exit 1 surfaces as the
    # outer shell's exit code, which is exactly what we assert.
    set -l tmp (mk_tmpdir)
    mkdir -p "$tmp/bin"
    set -l body "set -x PATH '$tmp/bin'; source '$SCRIPT'"
    set -l out (command fish -N -c "$body" 2>&1)
    set -l rc $status
    assert_eq "slop no-gum fails" $rc 1
    assert_contains "slop no-gum mentions gum" "$out" "gum"
    assert_contains "slop no-gum suggests brew install" "$out" "brew install gum"
    assert_contains "slop no-gum suggests CLI fallback" "$out" "sandboxctl.fish help"
end

function test_install_fish_tools_wraps_slop
    # The conf.d snippet wraps standalone scripts as fish functions; verify
    # 'slop' is in the standalone list so the wrapper is generated.
    set -l installer "$REPO_ROOT/scripts/install-fish-tools.fish"
    set -l content (cat "$installer")
    assert_contains "install-fish-tools knows about slop" "$content" "slop"
end

run_tests_in_file (basename (status filename))
