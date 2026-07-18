# 0118 — Working egress review controls and visible denial chrome

Status: implemented and accepted

SCOPE: repair the Emacs progressive-egress controls in raw Emacs and Evil/Doom; make successful or failed review actions visibly respond; place the cursor on an actionable denied-destination row; emphasize pending denied requests in live session header/mode-line chrome with theme-aware color plus literal text.

OFF-LIMITS: do not trigger review from agent traffic, auto-grant or auto-dismiss, broaden destinations, weaken proxy/runtime ACK rules, render request payloads or credentials, or make color the only signal. Session grants remain explicit and session-scoped; persistent rules remain a separate hash-checked preview and write.

## Root cause

The review and session-detail buffers derive from generic `safeslop-output-mode` and install action closures only in the ordinary buffer-local map. Evil normal-state maps have higher precedence, so the documented `a`, `k`, `A`, `g`, and detail `v` controls resolve to Evil editing/motion commands instead. The review renderer also leaves point on its title rather than an observation row, and quiet mutation calls provide no refresh or visible completion, making raw-Emacs actions appear inert too.

Reproduction on `c133d57` with the real local Evil build:

```text
point=1 observation-at-point=nil
a -> evil-append
k -> evil-previous-line
A -> evil-append-line
g -> <evil g-prefix map>
```

## Pinned behavior

1. Session detail, denied-egress review, and persistent-rule review use dedicated action modes/maps with named commands, and those exact commands resolve in raw Emacs and Evil/Doom.
2. A populated review opens on the first denied destination. Refresh preserves the selected destination when it remains and otherwise selects the first row.
3. `a` Allow now and `k` Keep denied show an in-progress/result response and refresh the same review buffer after success without popping another buffer. Allow-now reminds the operator to retry the original request. Late callbacks cannot update a reused/dead review buffer.
4. Live eligible terminals retain the literal pending count and `C-c C-v` route. A nonzero count says `REQUEST DENIED` for one or `REQUESTS DENIED` otherwise, with a theme-aware warning face in both header and mode line; zero remains legible but non-alarming.
5. Observation polling remains read-only/non-modal, and all destinations/payloads remain absent from terminal chrome.

## Tasks

- [x] T1 — Add RED ERT/UI-matrix coverage for real review/detail key resolution, initial row selection, action feedback/refresh, and denial faces.
- [x] T2 — Introduce dedicated review/detail maps and named action commands; refresh in place with stale-callback guards.
- [x] T3 — Add color-redundant pending-denial chrome and retain the terminal shortcut across supported terminal backends.
- [x] T4 — Synchronize `README.md`, `emacs/README.md`, and `skills/agent-sandbox-ops/SKILL.md`.
- [x] T5 — Run targeted ERT, `make test-emacs-ui-matrix`, `make check`, and `make build`.
