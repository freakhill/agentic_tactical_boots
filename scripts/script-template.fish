#!/usr/bin/env fish

# Purpose:
# - <what this script does>
# - <why it exists>
#
# Safety/model notes:
# - <security assumptions and default policy>
#
# References:
# - <official doc link 1>
# - <official doc link 2>

function __example_usage
    echo "Usage:"
    echo "  source scripts/<file>.fish"
    echo "  <command> <subcommand> [options]"
    echo "  <command> --help"
    echo ""
    echo "Notes:"
    echo "  - <important operational note>"
end

function <command> --description "<short description>"
    if test (count $argv) -eq 0
        __example_usage
        return 0
    end

    set -l subcmd "$argv[1]"
    set -e argv[1]

    switch "$subcmd"
        case help --help -h
            __example_usage
        case '*'
            echo "Unknown command: $subcmd" 1>&2
            __example_usage
            return 1
    end
end
