#!/usr/bin/env fish

# Test runner: discovers tests/test_*.fish and runs each in its own fish process
# so per-file function definitions and global counters do not leak.
#
# Usage:
#   fish tests/run.fish [pattern]
#
# If pattern is given, only test files whose path matches it are run.

set -l tests_dir (cd (dirname (status filename)); pwd)
set -l pattern "$argv[1]"

set -l files (ls $tests_dir/test_*.fish 2>/dev/null | sort)

if test (count $files) -eq 0
    echo "No test files found in $tests_dir" 1>&2
    exit 1
end

set -l total_files 0
set -l failed_files 0
set -l failed_names

for f in $files
    if test -n "$pattern"; and not string match -q "*$pattern*" -- "$f"
        continue
    end
    set total_files (math "$total_files + 1")
    command fish "$f"
    if test $status -ne 0
        set failed_files (math "$failed_files + 1")
        set -a failed_names (basename "$f")
    end
end

echo ""
echo "==============================="
echo "Test files run:    $total_files"
echo "Test files failed: $failed_files"
if test $failed_files -gt 0
    echo "Failures:"
    for n in $failed_names
        echo "  - $n"
    end
    exit 1
end
echo "All tests passed."
exit 0
