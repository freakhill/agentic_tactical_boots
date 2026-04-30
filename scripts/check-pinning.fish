#!/usr/bin/env fish

# Why this check exists:
# - Prevent drift to unpinned or `latest` tool versions in sandbox images.
# - Keep automation reproducible and supply-chain risk reviewable.

if test (count $argv) -gt 0; and contains -- "$argv[1]" --help -h help
    echo "Usage:"
    echo "  ./scripts/check-pinning.fish"
    echo ""
    echo "Checks:"
    echo "  - Ensures pinned CLI versions are not set to latest"
    echo "  - Ensures Dockerfile and compose defaults are pinned"
    echo "  - Ensures uv pip installs in tool image are exact-version pins"
    exit 0
end

set -l files \
    examples/agent-tools.env \
    examples/agent-tools.env.example \
    examples/Dockerfile.agent.tools \
    examples/docker-compose.yml

set -l failed 0

for f in $files
    if not test -f $f
        echo "missing required file: $f" 1>&2
        set failed 1
    end
end

if test $failed -eq 1
    exit 1
end

if grep -nE '^(CLAUDE_CODE_VERSION|OPENCODE_VERSION)=latest$' examples/agent-tools.env examples/agent-tools.env.example >/dev/null
    echo "unpinned npm CLI version found in agent-tools env files" 1>&2
    grep -nE '^(CLAUDE_CODE_VERSION|OPENCODE_VERSION)=latest$' examples/agent-tools.env examples/agent-tools.env.example 1>&2
    set failed 1
end

if grep -nE '^ARG (CLAUDE_CODE_VERSION|OPENCODE_VERSION)=latest$' examples/Dockerfile.agent.tools >/dev/null
    echo "unpinned npm CLI ARG default found in examples/Dockerfile.agent.tools" 1>&2
    grep -nE '^ARG (CLAUDE_CODE_VERSION|OPENCODE_VERSION)=latest$' examples/Dockerfile.agent.tools 1>&2
    set failed 1
end

if grep -nE '(CLAUDE_CODE_VERSION|OPENCODE_VERSION): \$\{\1:-latest\}' examples/docker-compose.yml >/dev/null
    echo "unpinned compose build arg default found in examples/docker-compose.yml" 1>&2
    grep -nE '(CLAUDE_CODE_VERSION|OPENCODE_VERSION): \$\{\1:-latest\}' examples/docker-compose.yml 1>&2
    set failed 1
end

set -l unpinned_uv_lines (grep -n 'uv pip install' examples/Dockerfile.agent.tools | grep -v '==')
if test (count $unpinned_uv_lines) -gt 0
    echo "found uv pip install without exact pins in examples/Dockerfile.agent.tools" 1>&2
    for line in $unpinned_uv_lines
        echo $line 1>&2
    end
    set failed 1
end

if test $failed -eq 1
    echo "pinning check failed" 1>&2
    exit 1
end

echo "pinning check passed"
