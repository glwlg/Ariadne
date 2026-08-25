#[path = "bin/clipboard.rs"]
mod clipboard;
#[path = "bin/hosts.rs"]
mod hosts;
#[path = "bin/json_compare.rs"]
mod json_compare;
#[path = "bin/launcher.rs"]
mod launcher;
#[path = "bin/network.rs"]
mod network;
#[path = "../search.rs"]
mod search;
#[path = "bin/settings.rs"]
mod settings;
#[path = "bin/theme.rs"]
mod theme;

use gpui::{
    App, AppContext, KeyBinding, TitlebarOptions, WindowBackgroundAppearance, WindowBounds,
    WindowKind, WindowOptions, px, size,
};
use gpui_component_assets::Assets;
use gpui_platform::application;

use launcher::{Back, Confirm, LauncherPage, MoveDown, MoveUp};

fn main() {
    application().with_assets(Assets).run(|cx: &mut App| {
        gpui_component::init(cx);
        cx.bind_keys([
            KeyBinding::new("up", MoveUp, Some("Ariadne")),
            KeyBinding::new("down", MoveDown, Some("Ariadne")),
            KeyBinding::new("enter", Confirm, Some("Ariadne")),
            KeyBinding::new("escape", Back, Some("Ariadne")),
        ]);

        let bounds = WindowBounds::centered(size(px(1080.), px(90.)), cx);
        cx.open_window(
            WindowOptions {
                window_bounds: Some(bounds),
                titlebar: Some(TitlebarOptions {
                    title: Some("Ariadne".into()),
                    appears_transparent: true,
                    ..Default::default()
                }),
                kind: WindowKind::Normal,
                window_background: WindowBackgroundAppearance::Opaque,
                is_resizable: false,
                window_min_size: Some(size(px(720.), px(86.))),
                ..Default::default()
            },
            |window, cx| {
                let page = cx.new(|cx| LauncherPage::new(window, cx));
                cx.new(|cx| gpui_component::Root::new(page, window, cx))
            },
        )
        .expect("failed to open Ariadne GPUI window");
        cx.activate(true);
    });
}
