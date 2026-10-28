# Security audit checkpoint
2026-10-28. Partial code review, not an independent security certification.

## Remediated locally
- Expired/revoked refresh tokens: rejected. Atomic hash comparison ensures one successful concurrent rotation.
- Attachment metadata IDOR: owner or active conversation member only. Foreign-message attachment association rejected.
- Presence/push device IDOR: device ownership and nonrevoked state required.
- Call-quality IDOR: conversation membership required.
Regression tests are in refresh_security_test.go and resource_authorization_test.go.

## Open findings
- Access JWTs do not identify/check their session in withAuth; logout does not immediately revoke them.
- QR challenge approval/polling lack complete expiry and atomic-consumption protection.
- SQLite starts with fixed demo credentials, including administrator credentials.
- File content is not stored; checksum claims are not byte-integrity validation.
- Push tokens are stored as bytes without demonstrated encryption at rest or provider delivery.
- Commerce handler accepts client pricing and writes success without a payment provider; no real-money readiness.
- No demonstrated E2EE, rate-limit defense, production privacy review or dependency vulnerability scan.
- Editor diagnostics identify missing rows.Err checks in existing list handlers in calling_media.go; Go vet nevertheless passes.
Do not infer a clean security audit from a passing test suite.
