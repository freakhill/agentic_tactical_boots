# Tiny test helpers for fish scripts in this repo.
# Why: keep tests dependency-free so CI on a stock Ubuntu runner only needs fish.
#
# Each test file sources this helpers file, defines `test_*` functions, then
# calls `run_tests_in_file`. The runner aggregates results across files.

set -g __TEST_PASSED 0
set -g __TEST_FAILED 0
set -g __TEST_FAIL_LOG

set -g REPO_ROOT (cd (dirname (status filename))/..; pwd)
set -g SCRIPTS_DIR "$REPO_ROOT/scripts"

# fish 4.x does not always propagate stdout when invoking nested `fish` via the
# function lookup path (e.g. when a user has a fish shell function named `fish`
# in their config). Resolving the absolute fish binary avoids that, and also
# means `env VAR=val "$FISH_BIN" ...` works on every platform — `env` cannot
# invoke shell builtins like `command`.
set -g FISH_BIN (command -v fish)

function run_fish
    $FISH_BIN $argv
end

function __test_record_pass --argument-names name
    set __TEST_PASSED (math "$__TEST_PASSED + 1")
    echo "  ok    $name"
end

function __test_record_fail --argument-names name reason
    set __TEST_FAILED (math "$__TEST_FAILED + 1")
    set -a __TEST_FAIL_LOG "$name: $reason"
    echo "  FAIL  $name -- $reason" 1>&2
end

# assert_eq <name> <actual> <expected>
function assert_eq --argument-names name actual expected
    if test "$actual" = "$expected"
        __test_record_pass "$name"
    else
        __test_record_fail "$name" "expected '$expected', got '$actual'"
    end
end

# assert_status <name> <actual_status> <expected_status>
function assert_status --argument-names name actual expected
    if test "$actual" -eq "$expected"
        __test_record_pass "$name"
    else
        __test_record_fail "$name" "expected exit status $expected, got $actual"
    end
end

# assert_contains <name> <haystack> <needle>
function assert_contains --argument-names name haystack needle
    if string match -q "*$needle*" -- "$haystack"
        __test_record_pass "$name"
    else
        __test_record_fail "$name" "output did not contain '$needle'"
    end
end

# assert_not_contains <name> <haystack> <needle>
function assert_not_contains --argument-names name haystack needle
    if string match -q "*$needle*" -- "$haystack"
        __test_record_fail "$name" "output unexpectedly contained '$needle'"
    else
        __test_record_pass "$name"
    end
end

# Run a fish snippet in a fresh `fish -c` and capture stdout+stderr+status.
# Usage:
#   set -l result (capture_fish "echo hi")
#   echo $result[1]   # status
#   echo $result[2..] # combined output (one line per element)
function capture_fish --argument-names snippet
    set -l tmp (mktemp)
    fish -c "$snippet" >$tmp 2>&1
    set -l rc $status
    set -l out (cat $tmp)
    rm -f $tmp
    echo $rc
    for line in $out
        echo $line
    end
end

# Run a single command (argv) and capture combined output + status.
# Returns multi-line: first line = status, rest = output.
function capture_cmd
    set -l tmp (mktemp)
    $argv >$tmp 2>&1
    set -l rc $status
    set -l out (cat $tmp)
    rm -f $tmp
    echo $rc
    for line in $out
        echo $line
    end
end

# Discover and run every function named test_* defined in the current shell,
# then exit with non-zero if any test failed.
function run_tests_in_file
    set -l file_label "$argv[1]"
    if test -z "$file_label"
        set file_label (status filename)
    end
    echo "=== $file_label ==="

    set -l names (functions --names | string match -r '^test_.*$')
    if test (count $names) -eq 0
        echo "  (no tests defined)"
        return 0
    end

    for name in $names
        eval $name
    end

    if test $__TEST_FAILED -gt 0
        echo "  $__TEST_FAILED failed in this file" 1>&2
        return 1
    end
    echo "  $__TEST_PASSED ok"
    return 0
end
