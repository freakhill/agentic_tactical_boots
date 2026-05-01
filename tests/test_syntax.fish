#!/usr/bin/env fish

# Why: any change that breaks fish parsing should be caught before runtime.
# script-template.fish is a documented placeholder template (uses <command> tokens
# that are not valid fish), so it is excluded.

source (dirname (status filename))/helpers.fish

set -g SCRIPT_TEMPLATE_BASENAME "script-template.fish"

function test_all_real_scripts_parse
    set -l failures
    for f in $SCRIPTS_DIR/*.fish
        if test (basename "$f") = "$SCRIPT_TEMPLATE_BASENAME"
            continue
        end
        if not command fish -n "$f" 2>/dev/null
            set -a failures (basename "$f")
        end
    end
    assert_eq "all-non-template-scripts-parse" (count $failures) 0
    if test (count $failures) -gt 0
        for n in $failures
            echo "    syntax error in: $n" 1>&2
        end
    end
end

function test_template_is_marked_template
    # The template intentionally has placeholders. We assert it is in the repo and
    # is not silently re-promoted to a real script.
    set -l path "$SCRIPTS_DIR/$SCRIPT_TEMPLATE_BASENAME"
    if not test -f "$path"
        __test_record_fail "template-exists" "expected $path to exist"
        return
    end
    set -l first_lines (head -n 5 "$path")
    set -l joined (string join "\n" $first_lines)
    assert_contains "template-self-identifies" "$joined" "Purpose"
end

run_tests_in_file (basename (status filename))
