#!/usr/bin/env fish

# Install repo-versioned skills into local runtime directory.
# Why: keeping skills in-repo makes changes reviewable and reproducible, then
# this script handles local deployment ergonomically.

set -l repo_root (cd (dirname (status filename))/..; pwd)
set -l src_dir "$repo_root/skills"
set -l dst_dir "$HOME/.claude/skills"
set -l force "false"
set -l dry_run "false"

function __install_skills_help
    echo "install-local-skills — copy repo skills into ~/.claude/skills"
    echo ""
    echo "Description:"
    echo "  Copies every immediate subdirectory of skills/ in this repo into"
    echo "  ~/.claude/skills. Skills that already exist at the destination are"
    echo "  skipped unless --force is given. Use --dry-run to preview without"
    echo "  writing."
    echo ""
    echo "Usage:"
    echo "  scripts/install-local-skills.fish [--force] [--dry-run]"
    echo "  scripts/install-local-skills.fish help"
    echo ""
    echo "Options:"
    echo "  --force       Replace existing skills at the destination."
    echo "  --dry-run     Print what would happen without writing anything."
    echo ""
    echo "Examples:"
    echo "  # Preview"
    echo "  scripts/install-local-skills.fish --dry-run"
    echo ""
    echo "  # Install (skip existing)"
    echo "  scripts/install-local-skills.fish"
    echo ""
    echo "  # Replace existing skills with the in-repo versions"
    echo "  scripts/install-local-skills.fish --force"
    echo ""
    echo "Notes:"
    echo "  - Source: skills/<name>/ in this repo."
    echo "  - Target: ~/.claude/skills/<name>/."
    echo "  - To uninstall a skill, remove its directory from ~/.claude/skills."
end

function __install_skills_help_to_stderr
    __install_skills_help 1>&2
end

for arg in $argv
    switch "$arg"
        case --force
            set force "true"
        case --dry-run
            set dry_run "true"
        case --help -h help
            __install_skills_help
            exit 0
        case '*'
            echo "Error: Unknown argument: $arg" 1>&2
            echo "" 1>&2
            __install_skills_help_to_stderr
            exit 1
    end
end

if not test -d "$src_dir"
    echo "Error: Missing skills source directory: $src_dir" 1>&2
    echo "" 1>&2
    __install_skills_help_to_stderr
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
