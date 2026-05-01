#!/usr/bin/env fish

# Tests for scripts/install-fish-tools.fish
# - help paths
# - install/uninstall/status against an isolated tmp conf-dir
# - generated snippet parses as valid fish
# - sourced snippet exposes both module functions and standalone wrappers
# - cleanup honors --no-cleanup; legacy paths in $HOME are not touched in CI

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/install-fish-tools.fish"

function test_help_subcommand
    set -l out (run_fish $SCRIPT help 2>&1)
    set -l rc $status
    assert_status "install-fish-tools help status" $rc 0
    assert_contains "install-fish-tools help mentions Usage" "$out" "Usage:"
    assert_contains "install-fish-tools help mentions install" "$out" "install"
    assert_contains "install-fish-tools help mentions uninstall" "$out" "uninstall"
end

function test_dash_dash_help
    set -l out (run_fish $SCRIPT --help 2>&1)
    set -l rc $status
    assert_status "install-fish-tools --help status" $rc 0
    assert_contains "install-fish-tools --help mentions Usage" "$out" "Usage:"
end

function test_help_includes_enriched_sections
    set -l out (run_fish $SCRIPT help 2>&1)
    assert_contains "install-fish-tools help has Description" "$out" "Description:"
    assert_contains "install-fish-tools help has Examples" "$out" "Examples"
end

function test_unknown_argument_fails
    set -l out (run_fish $SCRIPT --bogus-flag 2>&1)
    set -l rc $status
    assert_eq "install-fish-tools unknown arg fails" $rc 1
    assert_contains "install-fish-tools unknown arg message" "$out" "Unknown argument"
end

function test_conf_dir_must_be_absolute
    set -l out (run_fish $SCRIPT install --conf-dir ./relative 2>&1)
    set -l rc $status
    assert_eq "install-fish-tools relative conf-dir fails" $rc 1
    assert_contains "install-fish-tools relative conf-dir message" "$out" "absolute"
end

function test_conf_dir_requires_value
    set -l out (run_fish $SCRIPT install --conf-dir 2>&1)
    set -l rc $status
    assert_eq "install-fish-tools --conf-dir without value fails" $rc 1
    assert_contains "install-fish-tools --conf-dir msg" "$out" "requires a value"
end

function test_status_does_not_modify_target
    # status should be read-only; running it against an empty conf-dir should
    # not write anything.
    set -l tmp (mk_tmpdir)
    set -l before (find $tmp -mindepth 1 2>/dev/null | wc -l | string trim)
    set -l out (run_fish $SCRIPT status --conf-dir $tmp 2>&1)
    set -l rc $status
    set -l after (find $tmp -mindepth 1 2>/dev/null | wc -l | string trim)
    assert_status "install-fish-tools status status" $rc 0
    assert_eq "install-fish-tools status did not write" "$after" "$before"
    assert_contains "install-fish-tools status reports not installed" "$out" "not installed"
end

function test_install_dry_run_writes_nothing
    set -l tmp (mk_tmpdir)
    set -l out (run_fish $SCRIPT install --conf-dir $tmp --dry-run --no-cleanup 2>&1)
    set -l rc $status
    set -l snippet "$tmp/agentic_tactical_boots.fish"
    assert_status "install --dry-run status" $rc 0
    if test -e "$snippet"
        __test_record_fail "install --dry-run wrote nothing" "snippet was created"
    else
        __test_record_pass "install --dry-run wrote nothing"
    end
    assert_contains "install --dry-run mentions Would write" "$out" "Would write snippet"
end

function test_install_creates_managed_snippet
    set -l tmp (mk_tmpdir)
    run_fish $SCRIPT install --conf-dir $tmp --no-cleanup >/dev/null
    set -l snippet "$tmp/agentic_tactical_boots.fish"
    if not test -f "$snippet"
        __test_record_fail "install creates snippet" "snippet missing"
        return
    end
    __test_record_pass "install creates snippet"
    set -l content (cat $snippet)
    assert_contains "snippet has marker" "$content" "managed-by: agentic_tactical_boots/install-fish-tools"
    assert_contains "snippet sets ATB_REPO_ROOT" "$content" "ATB_REPO_ROOT"
    assert_contains "snippet wraps sandboxctl" "$content" "function sandboxctl"
    assert_contains "snippet wraps slop" "$content" "function slop"
    assert_contains "snippet sources module loop" "$content" "for __atb_m in"
    assert_contains "snippet sources completions" "$content" "scripts/completions"
end

function test_generated_snippet_parses_as_fish
    set -l tmp (mk_tmpdir)
    run_fish $SCRIPT install --conf-dir $tmp --no-cleanup >/dev/null
    if command fish -n "$tmp/agentic_tactical_boots.fish" 2>/dev/null
        __test_record_pass "generated snippet parses as fish"
    else
        __test_record_fail "generated snippet parses as fish" "fish -n reported errors"
    end
end

function test_sourced_snippet_exposes_commands
    set -l tmp (mk_tmpdir)
    run_fish $SCRIPT install --conf-dir $tmp --no-cleanup >/dev/null
    set -l snippet "$tmp/agentic_tactical_boots.fish"

    # Module function: agent-sandbox is defined inside agent-sandbox.fish, so
    # sourcing the snippet should make it callable.
    set -l body "source '$snippet'; functions -q agent-sandbox; and echo MODULE_OK"
    set -l out (command fish -N -c "$body" 2>&1)
    assert_contains "snippet exposes agent-sandbox module function" "$out" "MODULE_OK"

    # Wrapper function: sandboxctl is defined as a thin wrapper in the snippet.
    set -l body2 "source '$snippet'; functions -q sandboxctl; and echo WRAPPER_OK"
    set -l out2 (command fish -N -c "$body2" 2>&1)
    assert_contains "snippet exposes sandboxctl wrapper" "$out2" "WRAPPER_OK"
end

function test_uninstall_removes_managed_snippet
    set -l tmp (mk_tmpdir)
    run_fish $SCRIPT install --conf-dir $tmp --no-cleanup >/dev/null
    run_fish $SCRIPT uninstall --conf-dir $tmp >/dev/null
    if test -e "$tmp/agentic_tactical_boots.fish"
        __test_record_fail "uninstall removed snippet" "snippet still present"
    else
        __test_record_pass "uninstall removed snippet"
    end
end

function test_uninstall_refuses_unmanaged_file
    # Drop a hand-written file with no marker; uninstall must refuse.
    set -l tmp (mk_tmpdir)
    set -l snippet "$tmp/agentic_tactical_boots.fish"
    echo "# user wrote this; not us" > "$snippet"
    set -l out (run_fish $SCRIPT uninstall --conf-dir $tmp 2>&1)
    set -l rc $status
    assert_eq "uninstall refuses unmanaged file" $rc 1
    assert_contains "uninstall mentions unmanaged" "$out" "unmanaged"
    if not test -f "$snippet"
        __test_record_fail "uninstall preserved unmanaged file" "file was deleted"
    else
        __test_record_pass "uninstall preserved unmanaged file"
    end
end

function test_install_no_cleanup_skips_legacy_check
    # The cleanup walks $HOME paths; --no-cleanup must not touch them.
    # We verify by asserting the install output does NOT mention "Removed
    # legacy" or "Would remove legacy", regardless of what is in $HOME.
    set -l tmp (mk_tmpdir)
    set -l out (run_fish $SCRIPT install --conf-dir $tmp --no-cleanup 2>&1)
    assert_not_contains "install --no-cleanup did not remove legacy" "$out" "Removed legacy"
    assert_not_contains "install --no-cleanup did not preview legacy" "$out" "Would remove legacy"
end

run_tests_in_file (basename (status filename))
