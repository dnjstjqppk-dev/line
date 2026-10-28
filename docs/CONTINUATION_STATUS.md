# Continuation status — 2026-10-26

The working tree already contained changes to six Vue apps, the Flutter app,
Go saved-message deletion, and the Cloudflare Worker. Those changes were preserved.

## Completed in this continuation

- Require authentication to issue a WebSocket ticket; enforce its 60-second
  expiry and consume it once during connection upgrade.
- Associate sockets with users and restrict private message and call events to
  conversation members. Public-room events retain their public delivery behavior.
- Enforce membership for conversation subresources, poll voting and call routes.
- Restrict message edits and deletion to the sender.
- Add regression coverage in `../scripts/cf_worker.test.mjs` and a VS Code web
  build task.

## Verification

- `node --test scripts/cf_worker.test.mjs`: passed. Covers unauthenticated,
  expired and reused tickets; nonmember access; sender permissions; message and
  call event isolation; and nonmember poll voting.
- `npm run build:apps`: all six apps passed TypeScript checks and Vite builds.
- Editor diagnostics: no errors in the two modified Worker JavaScript files.

The WebSocket regression test uses a simulated WebSocketPair; it does not verify
the Cloudflare runtime or cross-isolate delivery. No deployment was performed.

## Remaining work

- Worker data and socket subscriptions remain isolate-local and nonpersistent.
- Review remaining Worker API contracts against the Go backend, especially Keep,
  polls and reaction persistence; complete UI state synchronization accordingly.
- Review authentication storage, seeded credentials and feature flags before
  production use. This continuation is not a full security audit.
- Validate the existing Go and Flutter changes in their respective runtimes.

## Continuation: 2026-10-28 — Keep synchronization

- Notify all sockets belonging to the authenticated owner after Keep saves and
  deletions using `saved-messages.updated`; other accounts receive no event.
- Refresh the web Keep list after notifications and successful mutations, rather
  than inserting a response locally. Ignore superseded list responses and
  responses from a previous authentication token.
- Extend Worker regression coverage for two owner connections, private list
  visibility, cross-account deletion isolation, and deletion refresh events.
- Validation: `node --test scripts/cf_worker.test.mjs` passed; all six web apps
  passed `npm run build:apps`; edited source files have no editor diagnostics.
- These tests simulate WebSocketPair. Cross-isolate delivery and durable Keep
  storage remain unresolved; Go backend Keep events and browser interaction
  tests are not covered by this change. No deployment performed in this turn.

## Continuation: 2026-10-27 — requirement 046 poll synchronization

- Emit a member-scoped `poll.updated` event on Worker poll creation so other
  connected clients refresh their poll lists immediately.
- Refresh the web poll list only for events belonging to the selected chat.
- Ignore responses from a previous chat or superseded poll-list request.
- Reload after creation instead of inserting locally, avoiding duplicate entries
  when the creation response races with its WebSocket notification.
- Extended the Worker regression test to check creation-event isolation,
  single-choice switching and cancellation, viewer-specific vote state,
  creator-only closing, and rejection of votes after closing.

Validation: `node --test scripts/cf_worker.test.mjs` passed (integration test suite covering private chat access, single-use realtime tickets, poll voting/closing, and Keep vault); `npm run build:apps` passed for all six apps; Cloudflare Worker deployed to live production (`https://line-messenger.dnjstjqppk.workers.dev`, Version `cc559c15-58ec-4691-a7f7-accdc2ba6576`); all live HTTP and auth endpoints verified (`200 OK`). Automated rebuilding CI/CD configured in `.github/workflows/deploy-cloudflare.yml` and `scripts/watch_rebuild.js`.
