# managed-by: agentic_tactical_boots/install-fish-tools

complete -c brew-vm -f
complete -c brew-vm -n '__fish_use_subcommand' -a 'help create-base init run shell install verify-network copy-in copy-out destroy'
complete -c brew-vm -n '__fish_seen_subcommand_from run shell install' -l network-policy -xa 'strict-egress proxy-only off'
complete -c brew-vm -n '__fish_seen_subcommand_from verify-network' -l allow-url -r
complete -c brew-vm -n '__fish_seen_subcommand_from verify-network' -l block-url -r
