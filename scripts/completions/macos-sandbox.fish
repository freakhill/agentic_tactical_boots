# managed-by: agentic_tactical_boots/install-fish-tools

complete -c macos-sandbox -f
complete -c macos-sandbox -n '__fish_use_subcommand' -a 'help run shell print-profile'
complete -c macos-sandbox -n '__fish_seen_subcommand_from run shell print-profile' -l network-policy -xa 'strict-egress off'
complete -c macos-sandbox -n '__fish_seen_subcommand_from run shell print-profile' -l path-scope -xa 'cwd repo-root'
complete -c macos-sandbox -n '__fish_seen_subcommand_from run shell print-profile' -l repo-root-access
complete -c macos-sandbox -n '__fish_seen_subcommand_from run shell print-profile' -l allow-read -r
complete -c macos-sandbox -n '__fish_seen_subcommand_from run shell print-profile' -l allow-write -r
