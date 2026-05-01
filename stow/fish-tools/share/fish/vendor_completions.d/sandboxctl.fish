# managed-by: agentic_tactical_boots/install-fish-tools

complete -c sandboxctl -f
complete -c sandboxctl -n '__fish_use_subcommand' -a 'help list tutorial docker docker-tools local brew-vm github forgejo radicle safe-npm safe-uv pinning'
complete -c sandboxctl -n '__fish_seen_subcommand_from tutorial' -a 'docker local brew-vm github-keys forgejo-keys radicle-access network-limiting file-sharing'
