#!/usr/bin/env fish

# Purpose:
# - Provide an optional macOS local sandbox layer using sandbox-exec.
# - Keep this as defense-in-depth when full containers/VMs are not practical.
#
# Safety/model notes:
# - Default path scope is current working directory only.
# - Default network policy is strict-egress (deny outbound network in profile).
# - This is not equivalent to VM/container isolation.
#
# References:
# - sandbox-exec man page: https://www.manpagez.com/man/1/sandbox-exec/
# - Apple Platform Security: https://support.apple.com/guide/security/welcome/web

function __macos_sandbox_usage
    echo "Usage:"
    echo "  source scripts/macos-sandbox.fish"
    echo "  macos-sandbox run [--network-policy strict-egress|off] [--path-scope cwd|repo-root] [--repo-root-access] [--allow-read <path>] [--allow-write <path>] <command...>"
    echo "  macos-sandbox shell [--network-policy strict-egress|off] [--path-scope cwd|repo-root] [--repo-root-access] [--allow-read <path>] [--allow-write <path>]"
    echo "  macos-sandbox print-profile [--network-policy strict-egress|off] [--path-scope cwd|repo-root] [--repo-root-access] [--allow-read <path>] [--allow-write <path>]"
    echo "  macos-sandbox help"
    echo ""
    echo "Notes:"
    echo "  - Optional local layer only; prefer Docker/VM isolation for untrusted execution."
    echo "  - Default path scope is cwd. Use --repo-root-access for repo-root scope."
    echo "  - strict-egress denies outbound network in the local sandbox profile."
end

function __macos_sandbox_require_support
    if not test (uname) = "Darwin"
        echo "macos-sandbox supports macOS only." 1>&2
        echo "Use: scripts/sandboxctl.fish docker ... or scripts/sandboxctl.fish brew-vm ..." 1>&2
        return 1
    end

    if not command -q sandbox-exec
        echo "sandbox-exec is not available on this system." 1>&2
        echo "Use: scripts/sandboxctl.fish docker ... or scripts/sandboxctl.fish brew-vm ..." 1>&2
        return 1
    end
end

function __macos_sandbox_validate_policy --argument-names policy
    if not contains -- "$policy" strict-egress off
        echo "Invalid --network-policy: $policy" 1>&2
        return 1
    end
end

function __macos_sandbox_validate_scope --argument-names scope
    if not contains -- "$scope" cwd repo-root
        echo "Invalid --path-scope: $scope" 1>&2
        return 1
    end
end

function __macos_sandbox_abs_path --argument-names candidate
    if string match -q -- '/*' "$candidate"
        echo "$candidate"
    else
        echo "$PWD/$candidate"
    end
end

function __macos_sandbox_escape_profile_path --argument-names raw_path
    set -l escaped (string replace -a '\\' '\\\\' -- "$raw_path")
    string replace -a '"' '\\"' -- "$escaped"
end

function __macos_sandbox_repo_root
    command git rev-parse --show-toplevel 2>/dev/null
end

function __macos_sandbox_build_profile --argument-names policy root_path
    set -l escaped_root (__macos_sandbox_escape_profile_path "$root_path")

    set -l profile
    set -a profile "(version 1)"
    set -a profile "(deny default)"
    set -a profile "(allow process-exec)"
    set -a profile "(allow process-fork)"
    set -a profile "(allow signal (target self))"
    set -a profile "(allow sysctl-read)"

    # Runtime/system reads are required for binaries, dynamic libs, and shell startup.
    for system_path in /System /usr /bin /sbin /Library /private/etc /etc /dev /var/db/timezone
        set -l escaped_system (__macos_sandbox_escape_profile_path "$system_path")
        set -a profile "(allow file-read* (subpath \"$escaped_system\"))"
    end

    set -a profile "(allow file-read* (literal \"/private/var/run/resolv.conf\"))"
    set -a profile "(allow file-read* (literal \"/private/var/run/utmpx\"))"

    set -a profile "(allow file-read* (subpath \"$escaped_root\"))"
    set -a profile "(allow file-write* (subpath \"$escaped_root\"))"

    # Commands and shells commonly need temporary directories even with tight path scope.
    for temp_path in /tmp /private/tmp /private/var/tmp
        set -l escaped_temp (__macos_sandbox_escape_profile_path "$temp_path")
        set -a profile "(allow file-read* (subpath \"$escaped_temp\"))"
        set -a profile "(allow file-write* (subpath \"$escaped_temp\"))"
    end

    for read_path in $__macos_sandbox_allow_read
        set -l abs_read (__macos_sandbox_abs_path "$read_path")
        set -l escaped_read (__macos_sandbox_escape_profile_path "$abs_read")
        set -a profile "(allow file-read* (subpath \"$escaped_read\"))"
    end

    for write_path in $__macos_sandbox_allow_write
        set -l abs_write (__macos_sandbox_abs_path "$write_path")
        set -l escaped_write (__macos_sandbox_escape_profile_path "$abs_write")
        set -a profile "(allow file-read* (subpath \"$escaped_write\"))"
        set -a profile "(allow file-write* (subpath \"$escaped_write\"))"
    end

    switch "$policy"
        case strict-egress
            set -a profile "(deny network*)"
        case off
            set -a profile "(allow network*)"
    end

    printf '%s\n' $profile
end

function __macos_sandbox_write_profile
    set -l profile_file (mktemp -t macos-sandbox.XXXXXX.sb)
    if test $status -ne 0
        echo "Failed to create temporary sandbox profile file" 1>&2
        return 1
    end

    printf '%s\n' $__macos_sandbox_profile_lines > "$profile_file"
    echo "$profile_file"
end

function __macos_sandbox_parse_options
    set -g __macos_sandbox_policy strict-egress
    set -g __macos_sandbox_scope cwd
    set -g __macos_sandbox_scope_set false
    set -g __macos_sandbox_repo_root_access false
    set -g __macos_sandbox_allow_read
    set -g __macos_sandbox_allow_write

    set -l i 1
    while test $i -le (count $argv)
        set -l arg "$argv[$i]"
        switch "$arg"
            case --network-policy
                set -l next_i (math "$i + 1")
                if test $next_i -gt (count $argv)
                    echo "--network-policy requires a value" 1>&2
                    return 1
                end
                set __macos_sandbox_policy "$argv[$next_i]"
                set i (math "$i + 2")
                continue
            case '--network-policy=*'
                set __macos_sandbox_policy (string replace -- '--network-policy=' '' "$arg")
                set i (math "$i + 1")
                continue
            case --path-scope
                set -l next_i (math "$i + 1")
                if test $next_i -gt (count $argv)
                    echo "--path-scope requires a value" 1>&2
                    return 1
                end
                set __macos_sandbox_scope "$argv[$next_i]"
                set __macos_sandbox_scope_set true
                set i (math "$i + 2")
                continue
            case '--path-scope=*'
                set __macos_sandbox_scope (string replace -- '--path-scope=' '' "$arg")
                set __macos_sandbox_scope_set true
                set i (math "$i + 1")
                continue
            case --repo-root-access
                set __macos_sandbox_repo_root_access true
                set i (math "$i + 1")
                continue
            case --allow-read
                set -l next_i (math "$i + 1")
                if test $next_i -gt (count $argv)
                    echo "--allow-read requires a value" 1>&2
                    return 1
                end
                set -a __macos_sandbox_allow_read "$argv[$next_i]"
                set i (math "$i + 2")
                continue
            case '--allow-read=*'
                set -a __macos_sandbox_allow_read (string replace -- '--allow-read=' '' "$arg")
                set i (math "$i + 1")
                continue
            case --allow-write
                set -l next_i (math "$i + 1")
                if test $next_i -gt (count $argv)
                    echo "--allow-write requires a value" 1>&2
                    return 1
                end
                set -a __macos_sandbox_allow_write "$argv[$next_i]"
                set i (math "$i + 2")
                continue
            case '--allow-write=*'
                set -a __macos_sandbox_allow_write (string replace -- '--allow-write=' '' "$arg")
                set i (math "$i + 1")
                continue
            case --
                set -g __macos_sandbox_remaining $argv[(math "$i + 1")..-1]
                return 0
            case '*'
                if string match -q -- '-*' "$arg"
                    echo "Unknown option: $arg" 1>&2
                    return 1
                end
                set -g __macos_sandbox_remaining $argv[$i..-1]
                return 0
        end
    end

    set -g __macos_sandbox_remaining
    return 0
end

function __macos_sandbox_prepare
    __macos_sandbox_validate_policy "$__macos_sandbox_policy"; or return 1

    if test "$__macos_sandbox_repo_root_access" = true
        if test "$__macos_sandbox_scope_set" = true; and test "$__macos_sandbox_scope" != repo-root
            echo "--repo-root-access conflicts with --path-scope $__macos_sandbox_scope" 1>&2
            return 1
        end
        set __macos_sandbox_scope repo-root
    end

    __macos_sandbox_validate_scope "$__macos_sandbox_scope"; or return 1

    set -l root_path "$PWD"
    if test "$__macos_sandbox_scope" = repo-root
        set root_path (__macos_sandbox_repo_root)
        if test -z "$root_path"
            echo "--path-scope repo-root requires running inside a git repository" 1>&2
            return 1
        end
    end

    set -g __macos_sandbox_root_path "$root_path"

    set -l profile_lines (__macos_sandbox_build_profile \
        "$__macos_sandbox_policy" \
        "$__macos_sandbox_root_path")

    if test $status -ne 0
        return 1
    end

    set -g __macos_sandbox_profile_lines $profile_lines
    set -g __macos_sandbox_profile_file (__macos_sandbox_write_profile)
    if test $status -ne 0
        return 1
    end
end

function __macos_sandbox_cleanup
    if set -q __macos_sandbox_profile_file; and test -n "$__macos_sandbox_profile_file"; and test -f "$__macos_sandbox_profile_file"
        rm -f "$__macos_sandbox_profile_file"
    end
end

function macos-sandbox --description "Run commands in optional macOS sandbox-exec profile"
    if test (count $argv) -eq 0
        __macos_sandbox_usage
        return 0
    end

    set -l cmd "$argv[1]"
    set -e argv[1]

    switch "$cmd"
        case help --help -h
            __macos_sandbox_usage
            return 0
    end

    __macos_sandbox_require_support; or return 1
    __macos_sandbox_parse_options $argv; or return 1
    __macos_sandbox_prepare; or return 1

    switch "$cmd"
        case run
            if test (count $__macos_sandbox_remaining) -eq 0
                echo "Usage: macos-sandbox run [options] <command...>" 1>&2
                __macos_sandbox_cleanup
                return 1
            end
            sandbox-exec -f "$__macos_sandbox_profile_file" -- $__macos_sandbox_remaining
            set -l cmd_status $status
            __macos_sandbox_cleanup
            return $cmd_status
        case shell
            sandbox-exec -f "$__macos_sandbox_profile_file" -- /bin/zsh -f
            set -l cmd_status $status
            __macos_sandbox_cleanup
            return $cmd_status
        case print-profile
            printf '%s\n' $__macos_sandbox_profile_lines
            __macos_sandbox_cleanup
            return 0
        case '*'
            echo "Unknown command: $cmd" 1>&2
            __macos_sandbox_usage
            __macos_sandbox_cleanup
            return 1
    end
end
