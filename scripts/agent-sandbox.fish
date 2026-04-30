#!/usr/bin/env fish

# Purpose:
# - Run the agent container with a predictable command interface.
# - Default to strict-egress policy to reduce accidental outbound access.
#
# References:
# - Docker Compose networking: https://docs.docker.com/compose/networking/

function __agent_sandbox_usage
    echo "Usage:"
    echo "  source scripts/agent-sandbox.fish"
    echo "  agent-sandbox run [--network-policy strict-egress|proxy-only|off] [command ...]"
    echo "  agent-sandbox shell [--network-policy strict-egress|proxy-only|off]"
    echo "  agent-sandbox up"
    echo "  agent-sandbox down"
    echo "  agent-sandbox help"
    echo ""
    echo "Notes:"
    echo "  - Host project is mounted at /workspace inside container."
    echo "  - In strict-egress mode, agent only reaches network through proxy service."
end

# Keep compose file checks centralized so every subcommand fails consistently.
function __agent_sandbox_check_files
    if not test -f examples/docker-compose.yml
        echo "Missing examples/docker-compose.yml" 1>&2
        return 1
    end
end

# Allowed values are explicit to avoid insecure typos.
function __agent_sandbox_validate_policy --argument-names policy
    if not contains -- "$policy" strict-egress proxy-only off
        echo "Invalid --network-policy: $policy" 1>&2
        return 1
    end
end

function __agent_sandbox_compose_cmd --argument-names policy
    # The policy switch is currently routing-compatible for all modes.
    # Keeping this shim lets us add stricter mode-specific compose files later
    # without changing command UX.
    set -e argv[1]
    switch "$policy"
        case strict-egress
            docker compose -f examples/docker-compose.yml $argv
        case proxy-only off
            docker compose -f examples/docker-compose.yml $argv
    end
end

function agent-sandbox --description "Run commands in agent sandbox container"
    if test (count $argv) -eq 0
        __agent_sandbox_usage
        return 0
    end

    set -l cmd "$argv[1]"
    set -e argv[1]

    if test "$cmd" = "--help"; or test "$cmd" = "-h"; or test "$cmd" = "help"
        __agent_sandbox_usage
        return 0
    end

    __agent_sandbox_check_files; or return 1

    set -l policy "strict-egress"
    if test (count $argv) -ge 2; and test "$argv[1]" = "--network-policy"
        set policy "$argv[2]"
        set -e argv[1..2]
    end

    __agent_sandbox_validate_policy "$policy"; or return 1

    switch "$cmd"
        case run
            __agent_sandbox_compose_cmd "$policy" build agent
            and __agent_sandbox_compose_cmd "$policy" up -d proxy
            and __agent_sandbox_compose_cmd "$policy" run --rm agent $argv
        case shell
            __agent_sandbox_compose_cmd "$policy" build agent
            and __agent_sandbox_compose_cmd "$policy" up -d proxy
            and __agent_sandbox_compose_cmd "$policy" run --rm agent
        case up
            __agent_sandbox_compose_cmd "$policy" build agent
            and __agent_sandbox_compose_cmd "$policy" up -d proxy
        case down
            __agent_sandbox_compose_cmd "$policy" down
        case '*'
            echo "Unknown command: $cmd" 1>&2
            __agent_sandbox_usage
            return 1
    end
end
