use std::{collections::BTreeMap, fs, net::IpAddr, path::PathBuf};

use gpui::{
    Context, Entity, MouseButton, MouseDownEvent, SharedString, Subscription, Window, div,
    prelude::*, px,
};
use gpui_component::input::{Textarea, TextareaState};
use serde_json::{Value, json};

use crate::theme;

struct Profile {
    title: String,
    enabled: bool,
    content: String,
    system: bool,
}

pub struct HostsPage {
    profiles: Vec<Profile>,
    selected: usize,
    editor: Entity<TextareaState>,
    feedback: SharedString,
    preview: bool,
    apply_armed: bool,
    _subscriptions: Vec<Subscription>,
}

impl HostsPage {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> Self {
        let editor = cx.new(|cx| TextareaState::new(window, cx));
        let profiles = load_profiles();
        let value = profiles
            .first()
            .map(|profile| profile.content.clone())
            .unwrap_or_default();
        editor.update(cx, |state, cx| state.set_value(value, window, cx));
        let mut page = Self {
            profiles,
            selected: 0,
            editor: editor.clone(),
            feedback: "方案保存在当前用户配置".into(),
            preview: false,
            apply_armed: false,
            _subscriptions: Vec::new(),
        };
        page._subscriptions.push(cx.subscribe_in(
            &editor,
            window,
            move |page, editor, event, _, cx| {
                if matches!(event, gpui_component::input::InputEvent::Change) {
                    if let Some(profile) = page.profiles.get_mut(page.selected) {
                        if !profile.system {
                            profile.content = editor.read(cx).value().to_string();
                            page.preview = false;
                            page.apply_armed = false;
                        }
                    }
                    cx.notify();
                }
            },
        ));
        page
    }

    fn back(&mut self, _: &MouseDownEvent, window: &mut Window, _: &mut Context<Self>) {
        window.remove_window();
    }

    fn select(
        &mut self,
        index: usize,
        _: &MouseDownEvent,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        if index >= self.profiles.len() {
            return;
        }
        self.selected = index;
        let value = self.profiles[index].content.clone();
        self.editor
            .update(cx, |state, cx| state.set_value(value, window, cx));
        self.preview = false;
        self.apply_armed = false;
        cx.notify();
    }

    fn toggle(&mut self, index: usize, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        if let Some(profile) = self.profiles.get_mut(index) {
            if profile.system {
                return;
            }
            profile.enabled = !profile.enabled;
            self.preview = false;
            self.apply_armed = false;
            self.feedback = format!(
                "{}：{}",
                profile.title,
                if profile.enabled {
                    "已启用"
                } else {
                    "已停用"
                }
            )
            .into();
            if let Err(error) = save_profiles(&self.profiles) {
                self.feedback = format!("保存失败：{error}").into();
            }
            cx.notify();
        }
    }

    fn new_profile(&mut self, _: &MouseDownEvent, window: &mut Window, cx: &mut Context<Self>) {
        let title = format!("新建方案 {}", self.profiles.len());
        self.profiles.push(Profile {
            title,
            enabled: false,
            content: String::new(),
            system: false,
        });
        self.selected = self.profiles.len() - 1;
        self.editor
            .update(cx, |state, cx| state.set_value("", window, cx));
        self.preview = false;
        self.apply_armed = false;
        self.feedback = "已新建方案".into();
        if let Err(error) = save_profiles(&self.profiles) {
            self.feedback = format!("保存失败：{error}").into();
        }
        cx.notify();
    }

    fn save(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        self.feedback = match save_profiles(&self.profiles) {
            Ok(()) => "方案已保存".into(),
            Err(error) => format!("保存失败：{error}").into(),
        };
        cx.notify();
    }

    fn build_preview(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        let final_content = self.final_content();
        let conflicts = host_conflicts(&final_content);
        self.preview = true;
        self.apply_armed = false;
        self.feedback = if conflicts == 0 {
            "预览已生成，未写入系统 Hosts".into()
        } else {
            format!("预览已生成 · 发现 {conflicts} 个冲突域名").into()
        };
        cx.notify();
    }

    fn apply(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        if !self.apply_armed {
            let final_content = self.final_content();
            let conflicts = host_conflicts(&final_content);
            self.preview = true;
            self.apply_armed = true;
            self.feedback = if conflicts == 0 {
                "已生成预览，请再次点击确认写入".into()
            } else {
                format!("发现 {conflicts} 个冲突域名，请再次点击确认写入").into()
            };
            cx.notify();
            return;
        }
        let path = system_hosts_path();
        match fs::write(&path, self.final_content()) {
            Ok(()) => {
                if let Some(system) = self.profiles.first_mut() {
                    system.content = fs::read_to_string(&path).unwrap_or_default();
                }
                self.apply_armed = false;
                self.preview = true;
                self.feedback = "已写入系统 Hosts".into();
            }
            Err(error) => {
                self.apply_armed = false;
                self.feedback = format!("写入失败：{error}").into();
            }
        }
        cx.notify();
    }

    fn final_content(&self) -> String {
        let base = self
            .profiles
            .first()
            .map(|profile| strip_managed_block(&profile.content))
            .unwrap_or_default()
            .trim()
            .to_string();
        let enabled = self
            .profiles
            .iter()
            .filter(|profile| {
                !profile.system && profile.enabled && !profile.content.trim().is_empty()
            })
            .map(|profile| {
                format!(
                    "# --- Profile: {} ---\n{}",
                    profile.title,
                    profile.content.trim()
                )
            })
            .collect::<Vec<_>>();
        if enabled.is_empty() {
            return base;
        }
        let mut final_content = base;
        if !final_content.is_empty() {
            final_content.push_str("\n\n");
        }
        final_content.push_str("# --- Ariadne managed hosts ---\n");
        final_content.push_str(&enabled.join("\n\n"));
        final_content.push_str("\n# --- End Ariadne managed hosts ---");
        final_content
    }

    fn stats(&self) -> (usize, usize, usize, usize) {
        let text = self
            .profiles
            .get(self.selected)
            .map(|profile| profile.content.as_str())
            .unwrap_or_default();
        let total = text.lines().count();
        let comments = text
            .lines()
            .filter(|line| line.trim_start().starts_with('#'))
            .count();
        let empty = text.lines().filter(|line| line.trim().is_empty()).count();
        let rules = total.saturating_sub(comments + empty);
        (total, rules, comments, empty)
    }
}

impl Render for HostsPage {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let (total, rules, comments, empty) = self.stats();
        div()
            .size_full()
            .flex()
            .flex_col()
            .gap_3()
            .p_5()
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
                            .child(div().text_xl().child("Hosts 管理"))
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("本地方案 · 预览 · 冲突检查"),
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
                                    .child("新建方案")
                                    .on_mouse_down(
                                        MouseButton::Left,
                                        cx.listener(Self::new_profile),
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
                                    .child(if self.apply_armed {
                                        "确认写入"
                                    } else {
                                        "应用到系统"
                                    })
                                    .on_mouse_down(MouseButton::Left, cx.listener(Self::apply)),
                            ),
                    ),
            )
            .child(
                div()
                    .flex_1()
                    .flex()
                    .gap_3()
                    .child(
                        div()
                            .w(px(220.))
                            .flex()
                            .flex_col()
                            .rounded_lg()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .bg(theme::hex(theme::SURFACE))
                            .p_2()
                            .child(div().px_2().py_2().text_sm().child("方案列表"))
                            .children(self.profiles.iter().enumerate().map(|(index, profile)| {
                                let selected = index == self.selected;
                                div()
                                    .flex()
                                    .items_center()
                                    .justify_between()
                                    .rounded_md()
                                    .bg(theme::hex(if selected {
                                        theme::SURFACE_SELECTED
                                    } else {
                                        theme::SURFACE
                                    }))
                                    .px_2()
                                    .py_2()
                                    .child(div().text_sm().child(profile.title.clone()))
                                    .child(
                                        div()
                                            .w(px(10.))
                                            .h(px(10.))
                                            .rounded_full()
                                            .bg(theme::hex(if profile.enabled {
                                                theme::SUCCESS
                                            } else {
                                                theme::BORDER_STRONG
                                            }))
                                            .on_mouse_down(
                                                MouseButton::Left,
                                                cx.listener(move |this, event, window, cx| {
                                                    this.toggle(index, event, window, cx)
                                                }),
                                            ),
                                    )
                                    .on_mouse_down(
                                        MouseButton::Left,
                                        cx.listener(
                                            move |this, event: &MouseDownEvent, window, cx| {
                                                this.select(index, event, window, cx)
                                            },
                                        ),
                                    )
                            })),
                    )
                    .child(
                        div()
                            .flex_1()
                            .flex()
                            .flex_col()
                            .gap_2()
                            .rounded_lg()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .bg(theme::hex(theme::SURFACE))
                            .p_3()
                            .child(
                                div()
                                    .flex()
                                    .items_center()
                                    .justify_between()
                                    .child(div().text_sm().child("方案编辑"))
                                    .child(
                                        div()
                                            .text_xs()
                                            .text_color(theme::hex(theme::MUTED))
                                            .child("修改会立即更新预览"),
                                    ),
                            )
                            .child(
                                Textarea::new(&self.editor)
                                    .h_full()
                                    .flex_1()
                                    .aria_label("Hosts 内容"),
                            ),
                    )
                    .child(
                        div()
                            .w(px(260.))
                            .flex()
                            .flex_col()
                            .gap_3()
                            .rounded_lg()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .bg(theme::hex(theme::SURFACE))
                            .p_3()
                            .child(div().text_sm().child("方案信息"))
                            .child(
                                div().flex().flex_col().gap_2().text_sm().children([
                                    div().flex().justify_between().child("状态").child(
                                        if self.profiles[self.selected].enabled {
                                            "已启用"
                                        } else {
                                            "已停用"
                                        },
                                    ),
                                    div()
                                        .flex()
                                        .justify_between()
                                        .child("总行数")
                                        .child(total.to_string()),
                                    div()
                                        .flex()
                                        .justify_between()
                                        .child("有效规则")
                                        .child(rules.to_string()),
                                    div()
                                        .flex()
                                        .justify_between()
                                        .child("注释行")
                                        .child(comments.to_string()),
                                    div()
                                        .flex()
                                        .justify_between()
                                        .child("空行")
                                        .child(empty.to_string()),
                                ]),
                            )
                            .child(if self.preview {
                                div()
                                    .rounded_md()
                                    .bg(theme::hex(theme::SURFACE_SUBTLE))
                                    .p_2()
                                    .text_xs()
                                    .text_color(theme::hex(theme::SUCCESS))
                                    .child("预览有效：未写入系统 Hosts")
                            } else {
                                div()
                                    .text_xs()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("生成预览后查看写入结果")
                            })
                            .child(
                                div()
                                    .flex()
                                    .gap_2()
                                    .child(
                                        div()
                                            .rounded_md()
                                            .border_1()
                                            .border_color(theme::hex(theme::BORDER))
                                            .px_2()
                                            .py_2()
                                            .text_xs()
                                            .child("生成预览")
                                            .on_mouse_down(
                                                MouseButton::Left,
                                                cx.listener(Self::build_preview),
                                            ),
                                    )
                                    .child(
                                        div()
                                            .rounded_md()
                                            .bg(theme::hex(theme::PRIMARY))
                                            .text_color(theme::hex(theme::SURFACE))
                                            .px_2()
                                            .py_2()
                                            .text_xs()
                                            .child("保存方案")
                                            .on_mouse_down(
                                                MouseButton::Left,
                                                cx.listener(Self::save),
                                            ),
                                    ),
                            ),
                    ),
            )
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child(self.feedback.clone())
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_2()
                            .py_1()
                            .child("返回 Launcher")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::back)),
                    ),
            )
    }
}

fn app_config_path(file: &str) -> PathBuf {
    let base = std::env::var_os("APPDATA")
        .map(PathBuf::from)
        .or_else(|| std::env::var_os("LOCALAPPDATA").map(PathBuf::from))
        .or_else(|| std::env::var_os("HOME").map(PathBuf::from))
        .unwrap_or_else(|| PathBuf::from("."));
    base.join("Ariadne").join(file)
}

fn system_hosts_path() -> PathBuf {
    if cfg!(windows) {
        std::env::var_os("SystemRoot")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\\Windows"))
            .join(r"System32\drivers\etc\hosts")
    } else {
        PathBuf::from("/etc/hosts")
    }
}

fn load_profiles() -> Vec<Profile> {
    let system_path = system_hosts_path();
    let system_content = fs::read_to_string(&system_path)
        .unwrap_or_else(|_| "# 系统 Hosts\n127.0.0.1 localhost\n::1 localhost".into());
    let mut profiles = vec![Profile {
        title: "系统 Hosts".into(),
        enabled: true,
        content: system_content,
        system: true,
    }];
    let path = app_config_path("gpui_hosts.json");
    if let Ok(raw) = fs::read_to_string(path) {
        if let Ok(Value::Array(items)) = serde_json::from_str::<Value>(&raw) {
            profiles.extend(items.into_iter().filter_map(|item| {
                let object = item.as_object()?;
                let title = object.get("title")?.as_str()?.trim();
                if title.is_empty() {
                    return None;
                }
                Some(Profile {
                    title: title.into(),
                    enabled: object
                        .get("enabled")
                        .and_then(Value::as_bool)
                        .unwrap_or(false),
                    content: object
                        .get("content")
                        .and_then(Value::as_str)
                        .unwrap_or_default()
                        .into(),
                    system: false,
                })
            }));
        }
    }
    if profiles.len() == 1 {
        profiles.extend([
            Profile {
                title: "开发环境".into(),
                enabled: false,
                content: "# 开发环境\n127.0.0.1 api.local\n127.0.0.1 admin.local".into(),
                system: false,
            },
            Profile {
                title: "测试环境".into(),
                enabled: false,
                content: "# 测试环境\n10.0.0.12 api.test.local\n10.0.0.13 admin.test.local".into(),
                system: false,
            },
        ]);
    }
    profiles
}

fn save_profiles(profiles: &[Profile]) -> Result<(), String> {
    let path = app_config_path("gpui_hosts.json");
    let items = profiles
        .iter()
        .filter(|profile| !profile.system)
        .map(|profile| {
            json!({
                "title": profile.title,
                "enabled": profile.enabled,
                "content": profile.content,
            })
        })
        .collect::<Vec<_>>();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|error| error.to_string())?;
    }
    let raw = serde_json::to_string_pretty(&items).map_err(|error| error.to_string())?;
    fs::write(path, raw).map_err(|error| error.to_string())
}

fn strip_managed_block(content: &str) -> String {
    let start = "# --- Ariadne managed hosts ---";
    let end = "# --- End Ariadne managed hosts ---";
    let Some(start_index) = content.find(start) else {
        return content.to_string();
    };
    let Some(end_offset) = content[start_index..].find(end) else {
        return content.to_string();
    };
    let end_index = start_index + end_offset + end.len();
    let mut result = String::new();
    result.push_str(content[..start_index].trim_end());
    result.push_str(content[end_index..].trim_start());
    result
}

fn host_conflicts(content: &str) -> usize {
    let mut hosts = BTreeMap::<String, Vec<String>>::new();
    for line in content.lines() {
        let line = line.split('#').next().unwrap_or_default();
        let mut fields = line.split_whitespace();
        let Some(ip) = fields.next() else {
            continue;
        };
        if ip.parse::<IpAddr>().is_err() {
            continue;
        }
        for host in fields {
            let values = hosts.entry(host.to_ascii_lowercase()).or_default();
            if !values.iter().any(|value| value == ip) {
                values.push(ip.into());
            }
        }
    }
    hosts.values().filter(|ips| ips.len() > 1).count()
}
