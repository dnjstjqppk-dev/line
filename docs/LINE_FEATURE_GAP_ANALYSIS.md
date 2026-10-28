# Feature gap analysis
This evaluates independent messenger behavior, not proprietary LINE internals.
| Module | Current evidence | Gap |
|---|---|---|
| A Identity | HTTP registration/login; refresh security regression | Recovery, verification, session-bound access tokens, rate limits |
| B Social | Existing social handler and API suite | Permission/consent UX and privacy audit |
| C Messaging | Loopback HTTP/WS; persistence and idempotency tests | Two-device UI, durable replica delivery, offline client queue |
| D Groups | Membership-backed server and existing tests | Complete invite/leave/admin/mention UI acceptance |
| E Media | Metadata only; access controls tested | Real byte storage, checksum validation, playback, resumable transfer |
| F/G Calling | SDP/ICE signaling handlers | Peer connection, audio/video capture, TURN and recovery |
| H Group calls | Participant records | SFU integration and capacity tests |
| I Push | Device token registration | FCM provider delivery, background receipt, deep links |
| J UX | Vue builds and Flutter smoke tests | Central i18n, accessibility/device/network acceptance |
| K Operations | Existing admin handlers | Full RBAC audit, privacy policy enforcement, monitoring and restore drill |
Physical-device, production and external provider checks remain unverified.
