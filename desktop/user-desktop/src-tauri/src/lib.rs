use std::sync::atomic::{AtomicU32, Ordering};

static UNREAD_BADGE_COUNT: AtomicU32 = AtomicU32::new(0);

#[tauri::command]
fn set_unread_badge(count: u32) -> u32 {
    UNREAD_BADGE_COUNT.store(count, Ordering::SeqCst);
    count
}

#[tauri::command]
fn get_unread_badge() -> u32 {
    UNREAD_BADGE_COUNT.load(Ordering::SeqCst)
}

#[tauri::command]
fn desktop_platform_capabilities() -> serde_json::Value {
    serde_json::json!({
        "standalone_messaging": true,
        "standalone_voice_video_call": true,
        "qr_login_supported": true,
        "system_tray": true,
        "file_drag_drop": true,
        "auto_update_ready": true
    })
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            set_unread_badge,
            get_unread_badge,
            desktop_platform_capabilities
        ])
        .run(tauri::generate_context!())
        .expect("error while running messenger desktop application");
}
