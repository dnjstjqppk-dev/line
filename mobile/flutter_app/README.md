# Flutter mobile client (Android/iOS)

This includes actual Flutter/Dart user interface and API integration for register/login, friend search/add, conversation creation/list, text sending, message history, read cursors and WebSocket events. It does **not** yet include native Android/iOS generated platform runners; create them with a locally installed Flutter SDK before building:

```bash
cd mobile/flutter_app
flutter create --platforms=android,ios .
flutter pub get
flutter run --dart-define=API_URL=http://10.0.2.2:8080 # Android Emulator
# iOS simulator: --dart-define=API_URL=http://localhost:8080
```

For physical devices, use a LAN-accessible HTTPS reverse proxy (and WSS). HTTP cleartext on mobile is not approved for production. Access tokens expire after 15 minutes and the MVP does not implement refresh-token rotation. The secure storage keeps the short-lived access token; device session refresh must be implemented before shipping.
