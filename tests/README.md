# Tests

Fish-native test harness for the scripts in this repo.

```fish
fish tests/run.fish
```

## Layout

- `run.fish` — discovers `test_*.fish` files and runs each in its own fish
  process so per-file globals never leak between files.
- `helpers.fish` — shared assertion library, cleanup hooks, and the
  `mk_tmpdir` helper. Sourced from each test file.
- `test_<script>.fish` — one file per `scripts/<script>.fish`.
- `test_py_helpers.fish` — direct tests of `scripts/_py/llm_*.py`,
  invoking each helper through `uv run --script`.

## Writing a test

```fish
#!/usr/bin/env fish

source (dirname (status filename))/helpers.fish

set -g SCRIPT "$SCRIPTS_DIR/<your-script>.fish"

function test_help_path
    set -l out (run_fish $SCRIPT --help 2>&1)
    set -l rc $status
    assert_status "<script> --help status" $rc 0
    assert_contains "<script> --help mentions Usage" "$out" "Usage:"
end

run_tests_in_file (basename (status filename))
```

`run_tests_in_file` discovers every function whose name starts with `test_`
and invokes it in declared order. Helpers in `helpers.fish` use names that
deliberately do **not** start with `test_` to avoid being picked up.

## Isolation rules

- **Always create temporary state with `mk_tmpdir`**, not bare `mktemp -d`.
  `mk_tmpdir` registers the path for cleanup at fish exit (the
  `--on-event fish_exit` handler in `helpers.fish` is fish's equivalent of
  `trap ... EXIT`). This means a tmpdir is removed even when an assertion
  fails or an exception aborts the test mid-way.
- **`run_tests_in_file` restores `$PWD` between tests**, so a test that
  cd's elsewhere cannot contaminate the next test.
- **Use `$FISH_BIN` to invoke fish from `env`-prefixed lines**. `env
  HOME=$tmp command fish ...` does **not** work on Linux because `command`
  is a fish builtin, not an external program — `env` cannot invoke it.

## Assertions

| Helper | What it checks |
|---|---|
| `assert_eq <name> <actual> <expected>` | string equality |
| `assert_status <name> <actual_rc> <expected_rc>` | exit-code equality |
| `assert_contains <name> <haystack> <needle>` | substring (haystack is usually `$out`) |
| `assert_not_contains <name> <haystack> <needle>` | absence of substring |

Assertions never call `exit`; they record pass/fail and the test continues.
The runner returns non-zero from a file iff at least one assertion failed.

## What the tests cover today

- `fish -n` syntax check for every fish script except the documented
  template (`scripts/script-template.fish`).
- `--help` / `help` paths for every script.
- Unknown-subcommand and missing-required-arg failure paths.
- Pure-logic, hermetic checks (`slop-pinning` against staged fixtures,
  `slop-safe-uv` validation, `slop-skills-install --dry-run`,
  `slop-install --target` validation, `slop-macos-sandbox` profile
  generation on Darwin / refusal on Linux, every Python helper subcommand).

What is **not** exercised: actual Docker / `tart` / `gh api` / `curl`
network calls. The wrappers around those are tested only at the
argv-parsing / pre-flight-validation level. Real integration testing
needs Docker / a macOS VM host / live API credentials and lives outside
this suite.
