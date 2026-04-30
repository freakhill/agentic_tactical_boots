#!/usr/bin/env fish

# Why this wrapper exists:
# - Keep Python dependency installs reproducible and less exposed to arbitrary
#   build execution by requiring frozen lock sync and pinned wheels-only install.
#
# References:
# - uv sync: https://docs.astral.sh/uv/concepts/projects/sync/
# - uv pip install: https://docs.astral.sh/uv/pip/

function __safe_uv_usage
    echo "Usage:"
    echo "  source scripts/safe-uv-install.fish"
    echo "  safe-uv sync"
    echo "  safe-uv pip-install <name==version>"
    echo "  safe-uv --help"
    echo ""
    echo "Notes:"
    echo "  - safe-uv sync requires uv.lock."
    echo "  - pip-install enforces pinned package and wheels-only install."
end

function safe-uv-sync --description "Sync dependencies with frozen lockfile"
    if not test -f uv.lock
        echo "uv.lock is required for safe-uv-sync" 1>&2
        return 1
    end

    uv sync --frozen
end

function safe-uv-pip-install --description "Install pinned wheel-only package in active env"
    if test (count $argv) -ne 1
        echo "Usage: safe-uv-pip-install <name==version>" 1>&2
        return 1
    end

    set pkg "$argv[1]"
    if not string match -rq '.+==.+' -- "$pkg"
        echo "Package must be pinned as name==version" 1>&2
        return 1
    end

    uv pip install --only-binary :all: "$pkg"
end

function safe-uv --description "Unified wrapper for safe uv operations"
    if test (count $argv) -eq 0
        __safe_uv_usage
        return 0
    end

    set -l cmd "$argv[1]"
    set -e argv[1]

    switch "$cmd"
        case sync
            safe-uv-sync $argv
        case pip-install
            safe-uv-pip-install $argv
        case --help -h help
            __safe_uv_usage
        case '*'
            echo "Unknown command: $cmd" 1>&2
            __safe_uv_usage
            return 1
    end
end
