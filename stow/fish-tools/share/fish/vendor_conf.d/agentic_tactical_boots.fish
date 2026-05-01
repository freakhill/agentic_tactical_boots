#!/usr/bin/env fish

# managed-by: agentic_tactical_boots/install-fish-tools

# Why: make shims available in all fish sessions without requiring users to edit
# config.fish manually after installation.
set -l atb_cfg "$HOME/.config/agentic_tactical_boots/fish-tools.env"
set -l atb_target "$HOME"

if test -f "$atb_cfg"
    source "$atb_cfg" 2>/dev/null
    if set -q ATB_TARGET; and test -n "$ATB_TARGET"
        set atb_target "$ATB_TARGET"
    end
end

set -l atb_bin "$atb_target/.local/bin"
if not contains -- "$atb_bin" $PATH
    set -gx PATH "$atb_bin" $PATH
end
