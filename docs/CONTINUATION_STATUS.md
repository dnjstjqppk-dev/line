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
