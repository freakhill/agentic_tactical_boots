# managed-by: agentic_tactical_boots/install-fish-tools

complete -c agent-sandbox-tools -f
complete -c agent-sandbox-tools -n '__fish_use_subcommand' -a 'run shell up down tui help'
complete -c agent-sandbox-tools -n '__fish_seen_subcommand_from run shell up down' -l network-policy -xa 'strict-egress proxy-only off'
