use gpui::{
    Context, Entity, MouseButton, MouseDownEvent, SharedString, Subscription, Window, actions, div,
    prelude::*, rgb,
};
use gpui_component::input::{Input, InputEvent, InputState};

use crate::{
    models::{SEARCH_RESULTS, SearchResult},
    theme,
};

actions!(launcher, [MoveUp, MoveDown, Confirm]);

pub struct LauncherPage {
    input: Entity<InputState>,
    filtered: Vec<usize>,
    selected: usize,
    status: SharedString,
    _subscriptions: Vec<Subscription>,
}

impl LauncherPage {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> Self {
        let input = cx.new(|cx| InputState::new(window, cx).placeholder("搜索应用、文件和工具"));
        let mut page = Self {
            input: input.clone(),
            filtered: Vec::new(),
            selected: 0,
            status: "输入关键词开始搜索".into(),
            _subscriptions: Vec::new(),
        };

        page._subscriptions.push(cx.subscribe_in(&input, window, {
            let input = input.clone();
            move |page, _, event, _, cx| match event {
                InputEvent::Change => {
                    page.apply_query(input.read(cx).value().to_string());
                    cx.notify();
                }
                InputEvent::PressEnter { .. } => page.confirm(cx),
                _ => {}
            }
        }));

        page.apply_query(String::new());
        page
    }

    fn apply_query(&mut self, query: String) {
        let query = query.trim().to_lowercase();
        self.filtered = matching_indices(&query);
        self.selected = self.selected.min(self.filtered.len().saturating_sub(1));
        self.status = if query.is_empty() {
            "输入关键词开始搜索".into()
        } else if self.filtered.is_empty() {
            "没有找到匹配结果".into()
        } else {
            format!("找到 {} 个结果", self.filtered.len()).into()
        };
    }

    fn move_up(&mut self, _: &MoveUp, _: &mut Window, cx: &mut Context<Self>) {
        self.selected = self.selected.saturating_sub(1);
        cx.notify();
    }

    fn move_down(&mut self, _: &MoveDown, _: &mut Window, cx: &mut Context<Self>) {
        if !self.filtered.is_empty() {
            self.selected = (self.selected + 1).min(self.filtered.len() - 1);
            cx.notify();
        }
    }

    fn confirm(&mut self, cx: &mut Context<Self>) {
        if let Some(result) = self.selected_result() {
            self.status = format!("已选择：{}", result.title).into();
            cx.notify();
        }
    }

    fn selected_result(&self) -> Option<&SearchResult> {
        self.filtered
            .get(self.selected)
            .and_then(|index| SEARCH_RESULTS.get(*index))
    }

    fn select(&mut self, index: usize, cx: &mut Context<Self>) {
        if index < self.filtered.len() {
            self.selected = index;
            cx.notify();
        }
    }
}

fn matching_indices(query: &str) -> Vec<usize> {
    SEARCH_RESULTS
        .iter()
        .enumerate()
        .filter_map(|(index, result)| {
            let haystack = [result.title, result.subtitle, result.detail]
                .join(" ")
                .to_lowercase();
            (query.is_empty() || haystack.contains(query)).then_some(index)
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::matching_indices;

    #[test]
    fn matching_indices_searches_all_result_text() {
        assert_eq!(matching_indices("json"), vec![2]);
        assert_eq!(matching_indices("流量"), vec![3]);
        assert_eq!(matching_indices("不存在"), Vec::<usize>::new());
    }
}

impl Render for LauncherPage {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let selected = self.selected;
        let filtered = self.filtered.clone();

        div()
            .size_full()
            .flex()
            .flex_col()
            .gap_4()
            .p_6()
            .bg(rgb(theme::BACKGROUND))
            .text_color(rgb(theme::FOREGROUND))
            .key_context("Launcher")
            .on_action(cx.listener(Self::move_up))
            .on_action(cx.listener(Self::move_down))
            .on_action(cx.listener(|this, _: &Confirm, _window, cx| this.confirm(cx)))
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
                            .child(div().text_xl().child("Ariadne"))
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(rgb(theme::MUTED))
                                    .child("找到工作中的线索"),
                            ),
                    )
                    .child(
                        div()
                            .rounded_md()
                            .px_2()
                            .py_1()
                            .bg(rgb(theme::SURFACE_SELECTED))
                            .text_color(rgb(theme::PRIMARY))
                            .text_sm()
                            .child("Launcher"),
                    ),
            )
            .child(
                div()
                    .rounded_md()
                    .border_1()
                    .border_color(rgb(theme::BORDER))
                    .bg(rgb(theme::SURFACE))
                    .child(Input::new(&self.input).w_full()),
            )
            .child(
                div().flex_1().flex().flex_col().gap_2().children(
                    filtered
                        .iter()
                        .enumerate()
                        .filter_map(|(position, result_index)| {
                            let result = SEARCH_RESULTS.get(*result_index)?;
                            let is_selected = position == selected;
                            Some(
                                div()
                                    .flex()
                                    .flex_col()
                                    .gap_1()
                                    .p_3()
                                    .rounded_md()
                                    .border_1()
                                    .border_color(rgb(theme::BORDER))
                                    .bg(rgb(if is_selected {
                                        theme::SURFACE_SELECTED
                                    } else {
                                        theme::SURFACE
                                    }))
                                    .hover(|style| style.bg(rgb(theme::SURFACE_SELECTED)))
                                    .on_mouse_down(
                                        MouseButton::Left,
                                        cx.listener(move |this, _: &MouseDownEvent, _, cx| {
                                            this.select(position, cx);
                                        }),
                                    )
                                    .child(div().text_lg().child(result.title))
                                    .child(
                                        div()
                                            .text_sm()
                                            .text_color(rgb(theme::MUTED))
                                            .child(result.subtitle),
                                    ),
                            )
                        }),
                ),
            )
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .text_sm()
                    .text_color(rgb(theme::MUTED))
                    .child(self.status.clone())
                    .child("↑ ↓ 选择 · Enter 确认"),
            )
    }
}
