# RC9 systemd service-management CF audit

**Run status: incomplete.** No confirmed source-grounded vulnerability was established. Candidate `agent-token-panel-group-membership` remains ledger-only; `findings.json` is empty. This run does not count toward coverage.

Scope: commits `2eeb15ee`, `6aca184a`, `bb14a308`, `62912014`, `e2968b30`; HEAD `e2968b30962b2ac6b89c9c73f6f84dc699705558`, tree `d5f014659a9eca25103ffd8bc59ff9ae4c173bd5`, base `4c0694aa8e02e46145a775707b8d5a0355f7ce10`. Coverage check listed 84 pending commits; only `bb14a3087886dd9ba1c7552af3a21e45670ff709` intersects, leaving 83 pending outside scope. Global coverage is false.

Four reconnaissance agents and four initial hunters completed with `gpt-6-luna` / `max`. Source review saw Panel session/Origin/CSRF checks, fixed service/action allowlists and fixed-argument commands, bounded reads, stale-version checks, and per-Agent-process write serialization. No code, tests, or builds were run. Post-wave critic requested IPC/GID re-review; hunter output had mismatched `coverage_id` and was discarded. Independent candidate verifier was invoked but interrupted at the 2700-second limit. Phase 5 and distinct final-clean critic were not run.

Candidate: app-market install source can reuse an existing `kejilion-panel` group without checking supplemental or primary-GID membership, then use that group for root Agent token and socket. A lower-trust member might obtain the shared token and reach privileged Agent routes. Standard installer checks group members. Actual host membership and effective ACLs are deployment facts, so this is not confirmed.

Minimum missing evidence: on a non-production app-market installation, deployment owner reads group membership and host primary GIDs, inspects token/socket-directory/socket owner, mode and effective ACLs, and determines whether any member is less trusted than authorized root-Agent operators. Do not connect to Agent or invoke routes.

This incomplete CF pass supplies no confirmed source blocker, but is not completed coverage. Project preview policy does not make unresolved `needs_validation` alone a CF preview blocker; this says nothing about other release gates. Machine review is a high-confidence lead, not a human security review conclusion.
