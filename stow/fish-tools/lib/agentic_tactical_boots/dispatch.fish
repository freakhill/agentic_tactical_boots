#!/usr/bin/env fish

# managed-by: agentic_tactical_boots/install-fish-tools

set -g __atb_cfg "$HOME/.config/agentic_tactical_boots/fish-tools.env"

function __atb_load_state
    if not test -f "$__atb_cfg"
        echo "Tool install state not found: $__atb_cfg" 1>&2
        echo "Run: scripts/install-fish-tools.fish install" 1>&2
        return 1
    end

    source "$__atb_cfg"

    if test -z "$ATB_REPO_ROOT"; or test -z "$ATB_INSTALL_MODE"; or test -z "$ATB_TARGET"
        echo "Invalid tool install state file: $__atb_cfg" 1>&2
        return 1
    end
end

function __atb_maybe_migrate
    if test "$ATB_INSTALL_MODE" != "direct"
        return 0
    end

    if not command -q stow
        return 0
    end

    set -l installer "$ATB_REPO_ROOT/scripts/install-fish-tools.fish"
    if not test -f "$installer"
        return 0
    end

    fish "$installer" install --target "$ATB_TARGET" >/dev/null 2>/dev/null; or return 0
    echo "Migrated to stow mode."
    __atb_load_state; or return 1
end

function atb-dispatch --argument-names mode rel_script rel_fn
    set -e argv[1..3]

    # Preserve the caller's cwd as ATB_USER_PWD before we cd into the repo
    # root below. Tools that infer context from cwd (e.g. `llm-gh-key here`,
    # which reads the user's git remote) need the original directory, not the
    # internal repo where dispatched scripts run.
    if not set -q ATB_USER_PWD
        set -gx ATB_USER_PWD "$PWD"
    end

    __atb_load_state; or return 1
    __atb_maybe_migrate; or return 1

    set -l script "$ATB_REPO_ROOT/$rel_script"
    if not test -f "$script"
        echo "Missing script: $script" 1>&2
        return 1
    end

    cd "$ATB_REPO_ROOT"; or return 1

    switch "$mode"
        case exec
            fish "$script" $argv
        case source
            set -l escaped (string escape -- $argv)
            fish -c "source '$script'; $rel_fn $escaped"
        case '*'
            echo "Unknown dispatch mode: $mode" 1>&2
            return 1
    end
end
