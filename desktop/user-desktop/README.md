# Desktop shell (UNVERIFIED scaffold)

Tauri 2 + Rust wrapping the Vue user-web app. Native window and tray/OS media APIs are NOT implemented. No PC installer has been built. Run after installing Rust, Tauri OS prerequisites, and npm dependencies:

```bash
cd desktop/user-desktop
npm install
npm run tauri:dev
```

For a true PC production build, complete signed installers, background notifications, autostart, tray integration, WebRTC devices and native key management. The `frontendDist` and `beforeBuildCommand` paths are relative to Tauri config conventions and require a platform build check.
