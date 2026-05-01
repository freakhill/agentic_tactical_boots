#!/usr/bin/env fish

# Tests for scripts/install-fish-tools.fish

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

function test_help_includes_enriched_sections
    set -l out (run_fish $SCRIPT help 2>&1)
    assert_contains "install-fish-tools help has Description" "$out" "Description:"
    assert_contains "install-fish-tools help has Examples" "$out" "Examples"
end

function test_dash_dash_help
    set -l out (run_fish $SCRIPT --help 2>&1)
    set -l rc $status
    assert_status "install-fish-tools --help status" $rc 0
    assert_contains "install-fish-tools --help mentions Usage" "$out" "Usage:"
end

function test_unknown_argument_fails
    set -l out (run_fish $SCRIPT --bogus-flag 2>&1)
    set -l rc $status
    assert_eq "install-fish-tools unknown arg fails" $rc 1
    assert_contains "install-fish-tools unknown arg message" "$out" "Unknown argument"
end

function test_target_must_be_absolute
    set -l out (run_fish $SCRIPT install --target ./relative 2>&1)
    set -l rc $status
    assert_eq "install-fish-tools relative target fails" $rc 1
    assert_contains "install-fish-tools relative target message" "$out" "absolute"
end

function test_target_requires_value
    set -l out (run_fish $SCRIPT install --target 2>&1)
    set -l rc $status
    assert_eq "install-fish-tools --target without value fails" $rc 1
    assert_contains "install-fish-tools --target msg" "$out" "requires a value"
end

function test_status_does_not_modify_target
    # `status` should be read-only; running it against an empty target should not
    # create wrappers. Use a fresh temp dir.
    set -l tmp (mk_tmpdir)
    set -l before (find $tmp -mindepth 1 2>/dev/null | wc -l | string trim)
    set -l out (run_fish $SCRIPT status --target $tmp 2>&1)
    set -l rc $status
    set -l after (find $tmp -mindepth 1 2>/dev/null | wc -l | string trim)
    assert_status "install-fish-tools status status" $rc 0
    assert_eq "install-fish-tools status did not write" "$after" "$before"
end

function test_install_dry_run_creates_no_wrappers
    set -l tmp (mk_tmpdir)
    set -l out (run_fish $SCRIPT install --target $tmp --dry-run 2>&1)
    set -l rc $status
    # In dry-run, no wrapper files should appear in $tmp/.local/bin.
    set -l created 0
    if test -d "$tmp/.local/bin"
        set created (count $tmp/.local/bin/*)
        if test $created -eq 1; and not test -e "$tmp/.local/bin/*"
            set created 0
        end
    end
    assert_status "install-fish-tools install --dry-run status" $rc 0
    assert_eq "install-fish-tools dry-run created nothing" $created 0
end

# Regression: when a previous stow install tree-folded a target subdirectory
# (so the dir itself is a symlink into the repo's stow source), re-running
# install / uninstall must NOT follow the symlink and delete the source file.
# https://github.com/freakhill/agentic_tactical_boots — the bug deleted 11
# tracked files from stow/fish-tools/share/fish/.
function test_remove_direct_skips_symlinked_parent
    set -l tmp (mk_tmpdir)
    set -l fixture "$tmp/fixture"
    mkdir -p $fixture
    # Drop a marked file matching one of the managed names into the fixture.
    # Without the guard, __ift_remove_direct would resolve the symlink and
    # delete this file.
    echo "# managed-by: agentic_tactical_boots/install-fish-tools" \
        > "$fixture/agentic_tactical_boots.fish"

    mkdir -p "$tmp/.local/share/fish"
    ln -s "$fixture" "$tmp/.local/share/fish/vendor_conf.d"

    # Drive __ift_remove_direct through `uninstall --dry-run`. The guard's
    # dry-run branch prints a recognizable message, and dry-run avoids
    # invoking stow.
    set -l out (run_fish $SCRIPT uninstall --target $tmp --dry-run 2>&1)
    set -l rc $status
    assert_status "uninstall --dry-run status" $rc 0
    assert_contains "guard reported skip for symlinked parent" "$out" "Skipping (parent is a symlink"

    # And the real (non-dry-run) uninstall must preserve the fixture file.
    # This is the bit that catches a regression where someone removes the
    # guard while keeping the dry-run message intact by accident.
    run_fish $SCRIPT uninstall --target $tmp 2>&1 >/dev/null
    if test -f "$fixture/agentic_tactical_boots.fish"
        __test_record_pass "fixture file survived real uninstall"
    else
        __test_record_fail "fixture file survived real uninstall" "fixture file was deleted via symlinked parent"
    end
end

run_tests_in_file (basename (status filename))
