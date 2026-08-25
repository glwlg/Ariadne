#![windows_subsystem = "windows"]

mod capture;
mod clipboard;
mod hosts;
mod json_compare;
mod launcher;
mod network;
#[path = "../search.rs"]
mod search;
mod settings;
mod theme;
mod tray;

use gpui::{
    App, AppContext, KeyBinding, WindowBackgroundAppearance, WindowBounds, WindowKind,
    WindowOptions, prelude::*, px, size,
};
use gpui_component_assets::Assets;
use gpui_platform::application;

use capture::launch_screenshot;
use launcher::{Back, Confirm, LauncherPage, MoveDown, MoveUp};
use settings::{configured_global_hotkey, configured_screenshot_hotkey};

#[cfg(windows)]
fn start_global_hotkeys() {
    use std::ptr::null_mut;
    use std::thread;
    use windows_sys::Win32::UI::Input::KeyboardAndMouse::{
        MOD_ALT, MOD_NOREPEAT, RegisterHotKey, UnregisterHotKey,
    };
    use windows_sys::Win32::UI::WindowsAndMessaging::{GetMessageW, MSG, PeekMessageW, WM_HOTKEY};

    thread::spawn(|| unsafe {
        const LAUNCHER_HOTKEY_ID: i32 = 0xA56;
        const SCREENSHOT_HOTKEY_ID: i32 = 0xA57;
        let launcher_hotkey =
            parse_hotkey(&configured_global_hotkey()).unwrap_or((MOD_ALT, b'Q' as u32));
        let screenshot_hotkey =
            parse_hotkey(&configured_screenshot_hotkey()).unwrap_or((MOD_ALT, b'A' as u32));
        // RegisterHotKey posts WM_HOTKEY to this thread; create its message queue first.
        let mut queue_probe = MSG::default();
        PeekMessageW(&mut queue_probe, null_mut(), 0, 0, 0);
        let launcher_registered = RegisterHotKey(
            null_mut(),
            LAUNCHER_HOTKEY_ID,
            launcher_hotkey.0 | MOD_NOREPEAT,
            launcher_hotkey.1,
        ) != 0;
        let screenshot_registered = RegisterHotKey(
            null_mut(),
            SCREENSHOT_HOTKEY_ID,
            screenshot_hotkey.0 | MOD_NOREPEAT,
            screenshot_hotkey.1,
        ) != 0;
        if !launcher_registered && !screenshot_registered {
            return;
        }
        let mut message = MSG::default();
        while GetMessageW(&mut message, null_mut(), 0, 0) > 0 {
            if message.message != WM_HOTKEY {
                continue;
            }
            match message.wParam as i32 {
                LAUNCHER_HOTKEY_ID => tray::show_launcher(),
                SCREENSHOT_HOTKEY_ID => launch_screenshot(),
                _ => {}
            }
        }
        if launcher_registered {
            UnregisterHotKey(null_mut(), LAUNCHER_HOTKEY_ID);
        }
        if screenshot_registered {
            UnregisterHotKey(null_mut(), SCREENSHOT_HOTKEY_ID);
        }
    });
}

#[cfg(windows)]
fn parse_hotkey(value: &str) -> Option<(u32, u32)> {
    use windows_sys::Win32::UI::Input::KeyboardAndMouse::{
        MOD_ALT, MOD_CONTROL, MOD_SHIFT, MOD_WIN, VK_F1,
    };

    let mut parts = value.split('+').map(str::trim).collect::<Vec<_>>();
    let key = parts.pop()?.to_ascii_uppercase();
    if key.is_empty() {
        return None;
    }
    let mut modifiers = 0;
    for modifier in parts {
        modifiers |= if modifier.eq_ignore_ascii_case("alt") {
            MOD_ALT
        } else if modifier.eq_ignore_ascii_case("ctrl") || modifier.eq_ignore_ascii_case("control")
        {
            MOD_CONTROL
        } else if modifier.eq_ignore_ascii_case("shift") {
            MOD_SHIFT
        } else if modifier.eq_ignore_ascii_case("win") || modifier.eq_ignore_ascii_case("windows") {
            MOD_WIN
        } else {
            return None;
        };
    }
    let virtual_key = if let Some(index) = key
        .strip_prefix('F')
        .and_then(|value| value.parse::<u32>().ok())
        .filter(|index| (1..=24).contains(index))
    {
        VK_F1 as u32 + index - 1
    } else if key.len() == 1 && key.as_bytes()[0].is_ascii_alphanumeric() {
        key.as_bytes()[0] as u32
    } else {
        return None;
    };
    Some((modifiers, virtual_key))
}
#[cfg(not(windows))]
fn start_global_hotkeys() {}
fn main() {
    // App::run drops its setup closure after window creation; keep the tray handle alive for the process.
    let tray = tray::start().map(|handle| Box::leak(Box::new(handle)));
    application().with_assets(Assets).run(move |cx: &mut App| {
        gpui_component::init(cx);
        gpui_component::Theme::set_scrollbar_mode(
            gpui_component::scroll::ScrollbarMode::Always,
            cx,
        );
        cx.bind_keys([
            KeyBinding::new("up", MoveUp, Some("Ariadne")),
            KeyBinding::new("down", MoveDown, Some("Ariadne")),
            KeyBinding::new("enter", Confirm, Some("Ariadne")),
            KeyBinding::new("escape", Back, Some("Ariadne")),
        ]);

        let _tray = tray;
        let bounds = WindowBounds::centered(size(px(1080.), px(90.)), cx);
        cx.open_window(
            WindowOptions {
                window_bounds: Some(bounds),
                titlebar: None,
                kind: WindowKind::PopUp,
                app_owns_titlebar_drag: true,
                window_background: WindowBackgroundAppearance::Transparent,
                is_movable: true,
                is_resizable: false,
                is_minimizable: false,
                window_min_size: Some(size(px(720.), px(86.))),
                ..Default::default()
            },
            |window, cx| {
                tray::set_launcher_window(window);
                let page = cx.new(|cx| LauncherPage::new(window, cx));
                cx.new(|cx| {
                    gpui_component::Root::new(page, window, cx)
                        .bordered(false)
                        .bg(gpui::transparent_black())
                })
            },
        )
        .expect("failed to open Ariadne GPUI window");
        start_global_hotkeys();
        cx.activate(true);
    });
}
