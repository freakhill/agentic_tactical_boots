;;; safeslop-doom.el --- Optional Doom Emacs integration for safeslop -*- lexical-binding: t; -*-

;; Package-Requires: ((emacs "32.0"))

;;; Commentary:

;; Optional Doom Emacs sugar for safeslop.  This file does not depend on Doom:
;; it loads in raw Emacs and only binds Doom leader keys when `map!' exists.
;;
;; Evil bindings are data-driven (specs/0062): one shared table for the keys
;; every safeslop buffer offers, plus a per-mode table of surface actions.
;; safeslop's dashboards and output buffers are read-only single-key UIs, so
;; each mode enters Evil normal state and gets its keys through Evil (Evil does
;; not consult the keymap parent, and normal state would otherwise interpret
;; the action keys as motions/edits).  The four hand-maintained binding blocks
;; this replaces drifted more than once — edit the tables, not copies.
;;
;; Motion discipline (specs/0063 F1): dashboard tables do not bind keys Evil
;; users read as motions/searches — j k g n f a stay free; refresh rides `gr'
;; and the portal auto-refresh toggle `ga'. Dedicated egress-review modes are
;; the deliberate exception: their literal, safety-relevant `a'/`k'/`A'/`g'
;; controls must match the in-buffer legend in every editor state.

;;; Code:

;;;###autoload (autoload 'safeslop-portal "safeslop" nil t)
;;;###autoload (autoload 'safeslop-debug-log "safeslop" nil t)
;;;###autoload (autoload 'safeslop-doctor "safeslop" nil t)
;;;###autoload (autoload 'safeslop-policy-check-file "safeslop" nil t)
;;;###autoload (autoload 'safeslop-session-new "safeslop" nil t)
;;;###autoload (autoload 'safeslop-session-attach "safeslop" nil t)
;;;###autoload (autoload 'safeslop-session-list "safeslop" nil t)
;;;###autoload (autoload 'safeslop-session-status "safeslop" nil t)
;;;###autoload (autoload 'safeslop-session-stop "safeslop" nil t)
;;;###autoload (autoload 'safeslop-session-reattach "safeslop" nil t)
;;;###autoload (autoload 'safeslop-switch-to-session-buffer "safeslop" nil t)
;;;###autoload (autoload 'safeslop-show-last-error "safeslop" nil t)
;;;###autoload (autoload 'safeslop-help "safeslop" nil t)
;;;###autoload (autoload 'safeslop-profiles "safeslop" nil t)
;;;###autoload (autoload 'safeslop-credentials "safeslop" nil t)

(require 'safeslop)

(declare-function evil-set-initial-state "ext:evil-states" (mode state))
(declare-function evil-define-key* "ext:evil-core" (state keymap &rest bindings))

(defconst safeslop-doom--evil-shared-keys
  '(("d" . safeslop-doctor)
    ("E" . safeslop-show-last-error)
    ("L" . safeslop-debug-log)
    ("?" . describe-mode)
    ("q" . quit-window)
    ;; Shared surface switch keys (Evil does not consult the keymap parent).
    ("P" . safeslop-portal)
    ("F" . safeslop-profiles)
    ("K" . safeslop-credentials)
    ("[" . safeslop-surface-prev)
    ("]" . safeslop-surface-next)
    ("TAB" . safeslop-surface-next)
    ("<backtab>" . safeslop-surface-prev))
  "Keys every safeslop buffer offers in Evil normal state, as (KEY . COMMAND).
Applied to each mode in `safeslop-doom--evil-mode-keys' before its own keys,
so mode-specific actions may override shared help/quit where needed.")

(defconst safeslop-doom--evil-mode-keys
  '((safeslop-output-mode safeslop-output-mode-map
     ("gr" . safeslop-output-refresh)
     ("e" . safeslop-show-last-error))
    (safeslop-session-detail-mode safeslop-session-detail-mode-map
     ("v"  . safeslop-session-detail-egress-review)
     ("o"  . safeslop-session-detail-egress-observations)
     ("G"  . safeslop-session-detail-egress-grants)
     ("+"  . safeslop-session-detail-egress-grant)
     ("-"  . safeslop-session-detail-egress-revoke)
     ("gr" . safeslop-output-refresh))
    (safeslop-egress-review-mode safeslop-egress-review-mode-map
     ("a"         . safeslop-egress-review-allow-now)
     ("k"         . safeslop-egress-review-keep-denied)
     ("A"         . safeslop-egress-review-always-allow)
     ("g"         . safeslop-egress-review-refresh)
     ("TAB"       . safeslop-egress-review-next)
     ("<backtab>" . safeslop-egress-review-previous)
     ("q"         . quit-window))
    (safeslop-profile-egress-review-mode safeslop-profile-egress-review-mode-map
     ("a" . safeslop-profile-egress-review-add)
     ("q" . quit-window))
    (safeslop-portal-mode safeslop-portal-mode-map
     ("RET" . safeslop-portal-open)
     ("o"   . safeslop-portal-open)
     ("r"   . safeslop-portal-run)
     ("R"   . safeslop-portal-run-detached)
     ("A"   . safeslop-portal-reattach)
     ("i"   . safeslop-portal-status)
     ("s"   . safeslop-portal-stop)
     ("x"   . safeslop-portal-remove)
     ("X"   . safeslop-portal-prune)
     ("c"   . safeslop-portal-new)
     ("N"   . safeslop-portal-rename)
     ("^"   . safeslop-portal-follow-profile)
     ("gr"  . safeslop-portal-refresh)
     ("ga"  . safeslop-portal-toggle-auto-refresh))
    (safeslop-profiles-mode safeslop-profiles-mode-map
     ("RET" . safeslop-profiles-inspect)
     ("i"   . safeslop-profiles-inspect)
     ("r"   . safeslop-profiles-launch)
     ("e"   . safeslop-profiles-edit)
     ("c"   . safeslop-profiles-create)
     ("C"   . safeslop-profiles-clone)
     ("v"   . safeslop-profiles-validate)
     ("D"   . safeslop-profiles-delete)
     ("gr"  . safeslop-profiles-refresh))
    (safeslop-profiles-compose-mode safeslop-profiles-compose-mode-map
     ("RET"     . safeslop-profiles-compose-toggle)
     ("?"       . safeslop-profiles-compose-help)
     ("gr"      . safeslop-profiles-compose-refresh)
     ("C-c C-c" . safeslop-profiles-compose-preview-save)
     ("q"       . safeslop-profiles-compose-cancel))
    (safeslop-credentials-mode safeslop-credentials-mode-map
     ("RET" . safeslop-credentials-inspect)
     ("i"   . safeslop-credentials-inspect)
     ("A"   . safeslop-credentials-link-account)
     ("U"   . safeslop-credentials-unlink-account)
     ("R"   . safeslop-credentials-pick-repositories)
     ("X"   . safeslop-credentials-clear-profile-forge)
     ("e"   . safeslop-credentials-edit)
     ("gr"  . safeslop-credentials-refresh))
    (safeslop-credentials-inspect-mode safeslop-credentials-inspect-mode-map
     ("RET" . safeslop-credentials-inspect-back)
     ("e"   . safeslop-credentials-inspect-edit)
     ("gr"  . safeslop-credentials-inspect-refresh)
     ("q"   . quit-window)))
  "Per-mode Evil normal-state actions: (MODE MAP-SYMBOL (KEY . COMMAND)...).
Every listed mode also receives `safeslop-doom--evil-shared-keys'. Dashboard
modes avoid Evil motion keys (specs/0063 F1); the dedicated egress-review modes
intentionally override their documented action keys so safety controls do not
silently resolve to editing commands. `gr'/`ga' carry ordinary dashboard
refresh and the portal auto-refresh toggle.")

(with-eval-after-load 'evil
  (dolist (entry safeslop-doom--evil-mode-keys)
    (let ((mode (nth 0 entry))
          (map (symbol-value (nth 1 entry))))
      ;; Read-only single-key UIs: make normal state explicit, then install the
      ;; Shared keys are installed first so a mode can deliberately override
      ;; help/quit semantics (the compose buffer's `?' and `q' are row/help and
      ;; cancel, not generic describe/quit).
      (evil-set-initial-state mode 'normal)
      (dolist (binding (append safeslop-doom--evil-shared-keys (cddr entry)))
        (evil-define-key* 'normal map (kbd (car binding)) (cdr binding))))))

;;;###autoload
(defun safeslop-doom-bind-leader ()
  "Bind `safeslop-command-map' under Doom's leader at SPC o s when Doom is present.
A no-op outside Doom Emacs.  Call this from Doom `config.el'.

This deliberately takes over SPC o s, which Doom's `:os macos' module otherwise
binds to its \"send to application\" prefix (Transmit/Launchbar/iTerm) — a chosen
override, not an accident; keep it on s (slopmaxx sits at SPC o m).  Rebind here
if you want the macOS prefix back."
  (interactive)
  (when (fboundp 'map!)
    (eval '(map! :leader
                 (:prefix ("o" . "open")
                  :desc "safeslop" "s" #'safeslop-command-map))
          t)
    t))

(provide 'safeslop-doom)
;;; safeslop-doom.el ends here
