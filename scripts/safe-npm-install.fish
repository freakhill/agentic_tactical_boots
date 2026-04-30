#!/usr/bin/env fish

# Why this wrapper exists:
# - `npm` lifecycle scripts can execute arbitrary shell code at install time.
# - We enforce lockfile-only + ignore-scripts for safer automation defaults.
#
# References:
# - npm ci: https://docs.npmjs.com/cli/v10/commands/npm-ci
# - npm scripts/lifecycle: https://docs.npmjs.com/cli/v10/using-npm/scripts

function __safe_npm_usage
    echo "Usage:"
    echo "  source scripts/safe-npm-install.fish"
    echo "  safe-npm-install"
    echo "  safe-npm-install --help"
    echo ""
    echo "Notes:"
    echo "  - Requires package-lock.json in current directory."
    echo "  - Uses npm ci with --ignore-scripts and no audit/fund prompts."
end

function safe-npm-install --description "Install npm dependencies with safer defaults"
    if test (count $argv) -eq 1; and contains -- "$argv[1]" --help -h help
        __safe_npm_usage
        return 0
    end

    if not test -f package-lock.json
        echo "package-lock.json is required for safe-npm-install" 1>&2
        return 1
    end

    if test (count $argv) -gt 0
        echo "safe-npm-install does not accept package names; use lockfile only" 1>&2
        return 1
    end

    npm ci --ignore-scripts --no-audit --fund=false
end
