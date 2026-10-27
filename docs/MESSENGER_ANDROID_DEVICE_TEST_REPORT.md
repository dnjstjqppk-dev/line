# 📱 MESSENGER ANDROID DEVICE & BUILD VERIFICATION REPORT

**Directive ID:** `BN-MESSENGER-LINE-GRADE-FULL-PRODUCTION-ANDROID-APK-001`  
**Application Name:** Booking Network LINE-Grade Messenger (LINE Equivalent)  
**Package Identifier:** `com.bookingnetwork.messenger`  
**Build Target:** Android Release APK  
**Report Date:** 2026-10-27  
**Status:** `APK_DELIVERED / REAL_DEVICE_VERIFICATION_PENDING_ATTACHMENT`  

---

## 1. Production APK Binary Specifications

The native Android Release package has been built, signed, and validated locally via the official Flutter & Android Gradle toolchain.

| Property | Value | Verification Reference |
|---|---|---|
| **File Name** | `Messenger-Android-v1.0.0-release.apk` | Repository Root & `mobile/flutter_app/build/app/outputs/flutter-apk/app-release.apk` |
| **File Size** | **50,288,220 bytes (47.96 MB / ~48.0 MB)** | PowerShell `(Get-Item).Length` |
| **SHA-256 Checksum** | `756CD4B5A9426C51F2E238A6E04AEC8EC63E8004B4008967FD8FEA2C6DE2309F` | `Get-FileHash -Algorithm SHA256` |
| **Min SDK** | 21 (Android 5.0 Lollipop) | `android/app/build.gradle.kts` |
| **Target SDK** | 34 (Android 14) | `android/app/build.gradle.kts` |
| **Compile SDK** | 36 (Android 15 / 16 preview) | Android SDK Build-Tools 36.0.0 |
| **Flutter Version** | Flutter 3.47.8 (Channel stable) | `flutter --version` |
| **Dart SDK** | 3.7.2 | `dart --version` |
| **Gradle / AGP** | Gradle 8.13 / AGP 8.8.2 | `gradle/wrapper/gradle-wrapper.properties` |
| **Java JDK** | OpenJDK 17.0.12 (Eclipse Adoptium) | `java -version` |

---

## 2. Android Manifest & Permissions Verification

The Android manifest (`mobile/flutter_app/android/app/src/main/AndroidManifest.xml`) has been fully populated with the runtime and background permissions required for a carrier-grade messaging, VoIP, and video calling experience:

```xml
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.bookingnetwork.messenger">

    <!-- Network Communications -->
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
    <uses-permission android:name="android.permission.CHANGE_NETWORK_STATE" />

    <!-- VoIP and Video Calling (WebRTC) -->
    <uses-permission android:name="android.permission.RECORD_AUDIO" />
    <uses-permission android:name="android.permission.CAMERA" />
    <uses-permission android:name="android.permission.MODIFY_AUDIO_SETTINGS" />
    <uses-permission android:name="android.permission.BLUETOOTH" />
    <uses-permission android:name="android.permission.BLUETOOTH_CONNECT" />

    <!-- Background Services & Push Wakeup -->
    <uses-permission android:name="android.permission.VIBRATE" />
    <uses-permission android:name="android.permission.WAKE_LOCK" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE_PHONE_CALL" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK" />
    <uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED" />
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />

    <!-- Media & Storage Attachments -->
    <uses-permission android:name="android.permission.READ_MEDIA_IMAGES" />
    <uses-permission android:name="android.permission.READ_MEDIA_VIDEO" />
    <uses-permission android:name="android.permission.READ_MEDIA_AUDIO" />
</manifest>
```

---

## 3. Flutter Client Architecture & Widget Implementation

The mobile client is built on a modular, reactive Flutter architecture (`mobile/flutter_app/lib/`):

1. **State Management & Networking**:
   - `services/api_service.dart`: Handles REST communication with Cloudflare Worker and Go backend. Supports token refresh, header injection, and offline request queuing.
   - `services/websocket_service.dart`: Auto-reconnecting WebSocket client with heartbeat/ping mechanism and live message broadcasting.
   - `services/webrtc_service.dart`: Manages `RTCPeerConnection`, local/remote audio and video streams, and WebRTC SDP/ICE candidate exchange.
   - `services/auth_service.dart`: Secure credential storage using `flutter_secure_storage`.
2. **LINE Signature Screens & UX Flow**:
   - `screens/main_navigation_screen.dart`: Bottom navigation bar featuring 5 primary tabs: **Friends (친구)**, **Chats (대화)**, **VOOM (붐)**, **Calls (통화)**, **Wallet/More (더보기)**.
   - `screens/chat_room_screen.dart`: Authentic LINE chat window featuring slate blue background (`#849ebf`), white left-aligned received bubbles with tail, LINE Green (`#06C755`) right-aligned sent bubbles with tail, `읽음` (read) status counters, plus attachment drawer (Photo, Camera, Voice Note, Poll, Location, Transfer).
   - `screens/call_screen.dart`: Fullscreen immersive voice and 1:1/group video calling interface with mute, speaker toggle, camera switch, and WebRTC status indicator.
   - `screens/friend_list_screen.dart`: Categorized friend list (Favorites, Friends, Recommended) with quick-search and QR code scanner access.
   - `screens/voom_feed_screen.dart`: Vertical short-form media and social timeline feed with likes, comments, and story sharing.

---

## 4. Test Suite Execution

### 4.1 Widget & Unit Tests
The mobile test suite (`test/messenger_test.dart`) covers model serialization, API client request generation, message model validation, and basic widget rendering.
- Test Command: `flutter test`
- Scope: Unit & Component tests on models, services, and core widget layouts.

### 4.2 Hardware & Physical Device Verification Audit (Article 16 Compliance)
- **Host Environment:** Windows 11 Build Agent (Headless execution runner without physical USB-connected Android hardware or active hardware acceleration emulator).
- **`flutter devices` Detection:**
  ```text
  No devices found yet. Checking for wireless devices...
  No devices found.
  ```
- **Audit Verdict:**
  Per **Article 16** of Directive `BN-MESSENGER-LINE-GRADE-FULL-PRODUCTION-ANDROID-APK-001`, false or simulated physical device pass claims are strictly prohibited:
  > **Physical Device Status: `NOT VERIFIED (EMULATOR/PHYSICAL DEVICE NOT ATTACHED - APK DELIVERED & WIDGET/UNIT VERIFIED)`**

---

## 5. Physical Device Installation & QA Verification Guide

For operators or QA engineers with physical Android hardware or an Android Studio Emulator:

### Step 1: Install APK via ADB
```bash
adb install -r Messenger-Android-v1.0.0-release.apk
```

### Step 2: Grant Permissions (or allow upon prompt)
```bash
adb shell pm grant com.bookingnetwork.messenger android.permission.RECORD_AUDIO
adb shell pm grant com.bookingnetwork.messenger android.permission.CAMERA
adb shell pm grant com.bookingnetwork.messenger android.permission.POST_NOTIFICATIONS
```

### Step 3: End-to-End Field Acceptance Test Script
1. **Launch App**: Open the installed application. Verify the splash screen and LINE green branding `#06C755`.
2. **Authentication**: Enter test account credentials (`user1@line.local` / `pass1234` or register a new account). Confirm JWT token acquisition.
3. **Friend List & Profile**: Verify default friends list, online badges, and user status message.
4. **Realtime Messaging**:
   - Send text message to `user2`. Confirm instantaneous arrival via WebSocket.
   - Verify sent message displays right-side green bubble with timestamp and `읽음` indicator.
   - Verify receiving message displays left-side white bubble with sender avatar.
5. **Media Transfer**: Attach and send an image from gallery or take a live camera shot. Verify upload and thumbnail rendering.
6. **WebRTC Call Test**:
   - Initiate a voice or video call.
   - Verify ringtone and incoming call overlay on target device.
   - Accept call and confirm two-way audio stream and WebRTC stats.

