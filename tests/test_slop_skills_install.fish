#!/usr/bin/env fish

# Tests for scripts/slop-skills-install.fish

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/slop-skills-install.fish"

function test_help_flag
    set -l out (run_fish $SCRIPT --help 2>&1)
    set -l rc $status
    assert_status "slop-skills-install --help status" $rc 0
    assert_contains "slop-skills-install --help mentions Usage" "$out" "Usage:"
    assert_contains "slop-skills-install --help mentions copy target" "$out" "~/.claude/skills"
end

function test_help_includes_enriched_sections
    set -l out (run_fish $SCRIPT --help 2>&1)
    assert_contains "slop-skills-install help has Description" "$out" "Description:"
    assert_contains "slop-skills-install help has Examples" "$out" "Examples:"
end

function test_unknown_argument_fails
    set -l out (run_fish $SCRIPT --bogus-flag 2>&1)
    set -l rc $status
    assert_eq "slop-skills-install unknown arg fails" $rc 1
    assert_contains "slop-skills-install unknown arg message" "$out" "Unknown argument"
end

function test_dry_run_does_not_write
    # Run with HOME pointed at a tmp dir so dst_dir = $tmp/.claude/skills.
    set -l tmp (mk_tmpdir)
    set -l out (env HOME=$tmp $FISH_BIN $SCRIPT --dry-run 2>&1)
    set -l rc $status

    set -l dst "$tmp/.claude/skills"
    set -l copied 0
    if test -d "$dst"
        # mkdir -p is allowed even in dry-run; we only assert no skill subdirs were created.
        for entry in $dst/*
            if test -d "$entry"
                set copied (math "$copied + 1")
            end
        end
    end

    assert_status "slop-skills-install --dry-run status" $rc 0
    assert_eq "slop-skills-install --dry-run wrote no skill dirs" $copied 0
    assert_contains "slop-skills-install --dry-run output uses 'Would'" "$out" "Would copy"
end

function test_real_install_copies_skill_dirs
    set -l tmp (mk_tmpdir)
    set -l out (env HOME=$tmp $FISH_BIN $SCRIPT 2>&1)
    set -l rc $status

    set -l dst "$tmp/.claude/skills"
    set -l copied 0
    if test -d "$dst"
        for entry in $dst/*
            if test -d "$entry"
                set copied (math "$copied + 1")
            end
        end
    end

    assert_status "slop-skills-install install status" $rc 0
    # The repo ships at least one skill dir (agent-sandbox-ops, agent-key-lifecycle).
    if test $copied -lt 1
        __test_record_fail "slop-skills-install installed at least one skill" "expected >= 1 skill dir, got $copied"
    else
        __test_record_pass "slop-skills-install installed at least one skill"
    end
    assert_contains "slop-skills-install output mentions Installed" "$out" "Installed skill"
end

run_tests_in_file (basename (status filename))
