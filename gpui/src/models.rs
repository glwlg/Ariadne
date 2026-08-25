#[derive(Clone, Copy)]
pub struct SearchResult {
    pub title: &'static str,
    pub subtitle: &'static str,
    pub detail: &'static str,
}

pub const SEARCH_RESULTS: &[SearchResult] = &[
    SearchResult {
        title: "设置中心",
        subtitle: "主题、快捷键、诊断和本地配置",
        detail: "集中管理 Ariadne 的桌面行为与本地设置。",
    },
    SearchResult {
        title: "Hosts 管理",
        subtitle: "预览、检查并应用 Hosts Profile",
        detail: "在确认后应用本地 Hosts 配置。",
    },
    SearchResult {
        title: "JSON 对比",
        subtitle: "格式化并比较两份 JSON 文档",
        detail: "快速查看结构差异并复制结果。",
    },
    SearchResult {
        title: "网络监控",
        subtitle: "查看进程网络活动",
        detail: "按进程查看本机网络流量和连接状态。",
    },
    SearchResult {
        title: "剪贴板历史",
        subtitle: "搜索最近复制的文本和图片",
        detail: "从本地历史中找回可复用内容。",
    },
    SearchResult {
        title: "截图历史",
        subtitle: "检索截图、OCR 和二维码结果",
        detail: "从本地截图集合中快速找到证据。",
    },
];
