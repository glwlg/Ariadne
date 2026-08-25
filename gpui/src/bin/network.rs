use std::{
    net::{SocketAddr, TcpStream},
    process::Command,
    time::Duration,
};

use gpui::{Context, MouseButton, MouseDownEvent, SharedString, Window, div, prelude::*, px};

use crate::theme;

struct Connection {
    protocol: String,
    local: String,
    remote: String,
    state: String,
    pid: String,
}

pub struct NetworkPage {
    status: SharedString,
    checks: Vec<(String, String, bool)>,
    connections: Vec<Connection>,
}

impl NetworkPage {
    pub fn new() -> Self {
        Self {
            status: "等待网络检查".into(),
            checks: Vec::new(),
            connections: Vec::new(),
        }
    }

    fn back(&mut self, _: &MouseDownEvent, window: &mut Window, _: &mut Context<Self>) {
        window.remove_window();
    }

    fn check(&mut self, _: &MouseDownEvent, _: &mut Window, cx: &mut Context<Self>) {
        let targets = [
            ("本机 DNS", "1.1.1.1:53"),
            ("HTTPS", "1.1.1.1:443"),
            ("Google DNS", "8.8.8.8:53"),
        ];
        self.checks = targets
            .iter()
            .map(|(label, target)| {
                let address = target.parse::<SocketAddr>().expect("static socket address");
                let ok = TcpStream::connect_timeout(&address, Duration::from_millis(700)).is_ok();
                ((*label).into(), (*target).into(), ok)
            })
            .collect();
        self.connections = read_connections();
        self.status = format!(
            "已完成 {} 项连接检查 · 发现 {} 条活动连接",
            self.checks.len(),
            self.connections.len()
        )
        .into();
        cx.notify();
    }
}

impl Render for NetworkPage {
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
                            .child(div().text_xl().child("网络监控"))
                            .child(
                                div()
                                    .text_sm()
                                    .text_color(theme::hex(theme::MUTED))
                                    .child("本地连接诊断 · 不采集流量内容"),
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
                    .gap_3()
                    .child(
                        div()
                            .rounded_md()
                            .bg(theme::hex(theme::PRIMARY))
                            .text_color(theme::hex(theme::SURFACE))
                            .px_3()
                            .py_2()
                            .text_sm()
                            .child("开始检查")
                            .on_mouse_down(MouseButton::Left, cx.listener(Self::check)),
                    )
                    .child(
                        div()
                            .text_sm()
                            .text_color(theme::hex(theme::MUTED))
                            .child(self.status.clone()),
                    ),
            )
            .child(
                div()
                    .flex_1()
                    .flex()
                    .gap_4()
                    .child(
                        div()
                            .flex_1()
                            .flex()
                            .flex_col()
                            .gap_3()
                            .rounded_lg()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .bg(theme::hex(theme::SURFACE))
                            .p_4()
                            .child(div().text_sm().child("连接状态"))
                            .children(self.checks.iter().map(|(label, target, ok)| {
                                div()
                                    .flex()
                                    .items_center()
                                    .justify_between()
                                    .border_b_1()
                                    .border_color(theme::hex(theme::BORDER))
                                    .py_3()
                                    .child(
                                        div()
                                            .flex()
                                            .flex_col()
                                            .gap_1()
                                            .child(div().text_sm().child(label.clone()))
                                            .child(
                                                div()
                                                    .text_xs()
                                                    .text_color(theme::hex(theme::MUTED))
                                                    .child(target.clone()),
                                            ),
                                    )
                                    .child(
                                        div()
                                            .text_sm()
                                            .text_color(theme::hex(if *ok {
                                                theme::SUCCESS
                                            } else {
                                                theme::DANGER
                                            }))
                                            .child(if *ok { "可达" } else { "不可达" }),
                                    )
                            }))
                            .child(div().text_sm().child("活动连接"))
                            .children(self.connections.iter().map(|connection| {
                                div()
                                    .flex()
                                    .items_center()
                                    .justify_between()
                                    .border_b_1()
                                    .border_color(theme::hex(theme::BORDER))
                                    .py_2()
                                    .child(
                                        div()
                                            .flex()
                                            .flex_col()
                                            .gap_1()
                                            .child(div().text_xs().child(format!(
                                                "{} · PID {}",
                                                connection.protocol, connection.pid
                                            )))
                                            .child(
                                                div()
                                                    .text_xs()
                                                    .text_color(theme::hex(theme::MUTED))
                                                    .child(format!(
                                                        "{} → {}",
                                                        connection.local, connection.remote
                                                    )),
                                            ),
                                    )
                                    .child(
                                        div()
                                            .text_xs()
                                            .text_color(theme::hex(theme::MUTED))
                                            .child(connection.state.clone()),
                                    )
                            })),
                    )
                    .child(
                        div()
                            .w(px(280.))
                            .flex()
                            .flex_col()
                            .gap_3()
                            .rounded_lg()
                            .border_1()
                            .border_color(theme::hex(theme::BORDER))
                            .bg(theme::hex(theme::SURFACE))
                            .p_4()
                            .child(div().text_sm().child("运行环境"))
                            .child(
                                div().flex().flex_col().gap_2().text_sm().children([
                                    div().flex().justify_between().child("工作目录").child(
                                        std::env::current_dir()
                                            .map(|path| path.display().to_string())
                                            .unwrap_or_else(|_| "未知".to_string()),
                                    ),
                                    div()
                                        .flex()
                                        .justify_between()
                                        .child("进程 ID")
                                        .child(std::process::id().to_string()),
                                    div()
                                        .flex()
                                        .justify_between()
                                        .child("平台")
                                        .child(std::env::consts::OS),
                                ]),
                            ),
                    ),
            )
    }
}

fn read_connections() -> Vec<Connection> {
    let Ok(output) = Command::new("netstat").arg("-ano").output() else {
        return Vec::new();
    };
    let text = String::from_utf8_lossy(&output.stdout);
    text.lines()
        .filter_map(|line| {
            let fields = line.split_whitespace().collect::<Vec<_>>();
            match fields.first().copied() {
                Some(protocol) if protocol.eq_ignore_ascii_case("TCP") && fields.len() >= 5 => {
                    Some(Connection {
                        protocol: protocol.into(),
                        local: fields[1].into(),
                        remote: fields[2].into(),
                        state: fields[3].into(),
                        pid: fields[4].into(),
                    })
                }
                Some(protocol) if protocol.eq_ignore_ascii_case("UDP") && fields.len() >= 4 => {
                    Some(Connection {
                        protocol: protocol.into(),
                        local: fields[1].into(),
                        remote: "*:*".into(),
                        state: "监听".into(),
                        pid: fields[3].into(),
                    })
                }
                _ => None,
            }
        })
        .take(80)
        .collect()
}
