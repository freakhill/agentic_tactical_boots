;;; safeslop-egress.el --- Progressive session egress UI -*- lexical-binding: t; -*-

;; Copyright (C) 2026

;; Author: safeslop
;; Package-Requires: ((emacs "32.0"))
;; Keywords: tools, processes, ai

;;; Commentary:

;; Explicit session-scoped egress command construction and passive review UI.
;; `safeslop-session.el' remains the public session front.

;;; Code:

(require 'subr-x)
(require 'cl-lib)
(require 'safeslop-contract)
(require 'safeslop-client)
(require 'safeslop-surface)
(require 'safeslop-output)
(require 'safeslop-session-terminal)

(declare-function safeslop-session-egress-grant "safeslop-session"
                  (&optional session-id host port callback quiet))
(declare-function safeslop-session-egress-dismiss "safeslop-session"
                  (&optional session-id host port callback quiet))
(declare-function safeslop-session-egress-observations "safeslop-session"
                  (&optional session-id callback quiet))
(declare-function safeslop-session-egress-review "safeslop-session"
                  (&optional session-id session-data))

;; specs/0097: Egress observation is strictly read-only.  The three mutation
;; functions below are explicit operator commands; no agent/proxy event calls
;; them, and none edits safeslop.cue or profile egress policy.
(defun safeslop-session--egress-observations-args (session-id)
  "Return exact argv for SESSION-ID's value-free denied observations."
  (list "session" "egress" "observations" "--session-id" session-id "--output" "json"))

(defun safeslop-session--egress-grants-args (session-id)
  "Return exact argv for SESSION-ID's active session-scoped grants."
  (list "session" "egress" "grants" "--session-id" session-id "--output" "json"))

(defun safeslop-session--egress-grant-args (session-id host port)
  "Return exact argv to grant HOST:PORT for SESSION-ID."
  (list "session" "egress" "grant" "--session-id" session-id "--host" host
        "--port" (number-to-string port) "--output" "json"))

(defun safeslop-session--egress-revoke-args (session-id grant-id)
  "Return exact argv to revoke GRANT-ID from SESSION-ID."
  (list "session" "egress" "revoke" "--session-id" session-id "--grant-id" grant-id
        "--output" "json"))

(defun safeslop-session--egress-dismiss-args (session-id host port)
  "Return exact argv to keep HOST:PORT denied for SESSION-ID."
  (list "session" "egress" "dismiss" "--session-id" session-id "--host" host
        "--port" (number-to-string port) "--output" "json"))

(defun safeslop-session--profile-egress-args (operation profile policy-path host port policy-hash)
  "Return exact argv for explicit durable OPERATION on PROFILE's typed rule.
POLICY-HASH is always supplied from the session snapshot so a changed policy
fails closed instead of silently editing newer reviewed bytes."
  (append (list "profile" "egress" operation profile)
          (when (and (stringp policy-path) (not (string-prefix-p "builtin:" policy-path)))
            (list policy-path))
          (list "--host" host "--port" (number-to-string port)
                "--expected-policy-hash" policy-hash "--output" "json")))

(defun safeslop-session--egress-dispatch (args buffer-name callback quiet)
  "Dispatch egress ARGS asynchronously, rendering BUFFER-NAME unless QUIET.
CALLBACK receives the JSON envelope.  This is deliberately a thin explicit CLI
bridge: it never consults or writes a profile policy."
  (safeslop--call-json-async
   args
   (lambda (envelope)
     (unless quiet
       (safeslop--show-envelope-buffer buffer-name args envelope))
     (when callback (funcall callback envelope)))))

(defvar-local safeslop-egress-review-session-id nil
  "Session id addressed by the current denied-egress review buffer.")

(defvar-local safeslop-egress-review-session-data nil
  "Value-free session snapshot used by the current denied-egress review.")

(defvar-local safeslop-egress-review-status "Ready."
  "Operator-visible status rendered in the current denied-egress review.")

(defvar-local safeslop-egress-review-status-face nil
  "Optional face for `safeslop-egress-review-status'.")

(defvar-local safeslop-egress-review-action-in-flight nil
  "Opaque token for the explicit review mutation currently in flight.")

(defvar-local safeslop-egress-review-request-token nil
  "Opaque token guarding asynchronous observation callbacks after buffer reuse.")

(defvar safeslop-egress-review-mode-map
  (let ((map (make-sparse-keymap)))
    (set-keymap-parent map safeslop-output-mode-map)
    (define-key map (kbd "a") #'safeslop-egress-review-allow-now)
    (define-key map (kbd "k") #'safeslop-egress-review-keep-denied)
    (define-key map (kbd "A") #'safeslop-egress-review-always-allow)
    (define-key map (kbd "g") #'safeslop-egress-review-refresh)
    (define-key map (kbd "TAB") #'safeslop-egress-review-next)
    (define-key map (kbd "<backtab>") #'safeslop-egress-review-previous)
    (define-key map (kbd "<down>") #'safeslop-egress-review-next)
    (define-key map (kbd "<up>") #'safeslop-egress-review-previous)
    (define-key map (kbd "q") #'quit-window)
    map)
  "Keymap for explicit denied-egress review buffers.")

(define-derived-mode safeslop-egress-review-mode safeslop-output-mode
  "safeslop-egress"
  "Major mode for explicit, value-free denied-egress review.")

(defun safeslop-session--review-observation-at-point ()
  "Return the value-free review observation on point's row, or signal clearly."
  (or (get-text-property (line-beginning-position) 'safeslop-egress-observation)
      (get-text-property (point) 'safeslop-egress-observation)
      (user-error "Move point to a denied destination")))

(defun safeslop-egress-review--observation-key (observation)
  "Return the stable value-free row key for OBSERVATION."
  (and observation
       (cons (alist-get 'host observation) (alist-get 'port observation))))

(defun safeslop-egress-review--status-line ()
  "Return the current review status as one faced line."
  (let ((text (format "Status: %s" (or safeslop-egress-review-status "Ready."))))
    (if safeslop-egress-review-status-face
        (propertize text 'face safeslop-egress-review-status-face)
      text)))

(defun safeslop-egress-review--set-status (status &optional face)
  "Set operator-visible STATUS and optional FACE without moving point."
  (setq safeslop-egress-review-status status
        safeslop-egress-review-status-face face)
  (let ((inhibit-read-only t))
    (save-excursion
      (goto-char (point-min))
      (when (re-search-forward "^Status:.*$" nil t)
        (let ((start (line-beginning-position))
              (end (line-end-position)))
          (delete-region start end)
          (goto-char start)
          (insert (safeslop-egress-review--status-line)))))))

(defun safeslop-session--open-review-buffer (name title loading &optional mode)
  "Open operator-requested NAME once, then return it for an async update.
LOADING is rendered before dispatch so a later proxy response cannot focus or
pop any window.  MODE defaults to `safeslop-output-mode'."
  (let ((buf (get-buffer-create name)))
    (with-current-buffer buf
      (funcall (or mode #'safeslop-output-mode))
      (let ((inhibit-read-only t))
        (erase-buffer)
        (insert title "\n" loading "\n")))
    (pop-to-buffer buf)
    buf))

(defun safeslop-session--review-render (session-id session-data envelope &optional buffer)
  "Render a value-free operator review into BUFFER without selecting it."
  (let ((buf (or buffer (get-buffer-create "*safeslop egress review*")))
        (observations (alist-get 'observations (safeslop-contract-data envelope))))
    (when (buffer-live-p buf)
      (with-current-buffer buf
        (unless (derived-mode-p 'safeslop-egress-review-mode)
          (safeslop-egress-review-mode))
        (setq safeslop-egress-review-session-id session-id
              safeslop-egress-review-session-data session-data)
        (let* ((inhibit-read-only t)
               (selected (get-text-property
                          (line-beginning-position) 'safeslop-egress-observation))
               (selected-key (safeslop-egress-review--observation-key selected))
               first-position selected-position)
          (erase-buffer)
          (insert (format "Progressive egress review — session %s\n" session-id))
          (insert "Passive observations are denied traffic, not prompts or authority.\n")
          (insert "Keys: a Allow now, k Keep denied, A Always allow, g refresh, TAB/S-TAB move, q quit\n")
          (insert (safeslop-egress-review--status-line) "\n\n")
          (if (not (safeslop-contract-ok-p envelope))
              (insert "Could not read observations; retry with g.\n")
            (if (null observations)
                (insert "No pending denied destinations.\n")
              (dolist (obs observations)
                (let ((start (point))
                      (host (safeslop-session--safe-display-field (alist-get 'host obs)))
                      (port (alist-get 'port obs)))
                  (unless first-position (setq first-position start))
                  (when (equal selected-key (safeslop-egress-review--observation-key obs))
                    (setq selected-position start))
                  ;; Deliberately render no request URI, header, or payload.
                  (insert (format "%s:%s  count=%s  last=%s  %s\n"
                                  (or host "[redacted]") (or port "?")
                                  (or (alist-get 'count obs) 0)
                                  (or (alist-get 'last_seen obs) "?")
                                  (if (eq (alist-get 'grantable obs) t)
                                      "grantable" "keep denied")))
                  (put-text-property start (point) 'safeslop-egress-observation obs)))))
          (goto-char (or selected-position first-position (point-min))))))))

(defun safeslop-egress-review--move (lines)
  "Move LINES until point reaches another denied-destination row."
  (let ((origin (point)) found)
    (forward-line lines)
    (while (and (not found)
                (if (> lines 0) (not (eobp)) (not (bobp))))
      (if (get-text-property (line-beginning-position) 'safeslop-egress-observation)
          (setq found t)
        (forward-line lines)))
    (if found
        (beginning-of-line)
      (goto-char origin)
      (user-error "No more denied destinations"))))

(defun safeslop-egress-review-next ()
  "Move to the next denied destination."
  (interactive)
  (safeslop-egress-review--move 1))

(defun safeslop-egress-review-previous ()
  "Move to the previous denied destination."
  (interactive)
  (safeslop-egress-review--move -1))

(defun safeslop-egress-review--request-observations ()
  "Refresh this review in place without selecting or popping its buffer."
  (unless (and (derived-mode-p 'safeslop-egress-review-mode)
               (stringp safeslop-egress-review-session-id))
    (user-error "This buffer has no denied-egress session"))
  (let* ((buffer (current-buffer))
         (session-id safeslop-egress-review-session-id)
         (session-data safeslop-egress-review-session-data)
         (token (list session-id)))
    (setq safeslop-egress-review-request-token token)
    (condition-case nil
        (safeslop-session-egress-observations
         session-id
         (lambda (envelope)
           (when (buffer-live-p buffer)
             (with-current-buffer buffer
               (when (and (equal session-id safeslop-egress-review-session-id)
                          (eq token safeslop-egress-review-request-token))
                 (safeslop-session--review-render
                  session-id session-data envelope buffer)))))
         t)
      (error
       (safeslop-egress-review--set-status
        "Observation refresh failed; press g to retry." 'error)))))

(defun safeslop-egress-review-refresh ()
  "Explicitly refresh denied destinations in the current review buffer."
  (interactive)
  (when safeslop-egress-review-action-in-flight
    (user-error "An egress action is still in progress"))
  (safeslop-egress-review--set-status "Refreshing denied destinations…" 'shadow)
  (safeslop-egress-review--request-observations))

(defun safeslop-egress-review--run-action (label success-message function)
  "Run explicit LABEL through FUNCTION, then display SUCCESS-MESSAGE and refresh."
  (when safeslop-egress-review-action-in-flight
    (user-error "An egress action is already in progress"))
  (let* ((observation (safeslop-session--review-observation-at-point))
         (session-id safeslop-egress-review-session-id)
         (host (alist-get 'host observation))
         (port (alist-get 'port observation))
         (buffer (current-buffer))
         (token (list session-id host port label)))
    (setq safeslop-egress-review-action-in-flight token)
    (safeslop-egress-review--set-status
     (format "%s in progress; waiting for runtime acknowledgement…" label)
     'warning)
    (condition-case nil
        (funcall
         function session-id host port
         (lambda (envelope)
           (when (buffer-live-p buffer)
             (with-current-buffer buffer
               (when (and (equal session-id safeslop-egress-review-session-id)
                          (eq token safeslop-egress-review-action-in-flight))
                 (setq safeslop-egress-review-action-in-flight nil)
                 (if (safeslop-contract-ok-p envelope)
                     (progn
                       (safeslop-egress-review--set-status success-message 'success)
                       (message "safeslop: %s" success-message)
                       (safeslop-egress-review--request-observations))
                   (let ((failure (format
                                   "%s failed; no network authority change was confirmed. Press g to retry."
                                   label)))
                     (safeslop-egress-review--set-status failure 'error)
                     (message "safeslop: %s" failure)))))))
         t)
      (error
       (setq safeslop-egress-review-action-in-flight nil)
       (safeslop-egress-review--set-status
        (format "%s failed before dispatch; no network authority changed." label)
        'error)))))

(defun safeslop-egress-review-allow-now ()
  "Explicitly allow the denied destination at point for this session."
  (interactive)
  (let ((observation (safeslop-session--review-observation-at-point)))
    (unless (eq (alist-get 'grantable observation) t)
      (user-error "This denied destination is not grantable")))
  (safeslop-egress-review--run-action
   "Allow now" "Allowed for this session; retry the original request."
   #'safeslop-session-egress-grant))

(defun safeslop-egress-review-keep-denied ()
  "Explicitly acknowledge the destination at point while keeping it denied."
  (interactive)
  (safeslop-egress-review--run-action
   "Keep denied" "Kept denied; no network authority changed."
   #'safeslop-session-egress-dismiss))

(defun safeslop-egress-review-always-allow ()
  "Open the separate hash-checked persistent-rule preview for point's row."
  (interactive)
  (let ((observation (safeslop-session--review-observation-at-point)))
    (unless (eq (alist-get 'grantable observation) t)
      (user-error "This denied destination is not grantable"))
    (safeslop-session--profile-egress-review
     safeslop-egress-review-session-data observation)))

(defvar-local safeslop-profile-egress-review-session-data nil
  "Session snapshot backing the current persistent-rule review.")

(defvar-local safeslop-profile-egress-review-observation nil
  "Denied destination backing the current persistent-rule review.")

(defvar-local safeslop-profile-egress-review-can-add nil
  "Non-nil only after a successful hash-checked persistent-rule preview.")

(defvar-local safeslop-profile-egress-review-token nil
  "Opaque token rejecting callbacks for a reused persistent-review buffer.")

(defvar safeslop-profile-egress-review-mode-map
  (let ((map (make-sparse-keymap)))
    (set-keymap-parent map safeslop-output-mode-map)
    (define-key map (kbd "a") #'safeslop-profile-egress-review-add)
    (define-key map (kbd "q") #'quit-window)
    map)
  "Keymap for the separate hash-checked persistent-rule review.")

(define-derived-mode safeslop-profile-egress-review-mode safeslop-output-mode
  "safeslop-profile-egress"
  "Major mode for the explicit persistent egress-rule preview and write.")

(defun safeslop-profile-egress-review-add ()
  "Write the exact persistent rule shown in the current preview."
  (interactive)
  (unless safeslop-profile-egress-review-can-add
    (user-error "No current hash-checked persistent rule is available to add"))
  (let ((session-data safeslop-profile-egress-review-session-data)
        (observation safeslop-profile-egress-review-observation)
        (token safeslop-profile-egress-review-token)
        (buffer (current-buffer)))
    (setq safeslop-profile-egress-review-can-add nil)
    (safeslop-session--egress-dispatch
     (safeslop-session--profile-egress-args
      "add" (alist-get 'profile session-data) (alist-get 'policy_path session-data)
      (alist-get 'host observation) (alist-get 'port observation)
      (alist-get 'policy_hash session-data))
     "*safeslop profile egress add*"
     (lambda (result)
       (when (buffer-live-p buffer)
         (with-current-buffer buffer
           (when (eq token safeslop-profile-egress-review-token)
             (if (safeslop-contract-ok-p result)
                 (let ((inhibit-read-only t))
                   (goto-char (point-max))
                   (insert (propertize
                            "\nPersistent rule written. Review and re-trust before creating a new session.\n"
                            'face 'success))
                   (message "safeslop: persistent rule written; review and re-trust before creating a new session"))
               (safeslop-session--profile-egress-render
                session-data observation result buffer))))))
     t)))

(defun safeslop-session--profile-egress-render (session-data observation envelope buffer)
  "Render the hash-checked persistent-rule review into BUFFER, never focusing it."
  (when (buffer-live-p buffer)
    (with-current-buffer buffer
      (unless (derived-mode-p 'safeslop-profile-egress-review-mode)
        (safeslop-profile-egress-review-mode))
      (setq safeslop-profile-egress-review-session-data session-data
            safeslop-profile-egress-review-observation observation
            safeslop-profile-egress-review-can-add (safeslop-contract-ok-p envelope))
      (let ((inhibit-read-only t)
            (data (safeslop-contract-data envelope)))
        (erase-buffer)
        (if (not (safeslop-contract-ok-p envelope))
            (insert "Policy changed; no persistent rule was written. Re-open review to inspect current policy.\n")
          (insert "Persistent egress review — future sessions only\n")
          (insert (format "Profile: %s\n" (or (alist-get 'profile data) "[redacted]")))
          (insert (format "Current policy hash: %s\n" (or (alist-get 'current_policy_hash data) "[redacted]")))
          (insert (format "Candidate policy hash: %s\n" (or (alist-get 'candidate_policy_hash data) "[redacted]")))
          (insert (format "Delta: + persistentEgress: {%s, %s}\n"
                          (or (safeslop-session--safe-display-field (alist-get 'host observation)) "[redacted]")
                          (or (alist-get 'port observation) "?")))
          (insert "Source/lifetime: profile-persistent / future sessions\n")
          (insert "\nPress a to add this exact rule, changing policy bytes; then review and re-trust before a new session can use it.\n"))
        (goto-char (point-min))))))

(defun safeslop-session--profile-egress-review (session-data observation)
  "Preview OBSERVATION as a durable rule; only a later explicit key writes it."
  (let ((profile (alist-get 'profile session-data))
        (hash (alist-get 'policy_hash session-data))
        (path (alist-get 'policy_path session-data)))
    (unless (and (stringp profile) (stringp hash)
                 (not (string-prefix-p "builtin:" (or path ""))))
      (user-error "Always allow requires a project profile snapshot"))
    (let* ((buf (safeslop-session--open-review-buffer
                 "*safeslop profile egress review*" "Persistent egress review"
                 "Loading hash-checked policy delta; no policy is being changed."
                 #'safeslop-profile-egress-review-mode))
           (token (list profile hash observation)))
      (with-current-buffer buf
        (setq safeslop-profile-egress-review-session-data session-data
              safeslop-profile-egress-review-observation observation
              safeslop-profile-egress-review-can-add nil
              safeslop-profile-egress-review-token token))
      (safeslop-session--egress-dispatch
       (safeslop-session--profile-egress-args "preview" profile path
                                             (alist-get 'host observation) (alist-get 'port observation) hash)
       "*safeslop profile egress preview*"
       (lambda (envelope)
         (when (buffer-live-p buf)
           (with-current-buffer buf
             (when (eq token safeslop-profile-egress-review-token)
               (safeslop-session--profile-egress-render
                session-data observation envelope buf)))))
       t))))

(defun safeslop-session--egress-grants-summary (data)
  "Return DATA's value-free session grants as a compact detail string."
  (let ((grants (alist-get 'egress_grants data))
        (revision (or (alist-get 'egress_grant_revision data) 0)))
    (if (null grants)
        (format "none (revision %s)" revision)
      (format "%s (revision %s)"
              (mapconcat
               (lambda (grant)
                 (format "%s:%s (%s)"
                         (or (alist-get 'host grant) "?")
                         (or (alist-get 'port grant) "?")
                         (or (alist-get 'id grant) "?")))
               grants ", ")
              revision))))


(defvar-local safeslop-session-detail-egress-request-token nil
  "Opaque token rejecting stale pending-count callbacks after detail reuse.")

(defun safeslop-session--detail-pending-render (buffer envelope)
  "Replace BUFFER's passive egress-count line without selecting its window."
  (when (buffer-live-p buffer)
    (with-current-buffer buffer
      (let ((inhibit-read-only t)
            (count (alist-get 'pending_count (safeslop-contract-data envelope))))
        (save-excursion
          (goto-char (point-min))
          (when (re-search-forward "^Egress review:.*$" nil t)
            (replace-match
             (if (and (safeslop-contract-ok-p envelope) (integerp count))
                 (format "Egress review: %d pending denied destination%s (v to review)"
                         count (if (= count 1) "" "s"))
               "Egress review: unavailable; press v to retry"))))))))

(defun safeslop-session--detail-request-pending-count (session-id data buffer)
  "Asynchronously discover the passive count for a container-deny detail view."
  (when (and (equal (alist-get 'environment data) "container")
             (equal (alist-get 'network data) "deny")
             (buffer-live-p buffer))
    (let ((token (list session-id)))
      (with-current-buffer buffer
        (setq safeslop-session-detail-egress-request-token token))
      (safeslop-session-egress-observations
       session-id
       (lambda (envelope)
         (when (buffer-live-p buffer)
           (with-current-buffer buffer
             (when (eq token safeslop-session-detail-egress-request-token)
               (safeslop-session--detail-pending-render buffer envelope)))))
       t))))

(provide 'safeslop-egress)
;;; safeslop-egress.el ends here
