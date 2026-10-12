# Needs validation / unresolved candidate

Not a final finding: independent disposition and Phase-5 verification were not completed. Candidate remains ledger-only.

- Fingerprint: `agent-token-panel-group-membership`
- Blocker: app-market host supplemental/primary-GID membership and effective ACLs on the Agent token and socket were not observed and are not source-visible.
- Minimum safe check: on a non-production app-market install, deployment owner reads group members, primary GIDs, and token/socket owner, mode, and effective ACLs; determines whether any member is less trusted than authorized root-Agent operators. Do not connect to the Agent or invoke routes.
- Owner: RC9 release owner / deployment owner.
