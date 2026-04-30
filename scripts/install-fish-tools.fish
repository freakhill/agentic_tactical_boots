#!/usr/bin/env fish

# Purpose:
# - Install repo tool shims into fish-friendly command paths.
# - Prefer GNU Stow for git-synced symlink management, with direct-copy fallback.
#
# Safety/model notes:
# - Only managed wrapper files are replaced/removed unless --force is provided.
# - Installer records state so direct installs can auto-migrate to stow later.
#
# References:
# - GNU Stow manual: https://www.gnu.org/software/stow/manual/stow.html
# - Fish docs: https://fishshell.com/docs/current/

set -g __ift_marker "managed-by: agentic_tactical_boots/install-fish-tools"
set -g __ift_repo_root (cd (dirname (status filename))/..; pwd)
set -g __ift_pkg_dir "$__ift_repo_root/stow/fish-tools"
set -g __ift_default_target "$HOME"
set -g __ift_state_dir "$HOME/.config/agentic_tactical_boots"
set -g __ift_state_file "$__ift_state_dir/fish-tools.env"
set -g __ift_bin_cmds \
    sandboxctl \
    agent-sandbox \
    agent-sandbox-tools \
    macos-sandbox \
    brew-vm \
    llm-gh-key \
    llm-forgejo-key \
    llm-radicle-access \
    safe-npm-install \
    safe-uv \
    check-pinning

function __ift_usage
    echo "Usage:"
    echo "  scripts/install-fish-tools.fish install [--target <dir>] [--dry-run] [--force]"
    echo "  scripts/install-fish-tools.fish uninstall [--target <dir>] [--dry-run] [--force]"
    echo "  scripts/install-fish-tools.fish status [--target <dir>]"
    echo "  scripts/install-fish-tools.fish help"
    echo ""
    echo "Notes:"
    echo "  - Default target is $HOME, which installs commands under ~/.local/bin."
    echo "  - Stow is preferred when available; direct-copy mode is fallback only."
end

function __ift_is_managed --argument-names path
    if not test -f "$path"
        return 1
    end
    grep -q "$__ift_marker" "$path"
end

function __ift_write_state --argument-names mode target
    mkdir -p "$__ift_state_dir"; or return 1
    printf 'set -gx ATB_REPO_ROOT %s\n' (string escape -- "$__ift_repo_root") > "$__ift_state_file"
    printf 'set -gx ATB_INSTALL_MODE %s\n' (string escape -- "$mode") >> "$__ift_state_file"
    printf 'set -gx ATB_TARGET %s\n' (string escape -- "$target") >> "$__ift_state_file"
end

function __ift_managed_paths --argument-names target
    set -l out
    for cmd in $__ift_bin_cmds
        set out $out "$target/.local/bin/$cmd"
    end
    printf '%s\n' $out
end

function __ift_check_sources
    if not test -d "$__ift_pkg_dir/.local/bin"
        echo "Missing stow package bin dir: $__ift_pkg_dir/.local/bin" 1>&2
        return 1
    end

    if not test -f "$__ift_pkg_dir/.local/lib/agentic_tactical_boots/dispatch.fish"
        echo "Missing dispatch helper: $__ift_pkg_dir/.local/lib/agentic_tactical_boots/dispatch.fish" 1>&2
        return 1
    end

    for cmd in $__ift_bin_cmds
        if not test -f "$__ift_pkg_dir/.local/bin/$cmd"
            echo "Missing wrapper in stow package: $__ift_pkg_dir/.local/bin/$cmd" 1>&2
            return 1
        end
    end
end

function __ift_install_direct --argument-names target dry_run force
    __ift_check_sources; or return 1

    set -l conflicts 0
    for path in (__ift_managed_paths "$target")
        if test -e "$path"
            if test "$force" = "true"
                continue
            end
            if not __ift_is_managed "$path"
                echo "Conflict (not managed): $path" 1>&2
                set conflicts 1
            end
        end
    end

    if test "$conflicts" -eq 1
        echo "Use --force to replace conflicting files." 1>&2
        return 1
    end

    if test "$dry_run" = "true"
        echo "[dry-run] Would ensure directory: $target/.local/bin"
    else
        mkdir -p "$target/.local/bin"; or return 1
    end

    for path in (__ift_managed_paths "$target")
        if test -e "$path"
            if test "$dry_run" = "true"
                echo "[dry-run] Would remove existing managed file: $path"
            else
                rm -f "$path"; or return 1
            end
        end
    end

    for cmd in $__ift_bin_cmds
        set -l src "$__ift_pkg_dir/.local/bin/$cmd"
        set -l dst "$target/.local/bin/$cmd"
        if test "$dry_run" = "true"
            echo "[dry-run] Would copy: $src -> $dst"
        else
            cp "$src" "$dst"; or return 1
            chmod +x "$dst"; or return 1
            echo "Installed wrapper: $dst"
        end
    end

    if test "$dry_run" = "true"
        echo "[dry-run] Would write state file: $__ift_state_file (mode=direct target=$target)"
    else
        __ift_write_state "direct" "$target"; or return 1
    end
end

function __ift_remove_direct --argument-names target dry_run force
    for path in (__ift_managed_paths "$target")
        if not test -e "$path"
            continue
        end

        if __ift_is_managed "$path"; or test "$force" = "true"
            if test "$dry_run" = "true"
                echo "[dry-run] Would remove: $path"
            else
                rm -f "$path"; or return 1
                echo "Removed: $path"
            end
        else
            echo "Skipping non-managed file: $path" 1>&2
        end
    end

end

function __ift_install_stow --argument-names target dry_run
    if not command -q stow
        echo "stow is not installed; cannot use stow mode." 1>&2
        return 1
    end

    __ift_check_sources; or return 1

    if test "$dry_run" = "true"
        echo "[dry-run] Would ensure directory: $target"
    else
        mkdir -p "$target"; or return 1
    end

    if test "$dry_run" = "true"
        __ift_remove_direct "$target" "$dry_run" "false"; or return 1
        echo "[dry-run] Would run: stow --restow --target $target --dir $__ift_repo_root/stow fish-tools"
        echo "[dry-run] Would write state file: $__ift_state_file (mode=stow target=$target)"
        return 0
    end

    __ift_remove_direct "$target" "false" "false"; or return 1
    stow --restow --target "$target" --dir "$__ift_repo_root/stow" fish-tools; or return 1
    __ift_write_state "stow" "$target"; or return 1
    echo "Installed via stow at target: $target"
end

function __ift_uninstall_stow --argument-names target dry_run
    if not command -q stow
        if test "$dry_run" = "true"
            echo "[dry-run] stow not installed; would skip stow uninstall and remove direct wrappers only."
            return 0
        end
        return 0
    end

    if test "$dry_run" = "true"
        echo "[dry-run] Would run: stow -D --target $target --dir $__ift_repo_root/stow fish-tools"
    else
        stow -D --target "$target" --dir "$__ift_repo_root/stow" fish-tools
    end
end

function __ift_print_status --argument-names target
    echo "Repo root: $__ift_repo_root"
    echo "Target: $target"

    if command -q stow
        echo "Stow: available"
    else
        echo "Stow: not found"
    end

    if test -f "$__ift_state_file"
        source "$__ift_state_file"
        echo "State: mode=$ATB_INSTALL_MODE target=$ATB_TARGET"
    else
        echo "State: not initialized"
    end

    for cmd in $__ift_bin_cmds
        set -l path "$target/.local/bin/$cmd"
        if test -L "$path"
            echo "$cmd: symlink ($path)"
        else if test -f "$path"
            if __ift_is_managed "$path"
                echo "$cmd: managed file ($path)"
            else
                echo "$cmd: non-managed file ($path)"
            end
        else
            echo "$cmd: missing"
        end
    end
end

set -l action "install"
set -l target "$__ift_default_target"
set -l dry_run "false"
set -l force "false"

set -l i 1
while test $i -le (count $argv)
    set -l arg "$argv[$i]"
    switch "$arg"
        case install uninstall status help --help -h
            set action "$arg"
        case --dry-run
            set dry_run "true"
        case --force
            set force "true"
        case --target
            set -l next_i (math "$i + 1")
            if test $next_i -gt (count $argv)
                echo "--target requires a value" 1>&2
                exit 1
            end
            set target "$argv[$next_i]"
            set i (math "$i + 1")
        case --target=*
            set target (string replace -- '--target=' '' "$arg")
        case '*'
            echo "Unknown argument: $arg" 1>&2
            __ift_usage
            exit 1
    end
    set i (math "$i + 1")
end

if not string match -q -- '/*' "$target"
    echo "--target must be an absolute path" 1>&2
    exit 1
end

switch "$action"
    case help --help -h
        __ift_usage
        exit 0
    case status
        __ift_print_status "$target"
        exit 0
    case install
        if command -q stow
            __ift_install_stow "$target" "$dry_run"; or exit 1
        else
            __ift_install_direct "$target" "$dry_run" "$force"; or exit 1
            echo "Installed without stow (fallback mode)."
            echo "If stow is installed later, wrappers auto-migrate on first run."
        end
    case uninstall
        __ift_uninstall_stow "$target" "$dry_run"; or exit 1
        __ift_remove_direct "$target" "$dry_run" "$force"; or exit 1
        if test "$dry_run" != "true"; and test -f "$__ift_state_file"
            rm -f "$__ift_state_file"
        end
    case '*'
        echo "Unknown command: $action" 1>&2
        __ift_usage
        exit 1
end
