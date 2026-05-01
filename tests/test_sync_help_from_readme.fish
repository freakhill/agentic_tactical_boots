#!/usr/bin/env fish

# Tests for scripts/sync-help-from-readme.fish — README→help generator.
# We exercise help paths and a controlled sync against a fixture script
# placed in a temp scripts dir, so the test does not mutate real scripts/*.

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/sync-help-from-readme.fish"
set -g HELPER_PY "$SCRIPTS_DIR/_py/sync_help_from_readme.py"

function test_help_subcommand
    set -l out (run_fish $SCRIPT help 2>&1)
    set -l rc $status
    assert_status "sync-help help status" $rc 0
    assert_contains "sync-help help mentions Usage" "$out" "Usage:"
    assert_contains "sync-help help mentions Examples" "$out" "Examples:"
    assert_contains "sync-help help mentions Subcommands" "$out" "Subcommands:"
end

function test_no_args_prints_help_and_fails
    # No args is an error-path; per project pattern, error paths print full
    # help (not just one-line usage) so users see examples next to the error.
    set -l out (run_fish $SCRIPT 2>&1)
    set -l rc $status
    assert_eq "sync-help no-args fails" $rc 1
    assert_contains "sync-help no-args mentions Usage" "$out" "Usage:"
end

function test_unknown_subcommand_fails
    set -l out (run_fish $SCRIPT bogus 2>&1)
    set -l rc $status
    assert_eq "sync-help unknown subcmd fails" $rc 1
    assert_contains "sync-help unknown subcmd error message" "$out" "unknown subcommand"
    assert_contains "sync-help unknown subcmd shows help" "$out" "Usage:"
end

function test_python_helper_exists
    if test -f "$HELPER_PY"
        __test_record_pass "sync-help python helper exists"
    else
        __test_record_fail "sync-help python helper exists" "missing $HELPER_PY"
    end
end

function test_check_against_fixture
    if not command -sq uv
        __test_record_pass "sync-help check (skipped: uv not installed)"
        return 0
    end

    set -l tmp (mk_tmpdir)
    set -l fixture_readme "$tmp/README.md"
    set -l fixture_scripts "$tmp/scripts"
    mkdir -p "$fixture_scripts"

    printf '%s\n' \
        '# Fixture' \
        '' \
        '## Heading One' \
        '' \
        '1. Step intro:' \
        '' \
        '```fish' \
        'echo hello' \
        '```' \
        > "$fixture_readme"

    printf '%s\n' \
        '#!/usr/bin/env fish' \
        'function fixture_examples' \
        '    # BEGIN AUTOGEN: examples section="Heading One"' \
        '    echo "stale"' \
        '    # END AUTOGEN: examples' \
        'end' \
        > "$fixture_scripts/fixture.fish"

    # Drift expected → check should fail.
    uv run --script "$HELPER_PY" --repo-root "$tmp" check >/dev/null 2>&1
    set -l rc $status
    assert_eq "sync-help check detects drift" $rc 1

    # Apply sync, then check again — should pass.
    uv run --script "$HELPER_PY" --repo-root "$tmp" sync >/dev/null 2>&1
    set -l sync_rc $status
    assert_eq "sync-help sync succeeds" $sync_rc 0

    uv run --script "$HELPER_PY" --repo-root "$tmp" check >/dev/null 2>&1
    set -l recheck_rc $status
    assert_eq "sync-help check passes after sync" $recheck_rc 0

    set -l content (cat "$fixture_scripts/fixture.fish")
    assert_contains "sync-help generated step caption" "$content" "Step intro:"
    assert_contains "sync-help generated code line" "$content" "echo hello"
end

run_tests_in_file (basename (status filename))
