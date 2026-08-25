use gpui::{
    Context, Entity, MouseButton, MouseDownEvent, SharedString, Subscription, Window, div,
    prelude::*, px,
};
use gpui_component::input::{InputEvent, Textarea, TextareaState};
use serde_json::Value;

use crate::theme;

pub struct JsonComparePage {
    left: Entity<TextareaState>,
    right: Entity<TextareaState>,
    left_formatted: String,
    right_formatted: String,
    summary: SharedString,
    report: SharedString,
    _subscriptions: Vec<Subscription>,
}

impl JsonComparePage {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> Self {
        let left = cx.new(|cx| TextareaState::new(window, cx));
        let right = cx.new(|cx| TextareaState::new(window, cx));
        left.update(cx, |state, cx| {
            state.set_value(
                "{\n  \"name\": \"Ariadne\",\n  \"version\": 1\n}",
                window,
                cx,
            )
        });
        right.update(cx, |state, cx| {
            state.set_value(
                "{\n  \"name\": \"Ariadne\",\n  \"version\": 2\n}",
                window,
                cx,
            )
        });
        let mut page = Self {
            left: left.clone(),
            right: right.clone(),
            left_formatted: String::new(),
            right_formatted: String::new(),
            summary: "输入两侧 JSON 后开始对比".into(),
            report: "暂无报告".into(),
            _subscriptions: Vec::new(),
        };
        page._subscriptions.push(cx.subscribe_in(
            &left,
            window,
            move |page, state, event, _, cx| {
                if matches!(event, InputEvent::Change) {
                    page.left_formatted = format_json(&state.read(cx).value());
                    page.compare_now(cx);
                }
            },
        ));
        page._subscriptions.push(cx.subscribe_in(
            &right,
            window,
            move |page, state, event, _, cx| {
                if matches!(event, InputEvent::Change) {
                    page.right_formatted = format_json(&state.read(cx).value());
                    page.compare_now(cx);
                }
            },
        ));
        page.left_formatted = format_json(&page.left.read(cx).value());
        page.right_formatted = format_json(&page.right.read(cx).value());
        page.compare_now(cx);
        page
    }

    fn back(&mut self, _: &MouseDownEvent, window: &mut Window, _: &mut Context<Self>) {
        window.remove_window();
    }

    fn compare(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        self.left_formatted = format_json(&self.left.read(cx).value());
        self.right_formatted = format_json(&self.right.read(cx).value());
        self.compare_now(cx);
    }

    fn format_side(
        &mut self,
        side: bool,
        _: &MouseDownEvent,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        let state = if side {
            self.left.clone()
        } else {
            self.right.clone()
        };
        let value = format_json(&state.read(cx).value());
        state.update(cx, |state, cx| state.set_value(value, window, cx));
        cx.notify();
    }

    fn compare_now(&mut self, cx: &mut Context<Self>) {
        let left_text = self.left.read(cx).value().to_string();
        let right_text = self.right.read(cx).value().to_string();
        let left_value = serde_json::from_str::<Value>(&left_text);
        let right_value = serde_json::from_str::<Value>(&right_text);
        self.left_formatted = format_json(&left_text);
        self.right_formatted = format_json(&right_text);
        if left_value.is_err() || right_value.is_err() {
            self.summary = "解析失败".into();
            self.report = format!(
                "左侧：{}\n右侧：{}",
                left_value
                    .as_ref()
                    .err()
                    .map(|error| error.to_string())
                    .unwrap_or_else(|| "有效 JSON".into()),
                right_value
                    .as_ref()
                    .err()
                    .map(|error| error.to_string())
                    .unwrap_or_else(|| "有效 JSON".into())
            )
            .into();
        } else if left_value.as_ref().ok() == right_value.as_ref().ok() {
            self.summary = "语义一致".into();
            self.report = "规范化后的两侧内容完全一致。".into();
        } else {
            let mut differences = Vec::new();
            diff_values(
                left_value.as_ref().expect("checked above"),
                right_value.as_ref().expect("checked above"),
                "$",
                &mut differences,
            );
            let count = differences.len();
            self.summary = format!("存在差异 · {} 处", count).into();
            if differences.len() > 80 {
                differences.truncate(80);
                differences.push("… 其余差异已省略".into());
            }
            self.report =
                format!("发现 {} 处语义差异。\n\n{}", count, differences.join("\n")).into();
        }
        cx.notify();
    }
}

impl Render for JsonComparePage {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
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
                            .child(div().text_xl().child("JSON 对比"))
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("本地格式化 · 规范化差异 · 复制结果"),
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
                    .gap_2()
                    .child(
                        div()
                            .rounded_md()
                            .bg(theme::hex(theme::PRIMARY))
                            .text_color(theme::hex(theme::SURFACE))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("对比")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::compare)),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("格式化左侧")
                            .on_mouse_down(
                                MouseButton::Left,
                                cx.listener(|this, event: &MouseDownEvent, window, cx| {
                                    this.format_side(true, event, window, cx)
                                }),
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
                            .child("格式化右侧")
                            .on_mouse_down(
                                MouseButton::Left,
                                cx.listener(|this, event: &MouseDownEvent, window, cx| {
                                    this.format_side(false, event, window, cx)
                                }),
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
                            .flex_1()
                            .flex()
                            .flex_col()
                            .gap_2()
                            .child(div().text_sm().child("左侧 JSON"))
                            .child(
                                Textarea::new(&self.left)
                                    .h_full()
                                    .flex_1()
                                    .aria_label("左侧 JSON"),
                            ),
                    )
                    .child(
                        div()
                            .flex_1()
                            .flex()
                            .flex_col()
                            .gap_2()
                            .child(div().text_sm().child("右侧 JSON"))
                            .child(
                                Textarea::new(&self.right)
                                    .h_full()
                                    .flex_1()
                                    .aria_label("右侧 JSON"),
                            ),
                    )
                    .child(
                        div()
                            .w(px(300.))
                            .flex()
                            .flex_col()
                            .gap_2()
                            .rounded_lg()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .bg(theme::hex(theme::SURFACE))
                            .p_3()
                            .child(div().text_sm().child(self.summary.clone()))
                            .child(
                                div()
                                    .text_xs()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child(self.report.clone()),
                            )
                            .child(
                                div()
                                    .flex()
                                    .flex_col()
                                    .gap_2()
                                    .rounded_md()
                                    .bg(theme::hex(theme::SURFACE_SUBTLE))
                                    .p_2()
                                    .child(div().text_xs().child("LEFT NORMALIZED"))
                                    .child(
                                        div()
                                            .text_xs()
                                            .text_color(theme::hex(theme::MUTED))
                                            .child(self.left_formatted.clone()),
                                    )
                                    .child(div().text_xs().child("RIGHT NORMALIZED"))
                                    .child(
                                        div()
                                            .text_xs()
                                            .text_color(theme::hex(theme::MUTED))
                                            .child(self.right_formatted.clone()),
                                    ),
                            ),
                    ),
            )
            .child(
                div()
                    .text_xs()
                    .text_color(theme::hex(theme::MUTED))
                    .child("对象字段顺序按文本规范化比较；所有内容留在本地。"),
            )
    }
}

fn format_json(value: &str) -> String {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        return "错误：内容为空".into();
    }
    match serde_json::from_str::<Value>(trimmed) {
        Ok(value) => serde_json::to_string_pretty(&normalize_json(value))
            .unwrap_or_else(|error| format!("错误：无法格式化 JSON：{error}")),
        Err(error) => format!("错误：{error}"),
    }
}

fn normalize_json(value: Value) -> Value {
    match value {
        Value::Object(object) => {
            let mut entries = object.into_iter().collect::<Vec<_>>();
            entries.sort_by(|left, right| left.0.cmp(&right.0));
            let mut normalized = serde_json::Map::new();
            for (key, value) in entries {
                normalized.insert(key, normalize_json(value));
            }
            Value::Object(normalized)
        }
        Value::Array(values) => Value::Array(values.into_iter().map(normalize_json).collect()),
        value => value,
    }
}

fn diff_values(left: &Value, right: &Value, path: &str, differences: &mut Vec<String>) {
    match (left, right) {
        (Value::Object(left), Value::Object(right)) => {
            for (key, value) in left {
                let child_path = format!("{path}.{key}");
                match right.get(key) {
                    Some(other) => diff_values(value, other, &child_path, differences),
                    None => differences.push(format!("- {child_path}")),
                }
            }
            for key in right.keys().filter(|key| !left.contains_key(*key)) {
                differences.push(format!("+ {path}.{key}"));
            }
        }
        (Value::Array(left), Value::Array(right)) => {
            for index in 0..left.len().max(right.len()) {
                let child_path = format!("{path}[{index}]");
                match (left.get(index), right.get(index)) {
                    (Some(value), Some(other)) => {
                        diff_values(value, other, &child_path, differences)
                    }
                    (Some(_), None) => differences.push(format!("- {child_path}")),
                    (None, Some(_)) => differences.push(format!("+ {child_path}")),
                    (None, None) => {}
                }
            }
        }
        (left, right) if left != right => differences.push(format!("~ {path}: {left} → {right}")),
        _ => {}
    }
}

#[cfg(test)]
mod tests {
    use super::{diff_values, format_json};
    use serde_json::json;

    #[test]
    fn formatting_uses_real_json_parser() {
        assert_eq!(
            format_json(r#"{"b":2,"a":[true,null]}"#),
            "{\n  \"a\": [\n    true,\n    null\n  ],\n  \"b\": 2\n}"
        );
        assert!(format_json(r#"{"a":}"#).starts_with("错误："));
    }

    #[test]
    fn semantic_diff_reports_paths() {
        let mut differences = Vec::new();
        diff_values(
            &json!({"same": 1, "removed": true}),
            &json!({"same": 2, "added": false}),
            "$",
            &mut differences,
        );
        assert!(differences.iter().any(|item| item.contains("$.same")));
        assert!(differences.iter().any(|item| item == "- $.removed"));
        assert!(differences.iter().any(|item| item == "+ $.added"));
    }
}
