use std::{
    fs,
    path::PathBuf,
    process::Command,
    sync::{Mutex, OnceLock},
};

use gpui::{
    ClipboardItem, Context, Entity, MouseButton, MouseDownEvent, SharedString, Window, div,
    prelude::*, px,
};
use gpui_component::{
    input::{Input, InputState},
    scroll::ScrollableElement as _,
};
use rusqlite::{Connection, OpenFlags};
use serde_json::json;

use crate::{capture::launch_screenshot, theme};

#[derive(Clone, Copy, PartialEq, Eq)]
enum SettingsTab {
    General,
    Shortcuts,
    Plugins,
    Launchers,
    Screenshot,
    Search,
    Data,
    Advanced,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub(crate) struct SearchExclusions {
    pub exclude_folders: Vec<String>,
    pub exclude_patterns: Vec<String>,
    pub include_extensions: Vec<String>,
    pub exclude_extensions: Vec<String>,
}

impl Default for SearchExclusions {
    fn default() -> Self {
        Self {
            exclude_folders: vec![
                r"%APPDATA%\Microsoft\Windows\Recent".into(),
                r"%APPDATA%".into(),
                r"%TEMP%".into(),
                r"%TMP%".into(),
                r"%LOCALAPPDATA%".into(),
                r"%USERPROFILE%\AppData\LocalLow".into(),
                r"%LOCALAPPDATA%\Temp".into(),
                r"%WINDIR%\Temp".into(),
                r"%WINDIR%".into(),
                r"%SystemRoot%".into(),
                r"%ProgramFiles%".into(),
                r"%ProgramFiles(x86)%".into(),
                r"%ProgramW6432%".into(),
                r"%ProgramData%".into(),
            ],
            exclude_patterns: vec![
                r"(?i)\.(tmp|temp|part|crdownload|download)$".into(),
                r"(?i)(^|[\\/])~\$[^\\/]*$".into(),
                r"(?i)(^|[\\/])\$recycle\.bin([\\/]|$)".into(),
                r"(?i)(^|[\\/])system volume information([\\/]|$)".into(),
                r"(?i)(^|[\\/])(tmp|temp|temp-index|tmp-index)([\\/]|$)".into(),
                r"(?i)[\\/]ebwebview([\\/]|$)".into(),
                r"(?i)(^|[\\/])\..*\.tmp[-.][^\\/]*$".into(),
                r"(?i)[\\/]users[\\/][^\\/]+[\\/]appdata[\\/](local|locallow|roaming)([\\/]|$)".into(),
                r"(?i)^[a-z]:[\\/](app|appdata|program files|program files \(x86\)|programdata|devapp)([\\/]|$)".into(),
                r"(?i)^[a-z]:[\\/]workspace[\\/]env([\\/]|$)".into(),
                r"(?i)^[a-z]:[\\/]users[\\/][^\\/]+[\\/]\.(cache|config|local|m2|gradle|nuget|npm|pnpm|yarn|cargo|rustup|wox|gemini|confirmo|switchhosts|antigravity|antigravity_cockpit|marscode)([\\/]|$)".into(),
                r"(?i)[\\/]appdata[\\/](local|locallow|roaming)[\\/].*[\\/](cache|cache2|cachestorage|code cache|gpucache|shadercache|crashpad|dawncache|blob_storage|inetcache|webcache|startupcache|browsermetrics|media cache|service worker|thumbnails)([\\/]|$)".into(),
                r"(?i)^[a-z]:[\\/](recovery|\$winreagent|config\.msi|windows\.old|msocache)([\\/]|$)".into(),
                r"(?i)(^|[\\/])(d3dscache|dxcache|gpucache|grshadercache|shadercache|dawncache)([\\/]|$)".into(),
                r"(?i)(^|[\\/])(\.m2|\.npm|\.pnpm-store|pnpm-store|npm-cache|pnpm-cache|go-build|pip-cache|ms-playwright|ms-playwright-go|package cache)([\\/]|$)".into(),
                r"(?i)(^|[\\/])(nuget[\\/]packages|\.cargo|\.rustup)([\\/]|$)".into(),
                r"(?i)(^|[\\/])(\.venv|venv|envs|virtualenv|__pypackages__)([\\/]|$)".into(),
                r"(?i)(^|[\\/])(site-packages|dist-packages)([\\/]|$)".into(),
                r"(?i)(^|[\\/])(jetbrains|trae cn|trae|antigravity|zed|qoder)([\\/]|$)".into(),
                r"(?i)(^|[\\/])(logs?|log)([\\/]|$)".into(),
                r"(?i)(^|[\\/])ariadne[\\/](capture_images|capture_thumbnails)([\\/]|$)".into(),
                r"(?i)\.(log|journal|db-journal|sqlite-wal|sqlite-shm|db-wal|db-shm|odlgz|statistic|dxcache-shm|dxcache-wal)$".into(),
                r"(?i)(^|[\\/])(\$mft|\$logfile|\$bitmap|\$boot|\$badclus|\$secure|\$upcase|\$volume|\$attrdef|pagefile\.sys|hiberfil\.sys|swapfile\.sys|dumpstack\.log\.tmp|thumbs\.db|desktop\.ini)$".into(),
                r"(?i)(^|[\\/])(\.git|\.hg|\.svn|node_modules|\.pnpm-store|__pycache__|\.pytest_cache|\.ruff_cache|\.mypy_cache|\.gradle|\.idea|\.vscode|\.cache|\.codex|\.codex-audit|coverage|dist|build|target|out|bin|obj|\.next|\.nuxt|\.vite|\.turbo|\.parcel-cache|\.svelte-kit|\.angular|\.vercel)([\\/]|$)".into(),
            ],
            include_extensions: Vec::new(),
            exclude_extensions: vec![
                ".tmp".into(),
                ".temp".into(),
                ".part".into(),
                ".crdownload".into(),
                ".download".into(),
                ".log".into(),
                ".journal".into(),
                ".db-journal".into(),
                ".sqlite-wal".into(),
                ".sqlite-shm".into(),
                ".db-wal".into(),
                ".db-shm".into(),
                ".odlgz".into(),
                ".statistic".into(),
                ".dxcache-shm".into(),
                ".dxcache-wal".into(),
            ],
        }
    }
}

#[derive(Clone, Debug)]
struct SettingsValues {
    run_on_startup: bool,
    global_hotkey: String,
    screenshot_hotkey: String,
    search: SearchExclusions,
}

impl Default for SettingsValues {
    fn default() -> Self {
        Self {
            run_on_startup: false,
            global_hotkey: "Alt+Q".into(),
            screenshot_hotkey: "Alt+A".into(),
            search: SearchExclusions::default(),
        }
    }
}
pub struct SettingsPage {
    run_on_startup: bool,
    global_hotkey: String,
    screenshot_hotkey: String,
    search: SearchExclusions,
    exclude_folders: Entity<InputState>,
    exclude_patterns: Entity<InputState>,
    include_extensions: Entity<InputState>,
    exclude_extensions: Entity<InputState>,
    tab: SettingsTab,
    status: SharedString,
}

impl SettingsPage {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> Self {
        let settings = load_settings();
        let exclude_folders =
            cx.new(|cx| InputState::new(window, cx).placeholder("排除文件夹路径，每行一项"));
        let exclude_patterns =
            cx.new(|cx| InputState::new(window, cx).placeholder("排除模式，每行一项"));
        let include_extensions = cx.new(|cx| {
            InputState::new(window, cx).placeholder("包含扩展名（可选），例如 .md、.txt")
        });
        let exclude_extensions =
            cx.new(|cx| InputState::new(window, cx).placeholder("排除扩展名，例如 .log、.tmp"));
        exclude_folders.update(cx, |state, cx| {
            state.set_value(settings.search.exclude_folders.join("\n"), window, cx)
        });
        exclude_patterns.update(cx, |state, cx| {
            state.set_value(settings.search.exclude_patterns.join("\n"), window, cx)
        });
        include_extensions.update(cx, |state, cx| {
            state.set_value(settings.search.include_extensions.join("\n"), window, cx)
        });
        exclude_extensions.update(cx, |state, cx| {
            state.set_value(settings.search.exclude_extensions.join("\n"), window, cx)
        });
        Self {
            run_on_startup: settings.run_on_startup,
            global_hotkey: settings.global_hotkey,
            screenshot_hotkey: settings.screenshot_hotkey,
            search: settings.search,
            exclude_folders,
            exclude_patterns,
            include_extensions,
            exclude_extensions,
            tab: SettingsTab::General,
            status: "设置保存在当前用户配置".into(),
        }
    }

    fn back(&mut self, _: &MouseDownEvent, window: &mut Window, _: &mut Context<Self>) {
        window.remove_window();
    }

    fn select_tab(
        &mut self,
        tab: SettingsTab,
        _: &MouseDownEvent,
        _: &mut Window,
        cx: &mut Context<Self>,
    ) {
        self.tab = tab;
        cx.notify();
    }

    fn toggle_startup(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        self.run_on_startup = !self.run_on_startup;
        self.status = if self.run_on_startup {
            "已开启开机启动（保存后生效）"
        } else {
            "已关闭开机启动（保存后生效）"
        }
        .into();
        cx.notify();
    }

    fn save(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        let mut search = self.search.clone();
        search.exclude_folders = parse_list(&self.exclude_folders.read(cx).value());
        search.exclude_patterns = parse_list(&self.exclude_patterns.read(cx).value());
        search.include_extensions = parse_list(&self.include_extensions.read(cx).value());
        search.exclude_extensions = parse_list(&self.exclude_extensions.read(cx).value());
        let config_result = save_settings(
            self.run_on_startup,
            &self.global_hotkey,
            &self.screenshot_hotkey,
            &search,
        );
        if config_result.is_ok() {
            self.search = search;
        }
        let startup_result = set_startup(self.run_on_startup);
        self.status = match (config_result, startup_result) {
            (Ok(()), Ok(())) => "设置已保存".into(),
            (Err(error), _) => format!("配置保存失败：{error}").into(),
            (_, Err(error)) => format!("配置已保存，但开机启动设置失败：{error}").into(),
        };
        cx.notify();
    }

    fn test_screenshot(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        launch_screenshot();
        self.status = "已打开截图覆盖层".into();
        cx.notify();
    }

    fn copy_shortcut(
        &mut self,
        shortcut: &str,
        _: &MouseDownEvent,
        _: &mut Window,
        cx: &mut Context<Self>,
    ) {
        cx.write_to_clipboard(ClipboardItem::new_string(shortcut.to_owned()));
        self.status = format!("已复制快捷键：{shortcut}").into();
        cx.notify();
    }

    fn open_config_dir(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        let config_path = settings_path();
        let Some(path) = config_path.parent() else {
            self.status = "配置目录不可用".into();
            cx.notify();
            return;
        };
        let result = if cfg!(windows) {
            Command::new("explorer.exe").arg(path).spawn()
        } else {
            Command::new("xdg-open").arg(path).spawn()
        };
        let status: SharedString = if result.is_ok() {
            "已打开配置目录".into()
        } else {
            "打开配置目录失败".into()
        };
        self.status = status;
        cx.notify();
    }

    fn reload(&mut self, _: &MouseDownEvent, window: &mut Window, cx: &mut Context<Self>) {
        let settings = load_settings();
        self.run_on_startup = settings.run_on_startup;
        self.global_hotkey = settings.global_hotkey;
        self.screenshot_hotkey = settings.screenshot_hotkey;
        self.search = settings.search;
        let folders = self.search.exclude_folders.join("\n");
        self.exclude_folders
            .update(cx, |state, cx| state.set_value(folders, window, cx));
        let patterns = self.search.exclude_patterns.join("\n");
        self.exclude_patterns
            .update(cx, |state, cx| state.set_value(patterns, window, cx));
        let includes = self.search.include_extensions.join("\n");
        self.include_extensions
            .update(cx, |state, cx| state.set_value(includes, window, cx));
        let excludes = self.search.exclude_extensions.join("\n");
        self.exclude_extensions
            .update(cx, |state, cx| state.set_value(excludes, window, cx));
        self.status = "已重新读取本地配置".into();
        cx.notify();
    }

    fn tab_button(
        &self,
        tab: SettingsTab,
        label: &'static str,
        cx: &mut Context<Self>,
    ) -> impl IntoElement {
        div()
            .id(label)
            .w_full()
            .rounded_md()
            .bg(theme::hex(if self.tab == tab {
                theme::SURFACE_SELECTED
            } else {
                theme::BACKGROUND
            }))
            .hover(|style| style.bg(theme::hex(theme::SURFACE_SELECTED)))
            .px_3()
            .py_2()
            .text_sm()
            .text_color(theme::hex(if self.tab == tab {
                theme::FOREGROUND
            } else {
                theme::MUTED
            }))
            .child(label)
            .on_mouse_down(
                MouseButton::Left,
                cx.listener(move |this, event, window, cx| {
                    this.select_tab(tab, event, window, cx);
                }),
            )
    }

    fn render_general(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("常规设置"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("亮石墨是当前唯一界面主题，设置与工具页保持一致。"),
            )
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .border_t_1()
                    .border_color(theme::hex(theme::BORDER))
                    .pt_3()
                    .child(
                        div()
                            .flex()
                            .flex_col()
                            .gap_1()
                            .child(div().text_sm().child("开机启动"))
                            .child(
                                div()
                                    .text_xs()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("登录 Windows 后自动打开 Ariadne"),
                            ),
                    )
                    .child(
                        div()
                            .rounded_full()
                            .bg(theme::hex(if self.run_on_startup {
                                theme::SUCCESS
                            } else {
                                theme::BORDER_STRONG
                            }))
                            .p_1()
                            .child(
                                div()
                                    .w(px(28.))
                                    .h(px(16.))
                                    .rounded_full()
                                    .bg(theme::hex(theme::SURFACE))
                                    .when(self.run_on_startup, |this| this.ml(px(12.))),
                            )
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::toggle_startup)),
                    ),
            )
            .child(
                div()
                    .flex()
                    .flex_col()
                    .gap_2()
                    .border_t_1()
                    .border_color(theme::hex(theme::BORDER))
                    .pt_3()
                    .child(div().text_sm().child("当前快捷键"))
                    .child(
                        div()
                            .flex()
                            .justify_between()
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("唤起 Launcher"),
                            )
                            .child(
                                div()
                                    .rounded_md()
                                    .border_1()
                                    .border_color(theme::hex(theme::BORDER))
                                    .px_3()
                                    .py_2()
                                    .text_sm()
                                    .child(self.global_hotkey.clone()),
                            ),
                    )
                    .child(
                        div()
                            .flex()
                            .justify_between()
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("截图覆盖层"),
                            )
                            .child(
                                div()
                                    .rounded_md()
                                    .border_1()
                                    .border_color(theme::hex(theme::BORDER))
                                    .px_3()
                                    .py_2()
                                    .text_sm()
                                    .child(self.screenshot_hotkey.clone()),
                            ),
                    ),
            )
            .child(
                div()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child(format!("配置文件：{}", settings_path().display())),
            )
    }

    fn render_shortcuts(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        let screenshot = self.screenshot_hotkey.clone();
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("快捷键"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("快捷键在应用未聚焦时也可用；点击按钮可立即验证动作。"),
            )
            .child(shortcut_row(
                "唤起 Launcher",
                &self.global_hotkey,
                "由 Ariadne 全局注册",
            ))
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .border_t_1()
                    .border_color(theme::hex(theme::BORDER))
                    .pt_3()
                    .child(
                        div()
                            .flex()
                            .flex_col()
                            .gap_1()
                            .child(div().text_sm().child("截图覆盖层"))
                            .child(
                                div()
                                    .text_xs()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("打开 Ariadne 截图覆盖层"),
                            ),
                    )
                    .child(
                        div()
                            .flex()
                            .gap_2()
                            .child(
                                div()
                                    .rounded_md()
                                    .border_1()
                                    .border_color(theme::hex(theme::BORDER))
                                    .px_3()
                                    .py_2()
                                    .text_sm()
                                    .child(screenshot.clone())
                                    .on_mouse_down(
                                        MouseButton::Left,
                                        cx.listener(move |this, event, window, cx| {
                                            this.copy_shortcut(&screenshot, event, window, cx)
                                        }),
                                    ),
                            )
                            .child(
                                div()
                                    .rounded_md()
                                    .bg(theme::hex(theme::PRIMARY))
                                    .text_color(theme::hex(theme::SURFACE))
                                    .px_3()
                                    .py_2()
                                    .text_sm()
                                    .child("测试截图")
                                    .on_mouse_down(
                                        MouseButton::Left,
                                        cx.listener(Self::test_screenshot),
                                    ),
                            ),
                    ),
            )
            .child(
                div()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child("当前版本使用 Alt+Q 与 Alt+A 默认组合，保存后会保留本地设置。"),
            )
    }

    fn render_plugins(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("插件"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("本地工具已接入启动器，直接从搜索结果打开。"),
            )
            .child(feature_row(
                "Hosts 管理",
                "查看、检查并应用 Hosts Profile",
                "已接入",
            ))
            .child(feature_row("JSON 对比", "格式化并比较两份 JSON", "已接入"))
            .child(feature_row("网络监控", "查看网络诊断与连接测试", "已接入"))
            .child(feature_row("剪贴板", "读取并复用最近复制的文本", "已接入"))
            .child(
                div()
                    .rounded_md()
                    .bg(theme::hex(theme::PRIMARY))
                    .text_color(theme::hex(theme::SURFACE))
                    .px_3()
                    .py_2()
                    .text_sm()
                    .child("打开配置目录")
                    .on_mouse_down(MouseButton::Left, cx.listener(Self::open_config_dir)),
            )
    }

    fn render_launchers(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("启动项"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("启动器支持应用、文件夹和本地工具。"),
            )
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .border_t_1()
                    .border_color(theme::hex(theme::BORDER))
                    .pt_3()
                    .child(
                        div()
                            .flex()
                            .flex_col()
                            .gap_1()
                            .child(div().text_sm().child("开机启动"))
                            .child(
                                div()
                                    .text_xs()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("登录 Windows 后自动打开 Ariadne"),
                            ),
                    )
                    .child(
                        div()
                            .rounded_full()
                            .bg(theme::hex(if self.run_on_startup {
                                theme::SUCCESS
                            } else {
                                theme::BORDER_STRONG
                            }))
                            .p_1()
                            .child(
                                div()
                                    .w(px(28.))
                                    .h(px(16.))
                                    .rounded_full()
                                    .bg(theme::hex(theme::SURFACE))
                                    .when(self.run_on_startup, |this| this.ml(px(12.))),
                            )
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::toggle_startup)),
                    ),
            )
            .child(info_row("配置文件", &settings_path().display().to_string()))
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
                            .child("打开配置目录")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::open_config_dir)),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("重新加载")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::reload)),
                    ),
            )
    }

    fn render_screenshot(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        let screenshot = self.screenshot_hotkey.clone();
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("截图"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("Ariadne 截图覆盖层，快捷键在后台也可用。"),
            )
            .child(feature_row("截图快捷键", &screenshot, "已注册"))
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
                            .child("测试截图")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::test_screenshot)),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("复制快捷键")
                            .on_mouse_down(
                                MouseButton::Left,
                                cx.listener(move |this, event, window, cx| {
                                    this.copy_shortcut(&screenshot, event, window, cx)
                                }),
                            ),
                    ),
            )
    }

    fn render_search(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("搜索"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("默认搜索当前目录、用户目录和本地磁盘，按需启用索引服务。"),
            )
            .child(feature_row("文件索引", "本地磁盘搜索", "可用"))
            .child(feature_row(
                "搜索范围",
                "当前目录、用户目录及 A–Z 本地盘",
                "已覆盖",
            ))
            .child(feature_row("结果动作", "打开文件或文件夹", "已接入"))
            .child(div().text_sm().child("排除文件夹"))
            .child(Input::new(&self.exclude_folders).w_full())
            .child(
                div()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child("跳过这些目录及其内容；每行一项，也可用逗号分隔"),
            )
            .child(div().text_sm().child("排除模式"))
            .child(Input::new(&self.exclude_patterns).w_full())
            .child(
                div()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child("匹配文件名或路径的模式；每行一项，也可用逗号分隔"),
            )
            .child(div().text_sm().child("包含扩展名（可选）"))
            .child(Input::new(&self.include_extensions).w_full())
            .child(
                div()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child("这些扩展名可覆盖排除规则，例如 .md、.txt；留空表示不额外保留"),
            )
            .child(div().text_sm().child("排除扩展名"))
            .child(Input::new(&self.exclude_extensions).w_full())
            .child(
                div()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child("不索引这些扩展名，例如 .log、.tmp；每行一项，也可用逗号分隔"),
            )
            .child(
                div()
                    .rounded_md()
                    .bg(theme::hex(theme::PRIMARY))
                    .text_color(theme::hex(theme::SURFACE))
                    .px_3()
                    .py_2()
                    .text_sm()
                    .child("打开配置目录")
                    .on_mouse_down(MouseButton::Left, cx.listener(Self::open_config_dir)),
            )
    }

    fn render_data(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("数据与存储"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("设置保存在当前用户目录，不上传数据。"),
            )
            .child(info_row("配置文件", &settings_path().display().to_string()))
            .child(info_row(
                "当前工作目录",
                &std::env::current_dir()
                    .map(|p| p.display().to_string())
                    .unwrap_or_else(|_| "未知".into()),
            ))
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
                            .child("打开配置目录")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::open_config_dir)),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("重新加载")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::reload)),
                    ),
            )
    }
    fn render_diagnostics(&mut self, cx: &mut Context<Self>) -> impl IntoElement {
        let cwd = std::env::current_dir()
            .map(|path| path.display().to_string())
            .unwrap_or_else(|_| "未知".into());
        div()
            .flex_1()
            .flex()
            .flex_col()
            .gap_3()
            .rounded_lg()
            .border_1()
            .border_color(theme::hex(theme::BORDER))
            .bg(theme::hex(theme::SURFACE))
            .p_5()
            .child(div().text_lg().child("诊断"))
            .child(
                div()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child("查看运行环境并快速打开本地配置。"),
            )
            .child(info_row("进程 ID", &std::process::id().to_string()))
            .child(info_row("工作目录", &cwd))
            .child(info_row(
                "配置目录",
                &settings_path()
                    .parent()
                    .map(|p| p.display().to_string())
                    .unwrap_or_else(|| "未知".into()),
            ))
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
                            .child("打开配置目录")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::open_config_dir)),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("重新加载")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::reload)),
                    ),
            )
    }
}

impl Render for SettingsPage {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let panel = match self.tab {
            SettingsTab::General => self.render_general(cx).into_any_element(),
            SettingsTab::Shortcuts => self.render_shortcuts(cx).into_any_element(),
            SettingsTab::Plugins => self.render_plugins(cx).into_any_element(),
            SettingsTab::Launchers => self.render_launchers(cx).into_any_element(),
            SettingsTab::Screenshot => self.render_screenshot(cx).into_any_element(),
            SettingsTab::Search => self.render_search(cx).into_any_element(),
            SettingsTab::Data => self.render_data(cx).into_any_element(),
            SettingsTab::Advanced => self.render_diagnostics(cx).into_any_element(),
        };
        let general = SettingsTab::General;
        let shortcuts = SettingsTab::Shortcuts;
        let plugins = SettingsTab::Plugins;
        let launchers = SettingsTab::Launchers;
        let screenshot = SettingsTab::Screenshot;
        let search = SettingsTab::Search;
        let data = SettingsTab::Data;
        let advanced = SettingsTab::Advanced;
        div()
            .size_full()
            .flex()
            .flex_col()
            .gap_4()
            .p_6()
            .bg(theme::hex(theme::BACKGROUND))
            .text_color(theme::hex(theme::FOREGROUND))
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .child(
                        div()
                            .flex()
                            .flex_col()
                            .gap_1()
                            .child(div().text_xl().child("设置中心"))
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("亮石墨 · 本地优先"),
                            ),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("亮石墨"),
                    ),
            )
            .child(
                div()
                    .flex_1()
                    .flex()
                    .gap_4()
                    .child(
                        div()
                            .w(px(220.))
                            .flex()
                            .flex_col()
                            .gap_1()
                            .child(self.tab_button(general, "常规", cx))
                            .child(self.tab_button(shortcuts, "快捷键", cx))
                            .child(self.tab_button(plugins, "插件", cx))
                            .child(self.tab_button(launchers, "启动项", cx))
                            .child(self.tab_button(screenshot, "截图", cx))
                            .child(self.tab_button(search, "搜索", cx))
                            .child(self.tab_button(data, "数据与存储", cx))
                            .child(self.tab_button(advanced, "高级维护", cx)),
                    )
                    .child(
                        div()
                            .id("settings-panel")
                            .flex_1()
                            .min_h_0()
                            .overflow_y_scrollbar()
                            .child(panel),
                    ),
            )
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .text_sm()
                    .text_color(theme::hex(theme::MUTED))
                    .child(self.status.clone())
                    .child(
                        div()
                            .rounded_md()
                            .bg(theme::hex(theme::PRIMARY))
                            .text_color(theme::hex(theme::SURFACE))
                            .px_3()
                            .py_2()
                            .child("保存设置")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::save)),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_3()
                            .py_2()
                            .child("返回 Launcher")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::back)),
                    ),
            )
    }
}

fn shortcut_row(title: &str, shortcut: &str, detail: &str) -> impl IntoElement {
    div()
        .flex()
        .items_center()
        .justify_between()
        .border_t_1()
        .border_color(theme::hex(theme::BORDER))
        .pt_3()
        .child(
            div()
                .flex()
                .flex_col()
                .gap_1()
                .child(div().text_sm().child(title.to_owned()))
                .child(
                    div()
                        .text_xs()
                        .text_color(theme::hex(theme::MUTED))
                        .child(detail.to_owned()),
                ),
        )
        .child(
            div()
                .rounded_md()
                .border_1()
                .border_color(theme::hex(theme::BORDER))
                .px_3()
                .py_2()
                .text_sm()
                .child(shortcut.to_owned()),
        )
}

fn feature_row(title: &str, detail: &str, status: &str) -> impl IntoElement {
    div()
        .flex()
        .items_center()
        .justify_between()
        .border_t_1()
        .border_color(theme::hex(theme::BORDER))
        .pt_3()
        .child(
            div()
                .flex()
                .flex_col()
                .gap_1()
                .child(div().text_sm().child(title.to_owned()))
                .child(
                    div()
                        .text_xs()
                        .text_color(theme::hex(theme::MUTED))
                        .child(detail.to_owned()),
                ),
        )
        .child(
            div()
                .rounded_full()
                .bg(theme::hex(theme::SURFACE_SELECTED))
                .px_2()
                .py_1()
                .text_xs()
                .child(status.to_owned()),
        )
}
fn info_row(title: &str, value: &str) -> impl IntoElement {
    div()
        .flex()
        .justify_between()
        .border_t_1()
        .border_color(theme::hex(theme::BORDER))
        .pt_3()
        .child(div().text_sm().child(title.to_owned()))
        .child(
            div()
                .text_xs()
                .text_color(theme::hex(theme::MUTED))
                .child(value.to_owned()),
        )
}

fn set_startup(enabled: bool) -> Result<(), String> {
    if !cfg!(windows) {
        return Ok(());
    }
    let key = r"HKCU\Software\Microsoft\Windows\CurrentVersion\Run";
    let status = if enabled {
        let executable = std::env::current_exe().map_err(|error| error.to_string())?;
        let value = format!("\"{}\"", executable.display());
        Command::new("reg.exe")
            .args(["ADD", key, "/v", "Ariadne", "/t", "REG_SZ", "/d"])
            .arg(value)
            .arg("/f")
            .status()
    } else {
        Command::new("reg.exe")
            .args(["DELETE", key, "/v", "Ariadne", "/f"])
            .status()
    }
    .map_err(|error| error.to_string())?;
    if status.success() || !enabled {
        Ok(())
    } else {
        Err(format!("reg.exe 返回 {}", status))
    }
}
fn app_data_base() -> PathBuf {
    std::env::var_os("APPDATA")
        .map(PathBuf::from)
        .or_else(|| std::env::var_os("LOCALAPPDATA").map(PathBuf::from))
        .or_else(|| std::env::var_os("HOME").map(PathBuf::from))
        .unwrap_or_else(|| PathBuf::from("."))
}

fn settings_path() -> PathBuf {
    app_data_base().join("Ariadne").join("gpui_settings.json")
}

fn original_config_path() -> PathBuf {
    app_data_base().join("Ariadne").join("config.json")
}

fn legacy_config_path() -> PathBuf {
    app_data_base().join("x-tools").join("config.json")
}

fn original_sqlite_path() -> PathBuf {
    app_data_base().join("Ariadne").join("ariadne.sqlite")
}

fn read_json(path: PathBuf) -> Option<serde_json::Value> {
    let raw = fs::read_to_string(path).ok()?;
    serde_json::from_str(&raw).ok()
}

fn value_at<'a>(value: &'a serde_json::Value, path: &[&str]) -> Option<&'a serde_json::Value> {
    path.iter()
        .try_fold(value, |current, key| current.get(*key))
}

fn first_value<'a>(
    value: &'a serde_json::Value,
    paths: &[&[&str]],
) -> Option<&'a serde_json::Value> {
    paths.iter().find_map(|path| value_at(value, path))
}

fn string_at(value: &serde_json::Value, paths: &[&[&str]]) -> Option<String> {
    first_value(value, paths)
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|item| !item.is_empty())
        .map(ToOwned::to_owned)
}

fn bool_at(value: &serde_json::Value, paths: &[&[&str]]) -> Option<bool> {
    first_value(value, paths).and_then(serde_json::Value::as_bool)
}

fn list_value(value: Option<&serde_json::Value>) -> Vec<String> {
    let Some(value) = value else {
        return Vec::new();
    };
    if let Some(items) = value.as_array() {
        return clean_list(items.iter().filter_map(serde_json::Value::as_str));
    }
    value.as_str().map(parse_list).unwrap_or_default()
}

fn clean_list<'a>(items: impl IntoIterator<Item = &'a str>) -> Vec<String> {
    let mut result: Vec<String> = Vec::new();
    for item in items {
        let item = item.trim();
        if item.is_empty()
            || result
                .iter()
                .any(|existing| existing.eq_ignore_ascii_case(item))
        {
            continue;
        }
        result.push(item.to_owned());
    }
    result
}

fn parse_list(value: &str) -> Vec<String> {
    clean_list(value.split([',', '\n', '\r']))
}

fn search_exclusions(value: &serde_json::Value) -> SearchExclusions {
    let mut exclude_folders = Vec::new();
    let mut exclude_patterns = Vec::new();
    let mut include_extensions = Vec::new();
    let mut exclude_extensions = Vec::new();
    for search in [
        "search",
        "fileSearch",
        "file_search",
        "searchSettings",
        "search_settings",
    ]
    .into_iter()
    .filter_map(|key| value.get(key))
    {
        exclude_folders.extend(list_value(
            search
                .get("fileExcludeFolders")
                .or_else(|| search.get("excludeFolders"))
                .or_else(|| search.get("exclude_folders")),
        ));
        exclude_patterns.extend(list_value(
            search
                .get("fileExcludePatterns")
                .or_else(|| search.get("excludePatterns"))
                .or_else(|| search.get("exclude_patterns")),
        ));
        include_extensions.extend(list_value(
            search
                .get("fileIncludeExtensions")
                .or_else(|| search.get("includeExtensions"))
                .or_else(|| search.get("include_extensions")),
        ));
        exclude_extensions.extend(list_value(
            search
                .get("fileExcludeExtensions")
                .or_else(|| search.get("excludeExtensions"))
                .or_else(|| search.get("exclude_extensions")),
        ));
    }
    for work_memory in ["workMemory", "work_memory"]
        .into_iter()
        .filter_map(|key| value.get(key))
    {
        exclude_folders.extend(list_value(
            work_memory
                .get("excludePaths")
                .or_else(|| work_memory.get("exclude_paths")),
        ));
    }
    SearchExclusions {
        exclude_folders: clean_list(exclude_folders.iter().map(String::as_str)),
        exclude_patterns: clean_list(exclude_patterns.iter().map(String::as_str)),
        include_extensions: clean_list(include_extensions.iter().map(String::as_str)),
        exclude_extensions: clean_list(exclude_extensions.iter().map(String::as_str)),
    }
}

fn apply_config(settings: &mut SettingsValues, value: &serde_json::Value) {
    if let Some(run_on_startup) = bool_at(
        value,
        &[
            &["runOnStartup"],
            &["run_on_startup"],
            &["general", "runOnStartup"],
        ],
    ) {
        settings.run_on_startup = run_on_startup;
    }
    if let Some(global_hotkey) = string_at(
        value,
        &[
            &["globalHotkey"],
            &["hotkeys", "toggleWindow"],
            &["hotkeys", "toggle_window"],
        ],
    ) {
        settings.global_hotkey = global_hotkey;
    }
    if let Some(screenshot_hotkey) =
        string_at(value, &[&["screenshotHotkey"], &["hotkeys", "screenshot"]])
    {
        settings.screenshot_hotkey = screenshot_hotkey;
    }
    let search = search_exclusions(value);
    if !search_is_empty(&search) {
        merge_search_exclusions(&mut settings.search, &search);
    }
}

fn search_is_empty(search: &SearchExclusions) -> bool {
    search.exclude_folders.is_empty()
        && search.exclude_patterns.is_empty()
        && search.include_extensions.is_empty()
        && search.exclude_extensions.is_empty()
}

fn merge_search_exclusions(target: &mut SearchExclusions, source: &SearchExclusions) {
    target.exclude_folders = clean_list(
        target
            .exclude_folders
            .iter()
            .chain(source.exclude_folders.iter())
            .map(String::as_str),
    );
    target.exclude_patterns = clean_list(
        target
            .exclude_patterns
            .iter()
            .chain(source.exclude_patterns.iter())
            .map(String::as_str),
    );
    target.include_extensions = clean_list(
        target
            .include_extensions
            .iter()
            .chain(source.include_extensions.iter())
            .map(String::as_str),
    );
    target.exclude_extensions = clean_list(
        target
            .exclude_extensions
            .iter()
            .chain(source.exclude_extensions.iter())
            .map(String::as_str),
    );
}

fn read_sqlite_search_exclusions(path: PathBuf) -> Option<SearchExclusions> {
    let connection = Connection::open_with_flags(path, OpenFlags::SQLITE_OPEN_READ_ONLY).ok()?;
    read_sqlite_search_exclusions_from(&connection)
}

fn read_sqlite_search_exclusions_from(connection: &Connection) -> Option<SearchExclusions> {
    let mut search = SearchExclusions {
        exclude_folders: Vec::new(),
        exclude_patterns: Vec::new(),
        include_extensions: Vec::new(),
        exclude_extensions: Vec::new(),
    };
    let mut statement = connection
        .prepare(
            "SELECT path, value \
             FROM settings2_string_lists \
             WHERE scope IN ('config', 'any') \
               AND path IN (\
                 'search.fileExcludeFolders',\
                 'search.fileExcludePatterns',\
                 'search.fileIncludeExtensions',\
                 'search.fileExcludeExtensions'\
               ) \
             ORDER BY path, position",
        )
        .ok()?;
    let rows = statement
        .query_map([], |row| {
            Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
        })
        .ok()?;
    for (path, value) in rows.flatten() {
        match path.as_str() {
            "search.fileExcludeFolders" => search.exclude_folders.push(value),
            "search.fileExcludePatterns" => search.exclude_patterns.push(value),
            "search.fileIncludeExtensions" => search.include_extensions.push(value),
            "search.fileExcludeExtensions" => search.exclude_extensions.push(value),
            _ => {}
        }
    }
    search.exclude_folders = clean_list(search.exclude_folders.iter().map(String::as_str));
    search.exclude_patterns = clean_list(search.exclude_patterns.iter().map(String::as_str));
    search.include_extensions = clean_list(search.include_extensions.iter().map(String::as_str));
    search.exclude_extensions = clean_list(search.exclude_extensions.iter().map(String::as_str));
    Some(search)
}

fn load_settings() -> SettingsValues {
    let mut settings = SettingsValues::default();
    if let Some(value) = read_json(settings_path()) {
        apply_config(&mut settings, &value);
        let raw_search = search_exclusions(&value);
        let current_search = settings.search.clone();
        let mut merged = current_search.clone();
        for path in [original_config_path(), legacy_config_path()] {
            let Some(source) = read_json(path) else {
                continue;
            };
            let source_search = search_exclusions(&source);
            merge_search_exclusions(&mut merged, &source_search);
        }
        if let Some(source_search) = read_sqlite_search_exclusions(original_sqlite_path()) {
            merge_search_exclusions(&mut merged, &source_search);
        }
        let defaults_missing = raw_search.exclude_folders.len() < merged.exclude_folders.len()
            || raw_search.exclude_patterns.len() < merged.exclude_patterns.len()
            || raw_search.include_extensions.len() < merged.include_extensions.len()
            || raw_search.exclude_extensions.len() < merged.exclude_extensions.len();
        if merged != current_search || defaults_missing {
            settings.search = merged;
            let _ = save_settings(
                settings.run_on_startup,
                &settings.global_hotkey,
                &settings.screenshot_hotkey,
                &settings.search,
            );
        }
        return settings;
    }
    let mut imported = false;
    let mut migrated = SearchExclusions::default();
    for path in [original_config_path(), legacy_config_path()] {
        let Some(value) = read_json(path) else {
            continue;
        };
        if !imported {
            apply_config(&mut settings, &value);
            imported = true;
        }
        let source_search = search_exclusions(&value);
        merge_search_exclusions(&mut migrated, &source_search);
    }
    if let Some(source_search) = read_sqlite_search_exclusions(original_sqlite_path()) {
        if !search_is_empty(&source_search) {
            merge_search_exclusions(&mut migrated, &source_search);
            imported = true;
        }
    }
    if imported {
        if !search_is_empty(&migrated) {
            settings.search = migrated;
        }
        let _ = save_settings(
            settings.run_on_startup,
            &settings.global_hotkey,
            &settings.screenshot_hotkey,
            &settings.search,
        );
    }
    settings
}

fn save_settings(
    run_on_startup: bool,
    global_hotkey: &str,
    screenshot_hotkey: &str,
    search: &SearchExclusions,
) -> Result<(), String> {
    let path = settings_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|error| error.to_string())?;
    }
    let value = json!({
        "runOnStartup": run_on_startup,
        "globalHotkey": global_hotkey,
        "screenshotHotkey": screenshot_hotkey,
        "search": {
            "fileExcludeFolders": search.exclude_folders,
            "fileExcludePatterns": search.exclude_patterns,
            "fileIncludeExtensions": search.include_extensions,
            "fileExcludeExtensions": search.exclude_extensions,
        },
    });
    let result = fs::write(
        path,
        serde_json::to_string_pretty(&value).map_err(|error| error.to_string())?,
    )
    .map_err(|error| error.to_string());
    if result.is_ok() {
        clear_search_cache();
    }
    result
}

fn search_cache() -> &'static Mutex<Option<SearchExclusions>> {
    static CACHE: OnceLock<Mutex<Option<SearchExclusions>>> = OnceLock::new();
    CACHE.get_or_init(|| Mutex::new(None))
}

fn clear_search_cache() {
    if let Ok(mut cache) = search_cache().lock() {
        *cache = None;
    }
}

pub(crate) fn load_search_exclusions() -> SearchExclusions {
    if let Ok(cache) = search_cache().lock() {
        if let Some(search) = cache.clone() {
            return search;
        }
    }
    let search = load_settings().search;
    if let Ok(mut cache) = search_cache().lock() {
        *cache = Some(search.clone());
    }
    search
}

pub(crate) fn configured_global_hotkey() -> String {
    load_settings().global_hotkey
}

pub(crate) fn configured_screenshot_hotkey() -> String {
    load_settings().screenshot_hotkey
}

#[cfg(test)]
mod tests {
    use super::{SettingsValues, apply_config, parse_list, read_sqlite_search_exclusions_from};
    use rusqlite::Connection;
    use serde_json::json;

    #[test]
    fn legacy_config_migrates_hotkeys_and_exclusion_paths() {
        let mut settings = SettingsValues::default();
        apply_config(
            &mut settings,
            &json!({
                "run_on_startup": true,
                "hotkeys": {"toggle_window": "Alt+W", "screenshot": "Alt+A"},
                "search": {
                    "fileExcludeFolders": ["C:/Cache"],
                    "fileExcludePatterns": ["*.tmp"],
                    "fileIncludeExtensions": [".md"],
                    "fileExcludeExtensions": [".log"]
                },
                "work_memory": {"exclude_paths": [" C:/Private ", "c:/private"]}
            }),
        );
        assert!(settings.run_on_startup);
        assert_eq!(settings.global_hotkey, "Alt+W");
        assert_eq!(settings.screenshot_hotkey, "Alt+A");
        assert!(
            settings
                .search
                .exclude_folders
                .iter()
                .any(|item| item == "C:/Cache")
        );
        assert!(
            settings
                .search
                .exclude_folders
                .iter()
                .any(|item| item == "C:/Private")
        );
        assert!(
            settings
                .search
                .exclude_patterns
                .iter()
                .any(|item| item == "*.tmp")
        );
        assert_eq!(settings.search.include_extensions, vec![".md".to_owned()]);
        assert!(
            settings
                .search
                .exclude_extensions
                .iter()
                .any(|item| item == ".log")
        );
        assert!(
            settings
                .search
                .exclude_extensions
                .iter()
                .any(|item| item == ".tmp")
        );
        assert_eq!(parse_list("a, b\nc"), ["a", "b", "c"]);
    }

    #[test]
    fn default_search_policy_matches_frontend_shape() {
        let search = SettingsValues::default().search;
        assert_eq!(search.exclude_folders.len(), 14);
        assert_eq!(search.exclude_patterns.len(), 24);
        assert!(search.include_extensions.is_empty());
        assert_eq!(search.exclude_extensions.len(), 16);
        assert!(search.exclude_extensions.iter().any(|item| item == ".log"));
    }

    #[test]
    fn sqlite_search_settings_are_migrated() {
        let connection = Connection::open_in_memory().expect("open sqlite");
        connection
            .execute_batch(
                r#"CREATE TABLE settings2_string_lists(
                    scope TEXT NOT NULL,
                    path TEXT NOT NULL,
                    position INTEGER NOT NULL,
                    value TEXT NOT NULL,
                    PRIMARY KEY(scope, path, position)
                );
                INSERT INTO settings2_string_lists VALUES
                    ('config', 'search.fileExcludeExtensions', 0, '.bak'),
                    ('config', 'search.fileExcludeExtensions', 1, '.BAK'),
                    ('config', 'search.fileExcludeFolders', 0, ' P:/workspace '),
                    ('config', 'search.fileExcludePatterns', 0, '(?i)\\.tmp$'),
                    ('other', 'search.fileExcludeFolders', 0, 'ignored');"#,
            )
            .expect("seed sqlite");
        let search = read_sqlite_search_exclusions_from(&connection).expect("read sqlite");
        assert_eq!(search.exclude_extensions, vec![".bak"]);
        assert_eq!(search.exclude_folders, vec!["P:/workspace"]);
        assert_eq!(search.exclude_patterns, vec![r"(?i)\\.tmp$"]);
    }
}
