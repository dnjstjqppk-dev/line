# Phase 1-2: Production Push & WebRTC Calling

## Phase 1: Production Push & Notifications (FCM/APNs)

Priority: P0 | Effort: 2-3 weeks

### 1.1 FCM Service Account Integration
- Implement RSA JWT signing for Google OAuth2 (replace placeholder in getFCMAccessToken)
- Add golang.org/x/oauth2/google dependency for service account auth
- Auto-refresh access token before expiry (1-hour TTL)
- Store FCM project ID in dedicated config (not reuse PushTokenKey)
- Add FCM topic-based broadcast for group messages
- Handle FCM error responses (invalid token, unregistered, quota exceeded)
- Implement token cleanup: revoke stale push tokens after 3 consecutive failures

### 1.2 APNs JWT Authentication
- Implement ECDSA P-256 JWT signing for APNs (replace placeholder)
- Add golang.org/x/crypto or golang-jwt/jwt for ES256
- Handle APNs response codes (410 Gone, 400 BadDeviceToken)
- Support both development and production APNs endpoints
- Implement APNs topic-based push for VoIP (PushKit)

### 1.3 Push Notification Content
- Rich notifications with images (FCM data payload with image URL)
- Action buttons (reply, mark as read) via FCM/APNs categories
- Notification grouping by conversation
- Badge count management (unread count sync)
- Silent push for read receipt sync
- Push notification preferences per conversation (mute, custom sound)

### 1.4 Push Token Management
- Encrypt push tokens at rest (verify PushTokenKey usage)
- Token rotation on app reinstall
- Multi-device push token deduplication
- Push token health check endpoint

## Phase 2: Real WebRTC Calling (Client + Backend)

Priority: P0 | Effort: 3-4 weeks

### 2.1 WebRTC Client Implementation (Frontend)
- Create WebRTCManager class in messenger-utils.ts
- Implement getUserMedia with camera/mic permission handling
- Implement RTCPeerConnection lifecycle (create, offer, answer, ICE)
- Add STUN/TURN server configuration
- Implement ICE candidate gathering and trickle ICE
- Add video rendering (local/remote video elements)
- Implement mute/unmute (audio + video) controls
- Add speaker/earpiece switching (mobile)
- Implement call timer and duration display
- Add picture-in-picture (PiP) for video calls
- Implement screen sharing via getDisplayMedia
- Add call quality indicators (packet loss, jitter, RTT)

### 2.2 TURN Server Integration
- Deploy coturn or use Cloudflare TURN
- Add TURN credentials to WebRTC config (short-lived)
- Implement TURN credential provisioning endpoint (backend)
- Add ICE candidate filtering (prefer host, srflx, relay)

### 2.3 Group Calling (SFU)
- Implement SFU (Selective Forwarding Unit) or use mediasoup
- Add group call UI (grid layout, active speaker detection)
- Implement bandwidth adaptation
- Add participant management (mute others, remove participant)

### 2.4 Call Signaling Backend Enhancement
- Add call state machine (ringing, connected, ended)
- Implement call timeout (30s no answer, missed)
- Add call history persistence
- Implement call recording metadata
- Add missed call notifications via push

### 2.5 Call Quality & Monitoring
- Implement getStats() polling for quality metrics
- Store call quality records
- Add call drop detection and auto-reconnect
- Implement network quality indicator