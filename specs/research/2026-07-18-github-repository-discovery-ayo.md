# GitHub App repository discovery — prior-art lessons (ayo)

Date: 2026-07-18 · Status: compiled/triaged; feeds the 0119 decision-FLO.

## Headline

1. GitHub can enumerate exactly the repositories visible to an App installation only through an installation access token. The safe exception to safeslop's session-owned mint rule is therefore a one-shot, explicit, host-only token attenuated to exactly `metadata:read`, with no repository selector, immediate cleanup, and GitHub's one-hour expiry as the backstop.
2. Repository discovery is selector assistance, never authorization. Kubernetes' SelfSubjectRulesReview scar applies directly: enumeration may be stale or incomplete and must not replace launch-time authorization.
3. Cancellation is a credential-cleanup event. Revocation must run on a fresh bounded cleanup context rather than the cancelled request context, and cleanup uncertainty must be visible and value-free.

## HIGH — carry into the decision

- **Pin discovery authority in code.** Mint with exactly `{metadata: read}` and omit `repositories` only because installation-wide scope is required to enumerate candidates. Never inherit the App's default permissions, request Contents, use ambient `gh`, or use a user token.
- **Keep the token one-shot and host-memory-only.** Resolve the existing linked key ref just in time; do not stage, persist, cache, log, return, or expose the token or private key. Do not cache the remote list across independent picker invocations.
- **Arm cleanup immediately after mint.** Revoke on success, list error, pagination error, and cancellation using a fresh, short cleanup context. A killed process can still leave a metadata-only token, so the honest hard residual bound is repository metadata for at most GitHub's one-hour expiry.
- **Paginate and reconcile.** Fetch bounded pages, validate each `owner/repo`, de-duplicate and sort, and compare collected rows to GitHub's `total_count`. Never silently label a partial list complete.
- **Keep discovery non-authoritative.** Describe rows as repositories accessible to the linked App installation at fetch time. Existing policy mutation confirmation and launch-time repo+permission mint remain authoritative; presence in the list is not a grant or readiness proof.
- **Preserve explicit operator control.** Fetch only after the operator chooses GitHub explicit repositories in `R`. Browsing, cancellation, or discovery failure writes nothing, trusts nothing, and retains current scopes/drafts.
- **Minimize the contract.** Public JSON and Emacs receive only the linked account's value-free identity/capability class and validated repository names needed by the picker. Strip raw GitHub repository objects, visibility, role, URLs, cursors, installation ids, permissions details, response bodies, refs, and values.

## MEDIUM — implementation choices after FLO

- Keep manual `owner/repo` entry as a fallback because a discovery snapshot can be stale; do not let the candidate list become an authorization allowlist.
- Show the installation's maximum Contents capability as a bounded `read|write` selector hint so Emacs can avoid offering an impossible write choice, while still treating launch as authoritative.
- Target one explicit linked account per discovery command. The Emacs flow can select a linked installation; multi-owner profiles retain manual entry rather than turning one operator action into multiple hidden token mints.
- Use ordinary Emacs completion over candidates with current read/write scopes as initial input, followed by the existing complete replacement confirmation. Avoid a new widget framework.

## CONTESTED — send to FLO

1. **Revoke failure:** return successful candidates plus a loud warning, or fail the command and withhold candidates? Expiry and metadata-only scope bound risk, but silent success would weaken lifecycle honesty.
2. **Incomplete pagination:** allow partial candidates with a warning/manual fallback, or fail closed so the UI cannot imply completeness?
3. **CLI shape:** one account-targeted read-only command versus an all-links aggregator that silently mints several tokens.
4. **Write capability:** expose a bounded top-level `max_access` hint versus keep the discovery response names-only and allow launch to reject impossible write requests.

## Sources

- GitHub, [List repositories accessible to the app installation](https://docs.github.com/en/rest/apps/installations?apiVersion=2022-11-28#list-repositories-accessible-to-the-app-installation).
- GitHub, [Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app).
- GitHub, [Best practices for creating a GitHub App](https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/best-practices-for-creating-a-github-app).
- GitHub, [Choosing permissions for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app).
- Kubernetes, [SelfSubjectRulesReview](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/self-subject-rules-review-v1/).

## Method

Expansion covered the current GitHub App client, account store, profile mutation contract, Emacs `R` flow, and specs 0068/0069/0090/0102/0111. Blind lanes: DeepSeek, Opus, and GLM; host lane independently checked primary sources and current code. Gemini was unavailable with provider billing HTTP 429. Consensus was strong on exact metadata attenuation, detached cleanup, non-authoritative snapshots, explicit invocation, bounded pagination, and output minimization. The warning-vs-error and partial-list choices remain adversarial decision work rather than being averaged away.
