use std::{fs, path::PathBuf};

use gpui::{
    ClipboardItem, Context, MouseButton, MouseDownEvent, SharedString, Window, div, prelude::*,
};
use serde_json::Value;

use crate::theme;

pub struct ClipboardPage {
    entries: Vec<String>,
    history_path: PathBuf,
    feedback: SharedString,
}

impl ClipboardPage {
    pub fn new() -> Self {
        let history_path = clipboard_history_path();
        let entries = load_history(&history_path);
        Self {
            entries,
            history_path,
            feedback: "点击读取当前剪贴板".into(),
        }
    }

    fn back(&mut self, _: &MouseDownEvent, window: &mut Window, _: &mut Context<Self>) {
        window.remove_window();
    }

    fn refresh(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        if let Some(item) = cx.read_from_clipboard() {
            if let Some(text) = item.text() {
                let text = text.to_string();
                self.entries.retain(|entry| entry != &text);
                self.entries.insert(0, text);
                self.entries.truncate(20);
                self.feedback = match save_history(&self.history_path, &self.entries) {
                    Ok(()) => "已读取当前剪贴板".into(),
                    Err(error) => format!("读取成功，但历史保存失败：{error}").into(),
                };
            } else {
                self.feedback = "当前剪贴板不是文本".into();
            }
        } else {
            self.feedback = "无法读取当前剪贴板".into();
        }
        cx.notify();
    }

    fn copy(&mut self, index: usize, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        if let Some(text) = self.entries.get(index) {
            cx.write_to_clipboard(ClipboardItem::new_string(text.clone()));
            self.feedback = "已复制".into();
            cx.notify();
        }
    }
}

impl Render for ClipboardPage {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
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
                            .child(div().text_xl().child("剪贴板历史"))
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("本地文本 · 重新复制"),
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
                            .child("返回 Launcher")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::back)),
                    ),
            )
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .child(
                        div()
                            .text_sm()
                            .text_color(theme::hex(theme::MUTED))
                            .child(self.feedback.clone()),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .bg(theme::hex(theme::PRIMARY))
                            .text_color(theme::hex(theme::SURFACE))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("读取当前剪贴板")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::refresh)),
                    ),
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
                    .when(self.entries.is_empty(), |this| {
                        this.child(
                            div()
                                .flex_1()
                                .flex()
                                .items_center()
                                .justify_center()
                                .text_sm()
                                .text_color(theme::hex(theme::MUTED))
                                .child("还没有读取到文本"),
                        )
                    })
                    .children(self.entries.iter().enumerate().map(|(index, text)| {
                        div()
                            .flex()
                            .items_center()
                            .justify_between()
                            .gap_3()
                            .border_b_1()
                            .border_color(theme::hex(theme::BORDER))
                            .py_3()
                            .child(div().flex_1().text_sm().child(text.clone()))
                            .child(
                                div()
                                    .rounded_md()
                                    .border_1()
                                    .border_color(theme::hex(theme::BORDER))
                                    .px_2()
                                    .py_1()
                                    .text_xs()
                                    .child("复制")
                                    .on_mouse_down(
                                        MouseButton::Left,
                                        cx.listener(
                                            move |this, event: &MouseDownEvent, window, cx| {
                                                this.copy(index, event, window, cx)
                                            },
                                        ),
                                    ),
                            )
                    })),
            )
    }
}

fn clipboard_history_path() -> PathBuf {
    let base = std::env::var_os("APPDATA")
        .map(PathBuf::from)
        .or_else(|| std::env::var_os("LOCALAPPDATA").map(PathBuf::from))
        .or_else(|| std::env::var_os("HOME").map(PathBuf::from))
        .unwrap_or_else(|| PathBuf::from("."));
    base.join("Ariadne").join("gpui_clipboard_history.json")
}

fn load_history(path: &PathBuf) -> Vec<String> {
    let Ok(raw) = fs::read_to_string(path) else {
        return Vec::new();
    };
    let Ok(Value::Array(items)) = serde_json::from_str::<Value>(&raw) else {
        return Vec::new();
    };
    items
        .into_iter()
        .filter_map(|item| item.as_str().map(str::to_owned))
        .filter(|item| !item.is_empty())
        .take(20)
        .collect()
}

fn save_history(path: &PathBuf, entries: &[String]) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|error| error.to_string())?;
    }
    let raw = serde_json::to_string_pretty(entries).map_err(|error| error.to_string())?;
    fs::write(path, raw).map_err(|error| error.to_string())
}
