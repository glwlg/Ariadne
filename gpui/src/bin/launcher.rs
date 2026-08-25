use std::time::Duration;

use gpui::{
    App, ClipboardItem, Context, Entity, MouseButton, MouseDownEvent, SharedString, Subscription,
    Task, TitlebarOptions, Window, WindowBackgroundAppearance, WindowBounds, WindowControlArea,
    WindowKind, WindowOptions, actions, div, prelude::*, px, size,
};
use gpui_component::{
    TitleBar,
    input::{Input, InputEvent, InputState},
    scroll::ScrollableElement as _,
};

use crate::{
    clipboard::ClipboardPage,
    hosts::HostsPage,
    json_compare::JsonComparePage,
    network::NetworkPage,
    search::{Route, SearchAction, SearchResult, search_results},
    settings::SettingsPage,
    theme,
};

actions!(launcher, [MoveUp, MoveDown, Confirm, Back]);

#[cfg(windows)]
fn remove_launcher_frame(window: &mut Window) {
    use raw_window_handle::{HasWindowHandle, RawWindowHandle};
    use windows_sys::Win32::UI::WindowsAndMessaging::{
        GWL_STYLE, GetWindowLongPtrW, SWP_FRAMECHANGED, SWP_NOMOVE, SWP_NOSIZE, SWP_NOZORDER,
        SetWindowLongPtrW, SetWindowPos, WS_CAPTION,
    };

    let Ok(handle) = HasWindowHandle::window_handle(window) else {
        return;
    };
    let RawWindowHandle::Win32(handle) = handle.as_raw() else {
        return;
    };
    let hwnd = handle.hwnd.get() as *mut core::ffi::c_void;
    let style = unsafe { GetWindowLongPtrW(hwnd, GWL_STYLE) };
    if style == 0 || style & WS_CAPTION as isize == 0 {
        return;
    }
    let style = style & !(WS_CAPTION as isize);
    unsafe {
        SetWindowLongPtrW(hwnd, GWL_STYLE, style);
        SetWindowPos(
            hwnd,
            core::ptr::null_mut(),
            0,
            0,
            0,
            0,
            SWP_FRAMECHANGED | SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER,
        );
    }
}

#[cfg(windows)]
fn hide_launcher_window(window: &Window) {
    use raw_window_handle::{HasWindowHandle, RawWindowHandle};
    use windows_sys::Win32::UI::WindowsAndMessaging::{SW_HIDE, ShowWindow};

    let Ok(handle) = HasWindowHandle::window_handle(window) else {
        return;
    };
    let RawWindowHandle::Win32(handle) = handle.as_raw() else {
        return;
    };
    let hwnd = handle.hwnd.get() as *mut core::ffi::c_void;
    unsafe {
        ShowWindow(hwnd, SW_HIDE);
    }
}

#[cfg(not(windows))]
fn hide_launcher_window(window: &Window) {
    window.minimize_window();
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum View {
    Launcher,
    Settings,
    Hosts,
    JsonCompare,
    Network,
    Clipboard,
}

enum ToolContent {
    Settings(Entity<SettingsPage>),
    Hosts(Entity<HostsPage>),
    JsonCompare(Entity<JsonComparePage>),
    Network(Entity<NetworkPage>),
    Clipboard(Entity<ClipboardPage>),
}

struct ToolWindow {
    title: &'static str,
    content: ToolContent,
}

impl ToolWindow {
    fn new(title: &'static str, content: ToolContent) -> Self {
        Self { title, content }
    }
}

impl Render for ToolWindow {
    fn render(&mut self, _window: &mut Window, _cx: &mut Context<Self>) -> impl IntoElement {
        let content = match &self.content {
            ToolContent::Settings(page) => page.clone().into_any_element(),
            ToolContent::Hosts(page) => page.clone().into_any_element(),
            ToolContent::JsonCompare(page) => page.clone().into_any_element(),
            ToolContent::Network(page) => page.clone().into_any_element(),
            ToolContent::Clipboard(page) => page.clone().into_any_element(),
        };
        div()
            .size_full()
            .flex()
            .flex_col()
            .bg(theme::hex(theme::BACKGROUND))
            .child(
                TitleBar::new()
                    .bg(theme::hex(theme::BACKGROUND))
                    .border_color(theme::hex(theme::BORDER))
                    .text_color(theme::hex(theme::FOREGROUND))
                    .child(div().text_sm().child(self.title)),
            )
            .child(div().flex_1().w_full().child(content))
    }
}

fn tool_window_options(title: &'static str, cx: &App) -> WindowOptions {
    WindowOptions {
        window_bounds: Some(WindowBounds::centered(size(px(1120.), px(760.)), cx)),
        titlebar: Some(TitlebarOptions {
            title: Some(title.into()),
            appears_transparent: true,
            ..Default::default()
        }),
        kind: WindowKind::Normal,
        app_owns_titlebar_drag: true,
        window_background: WindowBackgroundAppearance::Opaque,
        is_movable: true,
        is_resizable: true,
        is_minimizable: true,
        window_min_size: Some(size(px(800.), px(540.))),
        ..Default::default()
    }
}

pub struct LauncherPage {
    input: Entity<InputState>,
    query: String,
    results: Vec<SearchResult>,
    selected: usize,
    status: SharedString,
    view: View,
    last_layout_height: f32,
    settings: Option<Entity<SettingsPage>>,
    hosts: Option<Entity<HostsPage>>,
    json_compare: Option<Entity<JsonComparePage>>,
    network: Option<Entity<NetworkPage>>,
    clipboard: Option<Entity<ClipboardPage>>,
    search_generation: u64,
    search_task: Option<Task<()>>,
    _subscriptions: Vec<Subscription>,
}

impl LauncherPage {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> Self {
        #[cfg(windows)]
        remove_launcher_frame(window);
        let input = cx.new(|cx| {
            InputState::new(window, cx)
                .placeholder("搜索工具和文件，或输入命令：json / base64 / url / uuid")
        });
        let mut page = Self {
            input: input.clone(),
            query: String::new(),
            results: Vec::new(),
            selected: 0,
            status: "输入关键词开始搜索".into(),
            view: View::Launcher,
            last_layout_height: 0.,
            settings: None,
            hosts: None,
            json_compare: None,
            network: None,
            clipboard: None,
            search_generation: 0,
            search_task: None,
            _subscriptions: Vec::new(),
        };

        page._subscriptions.push(cx.subscribe_in(&input, window, {
            let input = input.clone();
            move |page, _, event, window, cx| match event {
                InputEvent::Change => {
                    page.queue_query(input.read(cx).value().to_string(), window, cx)
                }

                _ => {}
            }
        }));
        page.sync_window(window);
        page
    }

    fn queue_query(&mut self, query: String, window: &mut Window, cx: &mut Context<Self>) {
        self.search_generation = self.search_generation.wrapping_add(1);
        let generation = self.search_generation;
        self.query = query;
        self.results.clear();
        self.selected = 0;
        self.status = if self.query.trim().is_empty() {
            "输入关键词开始搜索".into()
        } else {
            "搜索中…".into()
        };
        self.sync_window(window);
        cx.notify();

        if self.query.trim().is_empty() {
            self.search_task = None;
            return;
        }

        let query = self.query.clone();
        self.search_task = Some(cx.spawn(async move |this, cx| {
            cx.background_executor()
                .timer(Duration::from_millis(50))
                .await;
            let results = cx
                .background_executor()
                .spawn(async move { search_results(&query) })
                .await;
            _ = this.update(cx, |page, cx| {
                if page.search_generation != generation {
                    return;
                }
                page.results = results;
                page.selected = page.selected.min(page.results.len().saturating_sub(1));
                page.status = if page.results.is_empty() {
                    "没有找到匹配线索".into()
                } else {
                    format!("搜索到 {} 个结果", page.results.len()).into()
                };
                cx.notify();
            });
        }));
    }

    fn sync_window(&mut self, window: &mut Window) {
        let height = if self.view == View::Launcher {
            if self.query.trim().is_empty() {
                90.
            } else {
                650.
            }
        } else {
            684.
        };
        if (height - self.last_layout_height).abs() > f32::EPSILON {
            window.resize(gpui::size(px(1080.), px(height)));
            self.last_layout_height = height;
        }
    }

    fn move_up(&mut self, _: &MoveUp, _: &mut Window, cx: &mut Context<Self>) {
        self.selected = self.selected.saturating_sub(1);
        cx.notify();
    }

    fn move_down(&mut self, _: &MoveDown, _: &mut Window, cx: &mut Context<Self>) {
        if !self.results.is_empty() {
            self.selected = (self.selected + 1).min(self.results.len() - 1);
            cx.notify();
        }
    }

    fn go_back(&mut self, _: &Back, window: &mut Window, cx: &mut Context<Self>) {
        if self.view != View::Launcher {
            self.back_to_launcher(window, cx);
        } else {
            self.search_generation = self.search_generation.wrapping_add(1);
            self.search_task = None;
            self.query.clear();
            self.results.clear();
            self.status = "输入关键词开始搜索".into();
            self.input
                .update(cx, |state, cx| state.set_value("", window, cx));
            self.sync_window(window);
            cx.notify();
            hide_launcher_window(window);
        }
    }

    pub(crate) fn back_to_launcher(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        self.search_generation = self.search_generation.wrapping_add(1);
        self.search_task = None;
        self.view = View::Launcher;
        self.query.clear();
        self.results.clear();
        self.selected = 0;
        self.status = "输入关键词开始搜索".into();
        self.input
            .update(cx, |state, cx| state.set_value("", window, cx));
        self.sync_window(window);
        cx.notify();
    }

    fn activate_selected(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        self.activate_index(self.selected, window, cx);
    }

    fn activate_index(&mut self, index: usize, window: &mut Window, cx: &mut Context<Self>) {
        let Some(result) = self.results.get(index).cloned() else {
            return;
        };
        self.selected = index;
        match result.action {
            SearchAction::Route(route) => self.open_route(route, window, cx),
            SearchAction::Copy(text) => {
                cx.write_to_clipboard(ClipboardItem::new_string(text));
                self.status = format!("{}：已复制", result.title).into();
                cx.notify();
            }
            SearchAction::OpenPath(path) => {
                let message = if std::process::Command::new("explorer.exe")
                    .arg(&path)
                    .spawn()
                    .is_ok()
                {
                    format!("已打开：{}", path.display())
                } else {
                    "打开路径失败".to_owned()
                };
                self.status = message.into();
                cx.notify();
            }
        }
    }

    fn open_route(&mut self, route: Route, window: &mut Window, cx: &mut Context<Self>) {
        hide_launcher_window(window);
        let opened = match route {
            Route::Settings => {
                let options = tool_window_options("设置中心", cx);
                cx.open_window(options, move |window, cx| {
                    let page = cx.new(|cx| SettingsPage::new(window, cx));
                    let shell =
                        cx.new(|_| ToolWindow::new("设置中心", ToolContent::Settings(page)));
                    window.activate_window();
                    cx.new(|cx| gpui_component::Root::new(shell, window, cx).bordered(false))
                })
            }
            Route::Hosts => {
                let options = tool_window_options("Hosts 管理", cx);
                cx.open_window(options, move |window, cx| {
                    let page = cx.new(|cx| HostsPage::new(window, cx));
                    let shell = cx.new(|_| ToolWindow::new("Hosts 管理", ToolContent::Hosts(page)));
                    window.activate_window();
                    cx.new(|cx| gpui_component::Root::new(shell, window, cx).bordered(false))
                })
            }
            Route::JsonCompare => {
                let options = tool_window_options("JSON 对比", cx);
                cx.open_window(options, move |window, cx| {
                    let page = cx.new(|cx| JsonComparePage::new(window, cx));
                    let shell =
                        cx.new(|_| ToolWindow::new("JSON 对比", ToolContent::JsonCompare(page)));
                    window.activate_window();
                    cx.new(|cx| gpui_component::Root::new(shell, window, cx).bordered(false))
                })
            }
            Route::Network => {
                let options = tool_window_options("网络工具", cx);
                cx.open_window(options, move |window, cx| {
                    let page = cx.new(|_| NetworkPage::new());
                    let shell = cx.new(|_| ToolWindow::new("网络工具", ToolContent::Network(page)));
                    window.activate_window();
                    cx.new(|cx| gpui_component::Root::new(shell, window, cx).bordered(false))
                })
            }
            Route::Clipboard => {
                let options = tool_window_options("剪贴板", cx);
                cx.open_window(options, move |window, cx| {
                    let page = cx.new(|_| ClipboardPage::new());
                    let shell = cx.new(|_| ToolWindow::new("剪贴板", ToolContent::Clipboard(page)));
                    window.activate_window();
                    cx.new(|cx| gpui_component::Root::new(shell, window, cx).bordered(false))
                })
            }
        };
        if opened.is_err() {
            self.status = "工具窗口打开失败".into();
        }
        cx.notify();
    }

    fn render_launcher(
        &mut self,
        _window: &mut Window,
        cx: &mut Context<Self>,
    ) -> impl IntoElement {
        let selected = self.selected;
        let results = self.results.clone();
        let selected_result = results.get(selected).cloned();
        let action_label = selected_result
            .as_ref()
            .map(|result| result.action_label)
            .unwrap_or("打开");
        div()
            .size_full()
            .flex()
            .flex_col()
            .gap_2()
            .p_3()
            .relative()
            .bg(gpui::transparent_black())
            .text_color(theme::hex(theme::FOREGROUND))
            .key_context("Ariadne")
            .child(
                div()
                    .absolute()
                    .top_0()
                    .left_0()
                    .right_0()
                    .h(px(20.))
                    .window_control_area(WindowControlArea::Drag),
            )
            .on_action(cx.listener(Self::move_up))
            .on_action(cx.listener(Self::move_down))
            .on_action(cx.listener(|this, _: &Confirm, window, cx| this.activate_selected(window, cx)))
            .on_action(cx.listener(Self::go_back))
            .child(
                div()
                    .h(px(52.))
                    .flex()
                    .items_center()
                    .gap_2()
                    .px_3()
                    .rounded_lg()
                    .border_1()
                    .border_color(theme::hex(theme::BORDER_STRONG))
                    .bg(theme::hex(theme::SURFACE))
                    .child(div().w(px(22.)).text_xl().text_color(theme::hex(theme::MUTED)).child("⌕"))
                    .child(Input::new(&self.input).flex_1())
                    .child(
                        div()
                            .rounded_sm()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_2()
                            .py_1()
                            .text_xs()
                            .text_color(theme::hex(theme::MUTED))
                            .child("Alt Q"),
                    ),
            )
            .when(self.view == View::Launcher && !self.query.trim().is_empty(), |this| {
                this.child(
                    div()
                        .flex_1()
                        .flex()
                        .border_1()
                        .border_color(theme::hex(theme::BORDER))
                        .bg(theme::hex(theme::SURFACE))
                        .child(
                            div()
                                .w(px(430.))
                                .flex()
                                .flex_col()
                                .border_r_1()
                                .border_color(theme::hex(theme::BORDER))
                                .child(
                                    div()
                                        .h(px(42.))
                                        .flex()
                                        .items_center()
                                        .justify_between()
                                        .px_3()
                                        .text_sm()
                                        .child("搜索结果")
                                        .child(
                                            div()
                                                .rounded_full()
                                                .bg(theme::hex(theme::SURFACE_SELECTED))
                                                .px_2()
                                                .py_1()
                                                .text_xs()
                                                .child(results.len().to_string()),
                                        ),
                                )
                                .child(
                                    div().flex_1().min_h_0().size_full().flex().flex_col().overflow_y_scrollbar().id("launcher-candidate-list").children(results.iter().enumerate().map(|(position, result)| {
                                        let active = position == selected;
                                        div()
                                            .flex_shrink_0()
                                            .flex()
                                            .items_center()
                                            .gap_3()
                                            .mx_2()
                                            .mb_1()
                                            .p_3()
                                            .rounded_md()
                                            .border_l_2()
                                            .border_color(theme::hex(if active { theme::PRIMARY } else { theme::SURFACE }))
                                            .bg(theme::hex(if active { theme::SURFACE_SELECTED } else { theme::SURFACE }))
                                            .hover(|style| style.bg(theme::hex(theme::SURFACE_SELECTED)))
                                            .on_mouse_down(MouseButton::Left, cx.listener(move |this, _: &MouseDownEvent, _, cx| this.select(position, cx)))
                                            .child(div().w(px(30.)).h(px(30.)).flex().items_center().justify_center().rounded_md().border_1().border_color(theme::hex(theme::BORDER)).text_sm().child(result.icon))
                                            .child(div().flex().flex_col().gap_1().child(div().text_sm().child(result.title.clone())).child(div().text_xs().text_color(theme::hex(theme::MUTED)).child(result.subtitle.clone())))
                                    })),
                                ),
                        )
                        .child(
                            div()
                                .flex_1()
                                .flex()
                                .flex_col()
                                .gap_3()
                                .p_6()
                                .child(if let Some(result) = selected_result {
                                    div()
                                        .flex()
                                        .flex_col()
                                        .gap_3()
                                        .child(div().flex().items_center().gap_3().child(div().w(px(42.)).h(px(42.)).flex().items_center().justify_center().rounded_lg().bg(theme::hex(theme::SURFACE_SELECTED)).text_lg().child(result.icon)).child(div().flex().flex_col().gap_1().child(div().text_xl().child(result.title.clone())).child(div().text_sm().text_color(theme::hex(theme::MUTED)).child(result.subtitle.clone()))))
                                        .child(div().text_sm().text_color(theme::hex(theme::FOREGROUND)).child(result.detail.clone()))
                                        .child(
                                            div()
                                                .flex()
                                                .gap_2()
                                                .child(
                                                    div()
                                                        .rounded_md()
                                                        .bg(theme::hex(theme::PRIMARY))
                                                        .text_color(theme::hex(theme::SURFACE))
                                                        .px_3()
                                                        .py_2()
                                                        .text_sm()
                                                        .child(action_label)
                                                        .on_mouse_down(MouseButton::Left, cx.listener(move |this, _: &MouseDownEvent, window, cx| this.activate_index(selected, window, cx))),
                                                )
                                                .child(div().rounded_md().border_1().border_color(theme::hex(theme::BORDER)).px_3().py_2().text_sm().text_color(theme::hex(theme::MUTED)).child("更多操作")),
                                        )
                                } else {
                                    div().text_sm().text_color(theme::hex(theme::MUTED)).child("没有找到匹配线索")
                                }),
                        ),
                )
            })
            .when(self.view == View::Launcher && !self.query.trim().is_empty(), |this| {
                this.child(
                    div()
                        .h(px(34.))
                        .flex()
                        .items_center()
                        .justify_between()
                        .px_2()
                        .text_xs()
                        .text_color(theme::hex(theme::MUTED))
                        .child(self.status.clone())
                        .child(format!("↑ ↓ 选择 · Enter {}", action_label)),
                )
            })
    }

    fn select(&mut self, position: usize, cx: &mut Context<Self>) {
        if position < self.results.len() {
            self.selected = position;
            cx.notify();
        }
    }
}

impl Render for LauncherPage {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.sync_window(window);
        let content = match self.view {
            View::Launcher => self.render_launcher(window, cx).into_any_element(),
            View::Settings => self
                .settings
                .as_ref()
                .map(|page| page.clone().into_any_element())
                .unwrap_or_else(|| div().into_any_element()),
            View::Hosts => self
                .hosts
                .as_ref()
                .map(|page| page.clone().into_any_element())
                .unwrap_or_else(|| div().into_any_element()),
            View::JsonCompare => self
                .json_compare
                .as_ref()
                .map(|page| page.clone().into_any_element())
                .unwrap_or_else(|| div().into_any_element()),
            View::Network => self
                .network
                .as_ref()
                .map(|page| page.clone().into_any_element())
                .unwrap_or_else(|| div().into_any_element()),
            View::Clipboard => self
                .clipboard
                .as_ref()
                .map(|page| page.clone().into_any_element())
                .unwrap_or_else(|| div().into_any_element()),
        };

        div()
            .size_full()
            .flex()
            .flex_col()
            .bg(gpui::transparent_black())
            .child(content)
    }
}
