# managed-by: agentic_tactical_boots/install-fish-tools

complete -c llm-radicle-access -f
complete -c llm-radicle-access -n '__fish_use_subcommand' -a 'create-identity bootstrap-config list-identities retire-identity retire-expired bind-repo list-bindings unbind-repo print-env help'
complete -c llm-radicle-access -n '__fish_seen_subcommand_from create-identity' -l name -r
complete -c llm-radicle-access -n '__fish_seen_subcommand_from create-identity' -l ttl -r
complete -c llm-radicle-access -n '__fish_seen_subcommand_from retire-identity bind-repo unbind-repo print-env' -l id -r
complete -c llm-radicle-access -n '__fish_seen_subcommand_from retire-identity bind-repo unbind-repo print-env' -l identity-id -r
complete -c llm-radicle-access -n '__fish_seen_subcommand_from bind-repo list-bindings unbind-repo' -l rid -r
complete -c llm-radicle-access -n '__fish_seen_subcommand_from bind-repo' -l access -xa 'ro rw'
complete -c llm-radicle-access -n '__fish_seen_subcommand_from bind-repo' -l note -r
complete -c llm-radicle-access -n '__fish_seen_subcommand_from list-identities list-bindings' -l all
complete -c llm-radicle-access -n '__fish_seen_subcommand_from retire-identity retire-expired unbind-repo' -l yes
complete -c llm-radicle-access -n '__fish_seen_subcommand_from bootstrap-config' -l force
