#!/usr/bin/env fish

# Purpose:
# - Same UX as agent-sandbox, but targets tool-preinstalled runtime.
# - Defaults to strict-egress to keep dependency/tooling pulls behind proxy.
#
# References:
# - Docker Compose env files: https://docs.docker.com/compose/environment-variables/

function __agent_sandbox_tools_usage
    echo "Usage:"
    echo "  source scripts/agent-sandbox-tools.fish"
    echo "  agent-sandbox-tools run [--network-policy strict-egress|proxy-only|off] [command ...]"
    echo "  agent-sandbox-tools shell [--network-policy strict-egress|proxy-only|off]"
    echo "  agent-sandbox-tools up"
    echo "  agent-sandbox-tools down"
    echo "  agent-sandbox-tools help"
    echo ""
    echo "Notes:"
    echo "  - Host project is mounted at /workspace inside container."
    echo "  - In strict-egress mode, agent-tools only reaches network through proxy service."
end

# Keep compose file checks centralized so every subcommand fails consistently.
function __agent_sandbox_tools_check_files
    if not test -f examples/docker-compose.yml
        echo "Missing examples/docker-compose.yml" 1>&2
        return 1
    end
end

# Allowed values are explicit to avoid insecure typos.
function __agent_sandbox_tools_validate_policy --argument-names policy
    if not contains -- "$policy" strict-egress proxy-only off
        echo "Invalid --network-policy: $policy" 1>&2
        return 1
    end
end

function __agent_sandbox_tools_compose_cmd --argument-names policy
    # We intentionally preserve support for optional examples/agent-tools.env.
    # This keeps pinned versions configurable without changing command UX.
    set -e argv[1]

    if test -f examples/agent-tools.env
        docker compose --env-file examples/agent-tools.env -f examples/docker-compose.yml $argv
    else
        docker compose -f examples/docker-compose.yml $argv
    end
end

function agent-sandbox-tools --description "Run commands in tool-preinstalled sandbox container"
    if test (count $argv) -eq 0
        __agent_sandbox_tools_usage
        return 0
    end

    set -l cmd "$argv[1]"
    set -e argv[1]

    if test "$cmd" = "--help"; or test "$cmd" = "-h"; or test "$cmd" = "help"
        __agent_sandbox_tools_usage
        return 0
    end

    __agent_sandbox_tools_check_files; or return 1

    set -l policy "strict-egress"
    if test (count $argv) -ge 2; and test "$argv[1]" = "--network-policy"
        set policy "$argv[2]"
        set -e argv[1..2]
    end

    __agent_sandbox_tools_validate_policy "$policy"; or return 1

    switch "$cmd"
        case run
            __agent_sandbox_tools_compose_cmd "$policy" build agent-tools
            and __agent_sandbox_tools_compose_cmd "$policy" up -d proxy
            and __agent_sandbox_tools_compose_cmd "$policy" run --rm agent-tools $argv
        case shell
            __agent_sandbox_tools_compose_cmd "$policy" build agent-tools
            and __agent_sandbox_tools_compose_cmd "$policy" up -d proxy
            and __agent_sandbox_tools_compose_cmd "$policy" run --rm agent-tools
        case up
            __agent_sandbox_tools_compose_cmd "$policy" build agent-tools
            and __agent_sandbox_tools_compose_cmd "$policy" up -d proxy
        case down
            __agent_sandbox_tools_compose_cmd "$policy" down
        case '*'
            echo "Unknown command: $cmd" 1>&2
            __agent_sandbox_tools_usage
            return 1
    end
end
