#!/usr/bin/env fish

# Install repo-versioned skills into local runtime directory.
# Why: keeping skills in-repo makes changes reviewable and reproducible, then
# this script handles local deployment ergonomically.

set -l repo_root (cd (dirname (status filename))/..; pwd)
set -l src_dir "$repo_root/skills"
set -l dst_dir "$HOME/.claude/skills"
set -l force "false"
set -l dry_run "false"

for arg in $argv
    switch "$arg"
        case --force
            set force "true"
        case --dry-run
            set dry_run "true"
        case --help -h help
            echo "Usage:"
            echo "  scripts/install-local-skills.fish [--force] [--dry-run]"
            echo ""
            echo "Copies repo skills from skills/ into ~/.claude/skills."
            exit 0
        case '*'
            echo "Unknown argument: $arg" 1>&2
            exit 1
    end
end

if not test -d "$src_dir"
    echo "Missing skills source directory: $src_dir" 1>&2
    exit 1
end

mkdir -p "$dst_dir"

for skill_path in $src_dir/*
    if not test -d "$skill_path"
        continue
    end

    set -l name (basename "$skill_path")
    set -l target "$dst_dir/$name"

    if test -e "$target"
        if test "$force" != "true"
            echo "Skipping existing skill: $name (use --force to replace)"
            continue
        end

        if test "$dry_run" = "true"
            echo "Would remove existing: $target"
        else
            rm -rf "$target"
        end
    end

    if test "$dry_run" = "true"
        echo "Would copy: $skill_path -> $target"
    else
        cp -R "$skill_path" "$target"
        echo "Installed skill: $name"
    end
end
