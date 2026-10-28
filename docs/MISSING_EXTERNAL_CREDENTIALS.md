# External prerequisites and non-external gaps
| Dependency | Required evidence |
|---|---|
| FCM project/service account | Actual push to background/terminated Android app |
| Email/SMS provider and verified sender | Possession verification and recovery delivery |
| TURN endpoint and short-lived credentials | Relay-only real audio/video across separate networks |
| SFU deployment | Group media acceptance and capacity |
| PostgreSQL/object storage staging services | Migration, restart, authorization and restore tests |
| Android release signing material | Gemini signed release APK/AAB verification |
Credentials must be configured out of source and never pasted into reports.
Missing local implementation (WebRTC peer connections, media bytes, offline queue, E2EE) is not merely a credential blocker.
No connected Android device was available; emulator/physical-device acceptance is pending.
