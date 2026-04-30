#!/usr/bin/env fish

# Purpose:
# - Run Homebrew installs inside a disposable macOS VM instead of on host.
# - Keep network policy explicit (default strict egress through proxy).
# - Keep host/guest file transfer explicit (copy-in/copy-out) to avoid accidental
#   broad host exposure.
#
# Design notes:
# - "strict-egress" is default because package install/build paths are high risk.
# - No automatic host mounts: explicit transfer is easier to reason about/audit.
#
# References:
# - Tart docs: https://tart.run/
# - OpenSSH ssh/scp: https://man.openbsd.org/ssh and https://man.openbsd.org/scp
# - Homebrew install docs: https://docs.brew.sh/Installation

set -g BREW_VM_BASE_TEMPLATE "brew-sandbox-base"
set -g BREW_VM_SESSION_NAME "brew-sandbox-session"
set -g BREW_VM_SOURCE_IMAGE "ghcr.io/cirruslabs/macos-sonoma-base:latest"
set -g BREW_VM_SSH_USER "admin"
set -g BREW_VM_KEEP_SESSION "false"
set -g BREW_VM_BOOT_TIMEOUT 120
set -g BREW_VM_SSH_TIMEOUT 120
set -g BREW_VM_NETWORK_POLICY "strict-egress"
set -g BREW_VM_PROXY_URL ""
set -g BREW_VM_SHARE_DIR "/tmp/llm-share"

function __brew_vm_usage
    echo "Usage:"
    echo "  source scripts/brew-vm.fish"
    echo "  brew-vm help"
    echo "  brew-vm create-base"
    echo "  brew-vm init"
    echo "  brew-vm run [--network-policy strict-egress|proxy-only|off] <command...>"
    echo "  brew-vm shell [--network-policy strict-egress|proxy-only|off]"
    echo "  brew-vm install [--network-policy strict-egress|proxy-only|off] <formula>"
    echo "  brew-vm verify-network [--allow-url <url>] [--block-url <url>]"
    echo "  brew-vm copy-in <host-path> <guest-path>"
    echo "  brew-vm copy-out <guest-path> <host-path>"
    echo "  brew-vm destroy"
    echo ""
    echo "Notes:"
    echo "  - Host/guest files are NOT auto-shared. Use copy-in/copy-out explicitly."
    echo "  - Recommended guest share path: $BREW_VM_SHARE_DIR"
    echo "  - strict-egress/proxy-only require BREW_VM_PROXY_URL to be set."
end

# Keep boolean parsing centralized so policy toggles stay consistent across future
# edits and wrappers.
function __brew_vm_truthy --argument-names value
    switch (string lower -- "$value")
        case 1 true yes on
            return 0
    end
    return 1
end

# Only allow explicit policy values to avoid silent insecure fallback.
function __brew_vm_validate_policy --argument-names policy
    if not contains -- "$policy" strict-egress proxy-only off
        echo "Invalid --network-policy: $policy" 1>&2
        return 1
    end
end

function __brew_vm_exists --argument-names name
    tart list 2>/dev/null | string match -rq "(^|[[:space:]])$name([[:space:]]|\$)"
end

function __brew_vm_require_tools
    for tool in tart ssh scp
        if not command -sq "$tool"
            echo "Missing required tool: $tool" 1>&2
            return 1
        end
    end
end

# SSH opts are deliberately non-interactive so scripts fail fast in automation.
function __brew_vm_ssh_opts
    set -l opts \
        -o BatchMode=yes \
        -o ConnectTimeout=5 \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null

    if set -q BREW_VM_SSH_KEY
        set -a opts -i "$BREW_VM_SSH_KEY"
    end

    echo $opts
end

function __brew_vm_ip
    tart ip "$BREW_VM_SESSION_NAME" 2>/dev/null
end

function __brew_vm_proxy_prefix --argument-names policy
    # Why: proxy env var injection is the least invasive way to force package
    # tooling through a policy-enforcing egress path. We fail closed in strict
    # modes when proxy URL is missing.
    if test "$policy" = "off"
        echo ""
        return 0
    end

    if test -z "$BREW_VM_PROXY_URL"
        echo "BREW_VM_PROXY_URL is required for --network-policy $policy" 1>&2
        return 1
    end

    echo "export HTTP_PROXY='$BREW_VM_PROXY_URL' HTTPS_PROXY='$BREW_VM_PROXY_URL' http_proxy='$BREW_VM_PROXY_URL' https_proxy='$BREW_VM_PROXY_URL'; "
end

function __brew_vm_ssh
    set -l ip (__brew_vm_ip)
    if test -z "$ip"
        echo "VM is not running: $BREW_VM_SESSION_NAME" 1>&2
        return 1
    end

    set -l opts (__brew_vm_ssh_opts)
    ssh $opts "$BREW_VM_SSH_USER@$ip" -- $argv
end

function brew-vm-create-base --description "Create Tart base VM template for brew sandboxing"
    __brew_vm_require_tools; or return 1

    if __brew_vm_exists "$BREW_VM_BASE_TEMPLATE"
        echo "Base VM already exists: $BREW_VM_BASE_TEMPLATE"
        return 0
    end

    echo "Cloning base image into local template: $BREW_VM_BASE_TEMPLATE"
    tart clone "$BREW_VM_SOURCE_IMAGE" "$BREW_VM_BASE_TEMPLATE"
end

function brew-vm-init --description "Clone and boot disposable brew VM"
    # Why: clone from trusted base each session so potentially compromised
    # install state does not persist to host or future runs.
    __brew_vm_require_tools; or return 1

    if not __brew_vm_exists "$BREW_VM_BASE_TEMPLATE"
        echo "Missing base VM template: $BREW_VM_BASE_TEMPLATE" 1>&2
        echo "Run: brew-vm create-base" 1>&2
        return 1
    end

    if not __brew_vm_exists "$BREW_VM_SESSION_NAME"
        echo "Cloning disposable session VM: $BREW_VM_SESSION_NAME"
        tart clone "$BREW_VM_BASE_TEMPLATE" "$BREW_VM_SESSION_NAME"; or return 1
    end

    if test -z "(__brew_vm_ip)"
        echo "Booting VM: $BREW_VM_SESSION_NAME"
        tart run --no-graphics "$BREW_VM_SESSION_NAME" >/tmp/brew-vm-$BREW_VM_SESSION_NAME.log 2>&1 &
        disown
    end

    set -l boot_deadline (math (date +%s) + $BREW_VM_BOOT_TIMEOUT)
    set -l ip ""
    while true
        set ip (__brew_vm_ip)
        if test -n "$ip"
            break
        end
        if test (date +%s) -ge $boot_deadline
            echo "Timed out waiting for VM IP" 1>&2
            return 1
        end
        sleep 1
    end

    set -l opts (__brew_vm_ssh_opts)
    set -l ssh_deadline (math (date +%s) + $BREW_VM_SSH_TIMEOUT)
    while true
        if ssh $opts "$BREW_VM_SSH_USER@$ip" "true" >/dev/null 2>&1
            break
        end
        if test (date +%s) -ge $ssh_deadline
            echo "Timed out waiting for SSH on $ip as $BREW_VM_SSH_USER" 1>&2
            return 1
        end
        sleep 1
    end

    __brew_vm_ssh zsh -lc "command -v brew >/dev/null"
    if test $status -ne 0
        echo "Homebrew is not available in VM session. Install it in the base template first." 1>&2
        return 1
    end

    __brew_vm_ssh mkdir -p "$BREW_VM_SHARE_DIR" >/dev/null
    echo "VM ready: $BREW_VM_SESSION_NAME ($ip)"
end

function __brew_vm_run_with_policy --argument-names policy
    # Why: single execution path keeps policy handling and escaping consistent.
    __brew_vm_validate_policy "$policy"; or return 1
    brew-vm-init >/dev/null; or return 1

    if test (count $argv) -eq 1
        echo "Usage: brew-vm run <command...>" 1>&2
        return 1
    end

    set -e argv[1]
    set -l escaped (string escape -- $argv)
    set -l cmd (string join " " -- $escaped)
    set -l prefix (__brew_vm_proxy_prefix "$policy")
    __brew_vm_ssh zsh -lc "$prefix$cmd"
end

function brew-vm-run --description "Run command inside disposable brew VM"
    __brew_vm_run_with_policy "$BREW_VM_NETWORK_POLICY" run $argv
end

function brew-vm-shell --description "Open interactive shell inside disposable brew VM"
    set -l policy "$BREW_VM_NETWORK_POLICY"
    if test (count $argv) -ge 2; and test "$argv[1]" = "--network-policy"
        set policy "$argv[2]"
    end

    __brew_vm_validate_policy "$policy"; or return 1
    brew-vm-init >/dev/null; or return 1
    set -l prefix (__brew_vm_proxy_prefix "$policy")
    __brew_vm_ssh zsh -lc "$prefix exec zsh -l"
end

function brew-vm-install --description "Audit and install formula in disposable brew VM"
    set -l policy "$BREW_VM_NETWORK_POLICY"
    if test (count $argv) -ge 2; and test "$argv[1]" = "--network-policy"
        set policy "$argv[2]"
        set -e argv[1..2]
    end

    if test (count $argv) -ne 1
        echo "Usage: brew-vm install [--network-policy strict-egress|proxy-only|off] <formula>" 1>&2
        return 1
    end

    set -l formula "$argv[1]"
    brew-vm-destroy >/dev/null 2>&1
    brew-vm-init; or return 1

    echo "[1/3] Reviewing formula metadata"
    __brew_vm_run_with_policy "$policy" run brew info "$formula"; or return 1

    echo "[2/3] Dry-run install"
    __brew_vm_run_with_policy "$policy" run brew install --dry-run "$formula"; or return 1

    echo "[3/3] Installing in disposable VM"
    __brew_vm_run_with_policy "$policy" run brew install "$formula"; or return 1

    if __brew_vm_truthy "$BREW_VM_KEEP_SESSION"
        echo "Keeping VM for inspection: $BREW_VM_SESSION_NAME"
    else
        echo "Destroying disposable VM session"
        brew-vm-destroy
    end
end

function brew-vm-copy-in --description "Copy a host file/dir into VM"
    # Why: explicit transfer boundary is easier to audit than broad mounts.
    if test (count $argv) -ne 2
        echo "Usage: brew-vm copy-in <host-path> <guest-path>" 1>&2
        return 1
    end

    set -l src "$argv[1]"
    set -l dst "$argv[2]"
    if not test -e "$src"
        echo "Host path does not exist: $src" 1>&2
        return 1
    end

    brew-vm-init >/dev/null; or return 1
    set -l ip (__brew_vm_ip)
    set -l opts (__brew_vm_ssh_opts)
    scp $opts -r "$src" "$BREW_VM_SSH_USER@$ip:$dst"
end

function brew-vm-copy-out --description "Copy a VM file/dir to host"
    # Why: explicit host writes reduce accidental data leakage from guest.
    if test (count $argv) -ne 2
        echo "Usage: brew-vm copy-out <guest-path> <host-path>" 1>&2
        return 1
    end

    set -l src "$argv[1]"
    set -l dst "$argv[2]"

    brew-vm-init >/dev/null; or return 1
    set -l ip (__brew_vm_ip)
    set -l opts (__brew_vm_ssh_opts)
    scp $opts -r "$BREW_VM_SSH_USER@$ip:$src" "$dst"
end

function brew-vm-verify-network --description "Verify allowed and blocked network behavior"
    # Quick policy check: one expected-allow URL + one expected-block URL.
    set -l allow_url "https://registry.npmjs.org"
    set -l block_url "https://example.com"

    while test (count $argv) -gt 0
        switch "$argv[1]"
            case --allow-url
                set allow_url "$argv[2]"
                set -e argv[1..2]
            case --block-url
                set block_url "$argv[2]"
                set -e argv[1..2]
            case '*'
                echo "Unknown argument: $argv[1]" 1>&2
                return 1
        end
    end

    echo "Checking allowlisted URL: $allow_url"
    __brew_vm_run_with_policy "$BREW_VM_NETWORK_POLICY" run curl -I "$allow_url"; or return 1
    echo "Checking blocked URL: $block_url"
    __brew_vm_run_with_policy "$BREW_VM_NETWORK_POLICY" run sh -lc "curl -I '$block_url' >/dev/null 2>&1 && exit 1 || exit 0"; or return 1
    echo "Network verification passed for current policy: $BREW_VM_NETWORK_POLICY"
end

function brew-vm-destroy --description "Stop and delete disposable brew VM"
    __brew_vm_require_tools; or return 1

    if not __brew_vm_exists "$BREW_VM_SESSION_NAME"
        echo "No VM session to destroy: $BREW_VM_SESSION_NAME"
        return 0
    end

    tart stop "$BREW_VM_SESSION_NAME" >/dev/null 2>&1
    tart delete "$BREW_VM_SESSION_NAME"
end

function brew-vm --description "Unified wrapper for brew VM sandbox operations"
    if test (count $argv) -eq 0
        __brew_vm_usage
        return 0
    end

    set -l cmd "$argv[1]"
    set -e argv[1]

    switch "$cmd"
        case help --help -h
            __brew_vm_usage
        case create-base
            brew-vm-create-base
        case init
            brew-vm-init
        case run
            set -l policy "$BREW_VM_NETWORK_POLICY"
            if test (count $argv) -ge 2; and test "$argv[1]" = "--network-policy"
                set policy "$argv[2]"
                set -e argv[1..2]
            end
            __brew_vm_run_with_policy "$policy" run $argv
        case shell
            brew-vm-shell $argv
        case install
            brew-vm-install $argv
        case verify-network
            brew-vm-verify-network $argv
        case copy-in
            brew-vm-copy-in $argv
        case copy-out
            brew-vm-copy-out $argv
        case destroy
            brew-vm-destroy
        case '*'
            echo "Unknown command: $cmd" 1>&2
            __brew_vm_usage
            return 1
    end
end
