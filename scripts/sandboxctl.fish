#!/usr/bin/env fish

# Unified command hub for this repository's sandbox/key scripts.
# Why: one memorisable entrypoint reduces operator error and shortens onboarding.

set -l script_dir (cd (dirname (status filename)); pwd)

function __sandboxctl_usage
    echo "Usage:"
    echo "  scripts/sandboxctl.fish help"
    echo "  scripts/sandboxctl.fish list"
    echo "  scripts/sandboxctl.fish tutorial <topic>"
    echo "  scripts/sandboxctl.fish docker <args...>"
    echo "  scripts/sandboxctl.fish docker-tools <args...>"
    echo "  scripts/sandboxctl.fish brew-vm <args...>"
    echo "  scripts/sandboxctl.fish github <args...>"
    echo "  scripts/sandboxctl.fish forgejo <args...>"
    echo "  scripts/sandboxctl.fish radicle <args...>"
    echo ""
    echo "Topics: docker, brew-vm, github-keys, forgejo-keys, radicle-access, network-limiting, file-sharing"
end

function __sandboxctl_list
    echo "Commands:"
    echo "  docker         -> scripts/agent-sandbox.fish (agent-sandbox ...)"
    echo "  docker-tools   -> scripts/agent-sandbox-tools.fish (agent-sandbox-tools ...)"
    echo "  brew-vm        -> scripts/brew-vm.fish (brew-vm ...)"
    echo "  github         -> scripts/llm-github-keys.fish (llm-gh-key ...)"
    echo "  forgejo        -> scripts/llm-forgejo-keys.fish (llm-forgejo-key ...)"
    echo "  radicle        -> scripts/llm-radicle-access.fish (llm-radicle-access ...)"
    echo "  safe-npm       -> scripts/safe-npm-install.fish (safe-npm-install)"
    echo "  safe-uv        -> scripts/safe-uv-install.fish (safe-uv ...)"
    echo "  pinning        -> scripts/check-pinning.fish"
end

function __sandboxctl_tutorial --argument-names topic
    switch "$topic"
        case docker
            echo "Docker sandbox quickstart:"
            echo "  scripts/sandboxctl.fish docker up"
            echo "  scripts/sandboxctl.fish docker shell"
            echo "  scripts/sandboxctl.fish docker down"
        case brew-vm
            echo "Brew VM quickstart:"
            echo "  source scripts/brew-vm.fish"
            echo "  set -x BREW_VM_PROXY_URL http://<proxy-host>:3128"
            echo "  brew-vm create-base"
            echo "  brew-vm install --network-policy strict-egress wget"
        case github-keys
            echo "GitHub key quickstart:"
            echo "  source scripts/llm-github-keys.fish"
            echo "  llm-gh-key create-pair --repo <owner>/<repo> --name session-1 --ttl 24h --install-ssh-config"
        case forgejo-keys
            echo "Forgejo key quickstart:"
            echo "  source scripts/llm-forgejo-keys.fish"
            echo "  llm-forgejo-key bootstrap-config"
            echo "  llm-forgejo-key create-pair --instance main --repo <owner>/<repo> --name session-1 --ttl 24h"
        case radicle-access
            echo "Radicle access quickstart:"
            echo "  source scripts/llm-radicle-access.fish"
            echo "  llm-radicle-access create-identity --name session-1 --ttl 24h"
            echo "  llm-radicle-access bind-repo --rid <rad:...> --identity-id <id> --access ro"
        case network-limiting
            echo "Network limiting quickstart:"
            echo "  1) Keep strict-egress defaults in sandbox scripts"
            echo "  2) Use examples/allowlist.domains to manage explicit outbound domains"
            echo "  3) For VM sessions set BREW_VM_PROXY_URL and run: brew-vm verify-network"
        case file-sharing
            echo "File sharing quickstart:"
            echo "  Docker: host repo is /workspace in containers"
            echo "  VM: use explicit transfers only: brew-vm copy-in <host> <guest> / brew-vm copy-out <guest> <host>"
            echo "  Recommended guest share dir: /tmp/llm-share"
        case '*'
            echo "Unknown tutorial topic: $topic" 1>&2
            __sandboxctl_usage
            return 1
    end
end

if test (count $argv) -eq 0
    __sandboxctl_usage
    exit 0
end

set -l cmd "$argv[1]"
set -e argv[1]

switch "$cmd"
    case help --help -h
        __sandboxctl_usage
    case list
        __sandboxctl_list
    case tutorial
        if test (count $argv) -ne 1
            echo "Usage: scripts/sandboxctl.fish tutorial <topic>" 1>&2
            exit 1
        end
        __sandboxctl_tutorial "$argv[1]"
    case docker
        fish -c "source '$script_dir/agent-sandbox.fish'; agent-sandbox $argv"
    case docker-tools
        fish -c "source '$script_dir/agent-sandbox-tools.fish'; agent-sandbox-tools $argv"
    case brew-vm
        fish -c "source '$script_dir/brew-vm.fish'; brew-vm $argv"
    case github
        fish -c "source '$script_dir/llm-github-keys.fish'; llm-gh-key $argv"
    case forgejo
        fish -c "source '$script_dir/llm-forgejo-keys.fish'; llm-forgejo-key $argv"
    case radicle
        fish -c "source '$script_dir/llm-radicle-access.fish'; llm-radicle-access $argv"
    case safe-npm
        fish -c "source '$script_dir/safe-npm-install.fish'; safe-npm-install $argv"
    case safe-uv
        fish -c "source '$script_dir/safe-uv-install.fish'; safe-uv $argv"
    case pinning
        fish "$script_dir/check-pinning.fish" $argv
    case '*'
        echo "Unknown command: $cmd" 1>&2
        __sandboxctl_usage
        exit 1
end
