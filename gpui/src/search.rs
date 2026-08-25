use std::{
    collections::HashSet,
    fs,
    io::{BufRead, BufReader, Cursor},
    path::{Path, PathBuf},
    sync::{Arc, Mutex, OnceLock},
};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Route {
    Settings,
    Hosts,
    JsonCompare,
    Network,
    Clipboard,
}

#[derive(Clone, Debug)]
pub enum SearchAction {
    Route(Route),
    Copy(String),
    OpenPath(PathBuf),
}

#[derive(Clone, Debug)]
pub struct SearchResult {
    pub id: String,
    pub title: String,
    pub subtitle: String,
    pub detail: String,
    pub icon: &'static str,
    pub action_label: &'static str,
    pub action: SearchAction,
}

#[derive(Clone, Debug, Default)]
struct SearchPolicy {
    apply_defaults: bool,
    include_extensions: Vec<String>,
    exclude_folders: Vec<String>,
    exclude_patterns: Vec<String>,
    exclude_extensions: Vec<String>,
}

fn search_policy() -> SearchPolicy {
    let settings = crate::settings::load_search_exclusions();
    SearchPolicy {
        apply_defaults: false,
        include_extensions: settings
            .include_extensions
            .into_iter()
            .map(|item| normalize_extension(&item))
            .filter(|item| !item.is_empty())
            .collect(),
        exclude_folders: settings
            .exclude_folders
            .into_iter()
            .map(|item| normalize_path(&item))
            .filter(|item| !item.is_empty())
            .collect(),
        exclude_patterns: settings
            .exclude_patterns
            .into_iter()
            .map(|item| item.to_ascii_lowercase())
            .filter(|item| !item.trim().is_empty())
            .collect(),
        exclude_extensions: settings
            .exclude_extensions
            .into_iter()
            .map(|item| normalize_extension(&item))
            .filter(|item| !item.is_empty())
            .collect(),
    }
}

fn normalize_path(value: &str) -> String {
    let mut value = value.trim().to_owned();
    for (name, replacement) in std::env::vars() {
        value = value.replace(&format!("%{name}%"), &replacement);
    }
    value
        .replace('/', "\\")
        .trim_end_matches('\\')
        .to_ascii_lowercase()
}
fn normalize_index_path(value: &str) -> String {
    value.trim().replace('/', "\\").to_ascii_lowercase()
}
fn normalize_extension(value: &str) -> String {
    let value = value.trim().trim_start_matches('*').to_ascii_lowercase();
    if value.is_empty() || value == "." || value.contains(['/', '\\', ' ']) {
        return String::new();
    }
    if value.starts_with('.') {
        value
    } else {
        format!(".{value}")
    }
}
pub fn search_results(query: &str) -> Vec<SearchResult> {
    const MAX_SEARCH_RESULTS: usize = 200;
    let query = query.trim();
    if query.is_empty() {
        return Vec::new();
    }
    let lower = query.to_lowercase();
    let mut results = builtin_results()
        .into_iter()
        .filter(|result| {
            [
                result.id.as_str(),
                result.title.as_str(),
                result.subtitle.as_str(),
                result.detail.as_str(),
            ]
            .join(" ")
            .to_lowercase()
            .contains(&lower)
        })
        .collect::<Vec<_>>();
    let commands = command_results(query);
    let has_command_results = !commands.is_empty();
    results.extend(commands);
    if !has_command_results && lower.chars().count() >= 2 {
        let policy = search_policy();
        let mut seen_paths = HashSet::new();
        if results.len() < MAX_SEARCH_RESULTS {
            results.extend(index_results_with_policy(
                &lower,
                &mut seen_paths,
                MAX_SEARCH_RESULTS - results.len(),
                &policy,
            ));
        }
        #[cfg(windows)]
        if results.len() < MAX_SEARCH_RESULTS
            && std::env::var_os("ARIADNE_USE_EVERYTHING").is_some()
        {
            for result in everything_results(&lower) {
                let include = result_path(&result)
                    .map(|path| {
                        !path_is_excluded(path, false, &policy) && seen_paths.insert(path_key(path))
                    })
                    .unwrap_or(true);
                if include {
                    results.push(result);
                }
                if results.len() >= MAX_SEARCH_RESULTS {
                    break;
                }
            }
        }
        if results.len() < MAX_SEARCH_RESULTS {
            results.extend(workspace_results_with_policy(
                &lower,
                &mut seen_paths,
                MAX_SEARCH_RESULTS - results.len(),
                &policy,
            ));
        }
    }
    results
}

fn index_results_with_policy(
    query: &str,
    seen_paths: &mut HashSet<String>,
    max_results: usize,
    policy: &SearchPolicy,
) -> Vec<SearchResult> {
    if max_results == 0 {
        return Vec::new();
    }
    let normalized = query.trim().to_lowercase();
    if normalized.chars().count() < 2 {
        return Vec::new();
    }
    let match_query = query_basename(&normalized);
    if match_query.is_empty() {
        return Vec::new();
    }
    let path_like = is_path_like_query(&normalized);
    let wildcard = normalized.contains('*')
        || normalized.contains('?')
        || match_query.contains('*')
        || match_query.contains('?');
    let max_candidates = max_results.saturating_mul(8).max(max_results);
    let Some(index) = load_file_index() else {
        return Vec::new();
    };
    let mut reader = BufReader::with_capacity(128 * 1024, Cursor::new(index));
    let mut candidates = Vec::with_capacity(max_candidates.min(4096));
    let mut worst_score = 0.;
    let mut seen = HashSet::new();
    let mut line = Vec::new();
    loop {
        line.clear();
        let Ok(read) = reader.read_until(b'\n', &mut line) else {
            break;
        };
        if read == 0 {
            break;
        }
        let line = String::from_utf8_lossy(&line);
        let line = line.trim_end_matches(['\r', '\n']);
        let mut fields = line.splitn(3, '\t');
        let indexed_name = fields.next().unwrap_or_default().trim();
        let flag = fields.next().unwrap_or_default();
        let raw_path = fields.next().unwrap_or_default().trim();
        if indexed_name.is_empty() || raw_path.is_empty() || (flag != "0" && flag != "1") {
            continue;
        }
        let path = PathBuf::from(raw_path);
        let is_dir = flag == "1";
        if path_is_excluded(&path, is_dir, policy) {
            continue;
        }
        let lower_name = indexed_name.to_lowercase();
        let lower_path = normalize_index_path(raw_path);
        let matches = if path_like {
            if wildcard {
                wildcard_matches(&lower_path, &normalize_glob(&normalized))
            } else {
                lower_path.contains(&normalize_index_path(&normalized))
            }
        } else if wildcard {
            wildcard_matches(&lower_name, &normalize_glob(&match_query))
        } else {
            lower_name.contains(&match_query)
        };
        if !matches {
            continue;
        }
        let key = path_key(&path);
        if !seen.insert(key) {
            continue;
        }
        let score = original_file_score(&lower_name, &lower_path, &match_query, &normalized);
        let candidate = IndexedCandidate {
            path,
            is_dir,
            score,
        };
        if candidates.len() < max_candidates {
            candidates.push(candidate);
            if candidates.len() == max_candidates {
                worst_score = candidates
                    .iter()
                    .map(|item| item.score)
                    .fold(f64::INFINITY, f64::min);
            }
            continue;
        }
        if candidate.score <= worst_score {
            continue;
        }
        let Some((worst_index, _)) =
            candidates
                .iter()
                .enumerate()
                .min_by(|(_, left), (_, right)| {
                    left.score
                        .partial_cmp(&right.score)
                        .unwrap_or(std::cmp::Ordering::Equal)
                        .then_with(|| path_key(&left.path).cmp(&path_key(&right.path)))
                })
        else {
            continue;
        };
        candidates[worst_index] = candidate;
        worst_score = candidates
            .iter()
            .map(|item| item.score)
            .fold(f64::INFINITY, f64::min);
    }
    candidates.sort_by(|left, right| {
        right
            .score
            .partial_cmp(&left.score)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then_with(|| path_key(&left.path).cmp(&path_key(&right.path)))
    });
    let mut results = Vec::with_capacity(max_results.min(candidates.len()));
    for candidate in candidates {
        if !seen_paths.insert(path_key(&candidate.path)) {
            continue;
        }
        let title =
            path_file_name(&candidate.path).unwrap_or_else(|| candidate.path.display().to_string());
        results.push(SearchResult {
            id: format!("index:{}", candidate.path.display()),
            title,
            subtitle: if candidate.is_dir {
                "文件索引 · 文件夹".into()
            } else {
                "文件索引 · 文件".into()
            },
            detail: candidate.path.display().to_string(),
            icon: if candidate.is_dir { "□" } else { "▤" },
            action_label: if candidate.is_dir {
                "打开文件夹"
            } else {
                "打开文件"
            },
            action: SearchAction::OpenPath(candidate.path),
        });
        if results.len() >= max_results {
            break;
        }
    }
    results
}

#[derive(Clone, Debug)]
struct IndexedCandidate {
    path: PathBuf,
    is_dir: bool,
    score: f64,
}

#[derive(Clone)]
struct CachedIndex {
    len: u64,
    modified: Option<std::time::SystemTime>,
    bytes: Arc<[u8]>,
}

fn load_file_index() -> Option<Arc<[u8]>> {
    static CACHE: OnceLock<Mutex<std::collections::HashMap<String, CachedIndex>>> = OnceLock::new();
    let path = file_index_path()?;
    let metadata = fs::metadata(&path).ok()?;
    if !metadata.is_file() || metadata.len() == 0 {
        return None;
    }
    let key = path_key(&path);
    let modified = metadata.modified().ok();
    let cache = CACHE.get_or_init(|| Mutex::new(std::collections::HashMap::new()));
    let mut guard = cache.lock().ok()?;
    if let Some(cached) = guard.get(&key)
        && cached.len == metadata.len()
        && cached.modified == modified
    {
        return Some(cached.bytes.clone());
    }
    let bytes: Arc<[u8]> = Arc::from(fs::read(&path).ok()?.into_boxed_slice());
    if bytes.is_empty() {
        return None;
    }
    guard.insert(
        key,
        CachedIndex {
            len: metadata.len(),
            modified,
            bytes: bytes.clone(),
        },
    );
    Some(bytes)
}
fn file_index_path() -> Option<PathBuf> {
    let mut candidates = Vec::new();
    for variable in ["ARIADNE_FILE_INDEX", "ARIADNE_FILE_INDEX_PATH"] {
        if let Some(value) = std::env::var_os(variable) {
            let value = expand_path_variables(&value.to_string_lossy());
            if !value.trim().is_empty() {
                candidates.push(PathBuf::from(value));
                break;
            }
        }
    }
    if candidates.is_empty() {
        #[cfg(windows)]
        {
            if let Some(base) = std::env::var_os("PROGRAMDATA") {
                candidates.push(PathBuf::from(base).join("Ariadne").join("file-index.tsv"));
            }
            candidates.push(PathBuf::from(r"C:\ProgramData\Ariadne\file-index.tsv"));
            if let Some(base) = std::env::var_os("LOCALAPPDATA") {
                candidates.push(PathBuf::from(base).join("Ariadne").join("file-index.tsv"));
            }
            if let Some(profile) = std::env::var_os("USERPROFILE") {
                candidates
                    .push(PathBuf::from(profile).join(r"AppData\Local\Ariadne\file-index.tsv"));
            }
            candidates.push(PathBuf::from(
                r"C:\Users\luwei\AppData\Local\Ariadne\file-index.tsv",
            ));
        }
        #[cfg(not(windows))]
        {
            candidates.push(PathBuf::from("file-index.tsv"));
        }
    }
    let mut seen = HashSet::new();
    candidates
        .into_iter()
        .find(|path| seen.insert(path_key(path)) && path.is_file())
}
fn expand_path_variables(value: &str) -> String {
    let mut expanded = value.to_owned();
    for (name, replacement) in std::env::vars() {
        expanded = expanded.replace(&format!("%{name}%"), &replacement);
    }
    expanded
}

fn query_basename(query: &str) -> String {
    query
        .rsplit(|character| character == '\\' || character == '/')
        .next()
        .unwrap_or(query)
        .trim()
        .to_owned()
}

fn is_path_like_query(query: &str) -> bool {
    query.contains('\\') || query.contains('/') || query.contains(':')
}
fn normalize_glob(value: &str) -> String {
    value.replace('\\', "/").to_ascii_lowercase()
}

fn wildcard_matches(text: &str, pattern: &str) -> bool {
    let text = normalize_glob(text).chars().collect::<Vec<_>>();
    let pattern = normalize_glob(pattern).chars().collect::<Vec<_>>();
    let mut text_index = 0;
    let mut pattern_index = 0;
    let mut star_index = None;
    let mut star_text_index = 0;
    while text_index < text.len() {
        if pattern_index < pattern.len()
            && (pattern[pattern_index] == '?' || pattern[pattern_index] == text[text_index])
        {
            text_index += 1;
            pattern_index += 1;
        } else if pattern_index < pattern.len() && pattern[pattern_index] == '*' {
            star_index = Some(pattern_index);
            pattern_index += 1;
            star_text_index = text_index;
        } else if let Some(star) = star_index {
            pattern_index = star + 1;
            star_text_index += 1;
            text_index = star_text_index;
        } else {
            return false;
        }
    }
    while pattern_index < pattern.len() && pattern[pattern_index] == '*' {
        pattern_index += 1;
    }
    pattern_index == pattern.len()
}

fn original_file_score(name: &str, path: &str, match_query: &str, normalized_query: &str) -> f64 {
    if name == match_query {
        return 95.;
    }
    if name.starts_with(match_query) {
        return 88.;
    }
    if name.contains(match_query) {
        return 72.;
    }
    if path.contains(normalized_query) {
        return 52.;
    }
    30.
}

fn default_excluded_extension(path: &str) -> bool {
    listed_extension(
        path,
        &[
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
        ],
    )
}

fn path_file_name(path: &Path) -> Option<String> {
    path.file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .filter(|name| !name.is_empty())
        .or_else(|| {
            path.to_string_lossy()
                .rsplit(['\\', '/'])
                .next()
                .map(str::to_owned)
                .filter(|name| !name.is_empty())
        })
}
fn builtin_results() -> Vec<SearchResult> {
    vec![
        tool(
            "settings",
            "设置中心",
            "配置 / 主题 / 快捷键",
            "管理 Ariadne 的启动、快捷键和本地配置。默认主题为亮石墨。",
            "⚙",
            "打开设置",
            Route::Settings,
        ),
        tool(
            "hosts",
            "Hosts 管理",
            "预览、检查并应用 Hosts Profile",
            "编辑本地 Hosts 方案，查看规则统计和冲突域名。",
            "▣",
            "打开 Hosts",
            Route::Hosts,
        ),
        tool(
            "json-compare",
            "JSON 对比",
            "格式化并比较两份 JSON 文档",
            "在本地格式化、规范化并查看两份 JSON 的字段差异。",
            "{}",
            "打开 JSON 对比",
            Route::JsonCompare,
        ),
        tool(
            "network",
            "网络监控",
            "查看进程网络活动",
            "查看当前机器的网络诊断信息，并快速测试常用地址。",
            "⌁",
            "打开网络监控",
            Route::Network,
        ),
        tool(
            "clipboard",
            "剪贴板历史",
            "搜索最近复制的文本",
            "读取当前系统剪贴板，并把常用文本重新复制出来。",
            "▤",
            "打开剪贴板",
            Route::Clipboard,
        ),
    ]
}

fn tool(
    id: &str,
    title: &str,
    subtitle: &str,
    detail: &str,
    icon: &'static str,
    action_label: &'static str,
    route: Route,
) -> SearchResult {
    SearchResult {
        id: id.into(),
        title: title.into(),
        subtitle: subtitle.into(),
        detail: detail.into(),
        icon,
        action_label,
        action: SearchAction::Route(route),
    }
}

fn command_results(query: &str) -> Vec<SearchResult> {
    let mut parts = query.splitn(2, char::is_whitespace);
    let command = parts.next().unwrap_or_default().to_lowercase();
    let value = parts.next().unwrap_or_default().trim();
    match command.as_str() {
        "json" | "j" if !value.is_empty() => match serde_json::from_str::<serde_json::Value>(value)
        {
            Ok(json) => {
                let formatted =
                    serde_json::to_string_pretty(&json).unwrap_or_else(|_| value.into());
                let minified = serde_json::to_string(&json).unwrap_or_else(|_| value.into());
                vec![
                    SearchResult {
                        id: "json-format".into(),
                        title: "格式化 JSON".into(),
                        subtitle: "命令结果 · json".into(),
                        detail: formatted.clone(),
                        icon: "{}",
                        action_label: "复制结果",
                        action: SearchAction::Copy(formatted),
                    },
                    SearchResult {
                        id: "json-minify".into(),
                        title: "压缩 JSON".into(),
                        subtitle: "命令结果 · json".into(),
                        detail: minified.clone(),
                        icon: "{}",
                        action_label: "复制结果",
                        action: SearchAction::Copy(minified),
                    },
                ]
            }
            Err(error) => vec![SearchResult {
                id: "json-error".into(),
                title: "JSON 解析错误".into(),
                subtitle: "命令结果 · json".into(),
                detail: error.to_string(),
                icon: "!",
                action_label: "复制错误",
                action: SearchAction::Copy(error.to_string()),
            }],
        },
        "base64" | "b64" if !value.is_empty() => {
            let encoded = base64_encode(value.as_bytes());
            vec![SearchResult {
                id: "base64".into(),
                title: "Base64 编码结果".into(),
                subtitle: "命令结果 · base64".into(),
                detail: encoded.clone(),
                icon: "⌘",
                action_label: "复制结果",
                action: SearchAction::Copy(encoded),
            }]
        }
        "url" | "u" if !value.is_empty() => {
            let encoded = url_encode(value);
            vec![SearchResult {
                id: "url".into(),
                title: "URL 编码结果".into(),
                subtitle: "命令结果 · url".into(),
                detail: encoded.clone(),
                icon: "↗",
                action_label: "复制结果",
                action: SearchAction::Copy(encoded),
            }]
        }
        "uuid" | "guid" => {
            let count = value.parse::<usize>().unwrap_or(1).clamp(1, 20);
            let text = (0..count)
                .map(|_| pseudo_uuid())
                .collect::<Vec<_>>()
                .join("\n");
            vec![SearchResult {
                id: "uuid".into(),
                title: format!("生成 {count} 个 UUID"),
                subtitle: "命令结果 · uuid / guid".into(),
                detail: text.clone(),
                icon: "#",
                action_label: "复制结果",
                action: SearchAction::Copy(text),
            }]
        }
        _ => Vec::new(),
    }
}

#[cfg(windows)]
fn everything_results(query: &str) -> Vec<SearchResult> {
    use std::{os::windows::ffi::OsStrExt, sync::OnceLock};
    use windows_sys::Win32::{
        Foundation::HMODULE,
        System::LibraryLoader::{GetProcAddress, LoadLibraryW},
    };

    type SetSearchW = unsafe extern "system" fn(*const u16);
    type SetRequestFlags = unsafe extern "system" fn(u32);
    type QueryW = unsafe extern "system" fn(i32) -> i32;
    type GetNumResults = unsafe extern "system" fn() -> u32;
    type GetResultFileNameW = unsafe extern "system" fn(u32) -> *const u16;
    type GetResultPathW = unsafe extern "system" fn(u32) -> *const u16;
    type SetMax = unsafe extern "system" fn(u32);

    struct EverythingApi {
        set_search: SetSearchW,
        set_request_flags: SetRequestFlags,
        query: QueryW,
        get_num_results: GetNumResults,
        get_result_file_name: GetResultFileNameW,
        get_result_path: GetResultPathW,
        set_max: SetMax,
    }

    unsafe fn function<T>(module: HMODULE, name: &[u8]) -> Option<T> {
        unsafe {
            GetProcAddress(module, name.as_ptr()).map(|address| std::mem::transmute_copy(&address))
        }
    }

    fn load_api() -> Option<EverythingApi> {
        let mut candidates = Vec::new();
        if let Ok(executable) = std::env::current_exe() {
            if let Some(parent) = executable.parent() {
                candidates.push(parent.join("Everything64.dll"));
            }
        }
        if let Ok(current_dir) = std::env::current_dir() {
            candidates.push(current_dir.join("Everything64.dll"));
            candidates.push(current_dir.join("bin").join("Everything64.dll"));
            candidates.push(
                current_dir
                    .join("legacy")
                    .join("x-tools-python")
                    .join("Everything64.dll"),
            );
        }
        candidates.push(std::path::PathBuf::from(
            r"C:\Program Files\Everything\Everything64.dll",
        ));
        candidates.push(std::path::PathBuf::from(
            r"C:\Program Files (x86)\Everything\Everything64.dll",
        ));

        for path in candidates {
            let wide = std::ffi::OsStr::new(&path)
                .encode_wide()
                .chain(std::iter::once(0))
                .collect::<Vec<_>>();
            let module = unsafe { LoadLibraryW(wide.as_ptr()) };
            if module.is_null() {
                continue;
            }
            let api = unsafe {
                Some(EverythingApi {
                    set_search: function(module, b"Everything_SetSearchW\0")?,
                    set_request_flags: function(module, b"Everything_SetRequestFlags\0")?,
                    query: function(module, b"Everything_QueryW\0")?,
                    get_num_results: function(module, b"Everything_GetNumResults\0")?,
                    get_result_file_name: function(module, b"Everything_GetResultFileNameW\0")?,
                    get_result_path: function(module, b"Everything_GetResultPathW\0")?,
                    set_max: function(module, b"Everything_SetMax\0")?,
                })
            };
            if api.is_some() {
                return api;
            }
        }
        None
    }

    fn wide_string(value: *const u16) -> String {
        if value.is_null() {
            return String::new();
        }
        let mut length = 0;
        unsafe {
            while *value.add(length) != 0 {
                length += 1;
            }
            String::from_utf16_lossy(std::slice::from_raw_parts(value, length))
        }
    }

    static API: OnceLock<Option<EverythingApi>> = OnceLock::new();
    let Some(api) = API.get_or_init(load_api).as_ref() else {
        return Vec::new();
    };
    let query = std::ffi::OsStr::new(query)
        .encode_wide()
        .chain(std::iter::once(0))
        .collect::<Vec<_>>();

    unsafe {
        (api.set_search)(query.as_ptr());
        (api.set_request_flags)(0x0000_0003);
        (api.set_max)(50);
        if (api.query)(1) == 0 {
            return Vec::new();
        }
        (0..(api.get_num_results)().min(50))
            .filter_map(|index| {
                let name = wide_string((api.get_result_file_name)(index));
                let directory = wide_string((api.get_result_path)(index));
                if name.is_empty() || directory.is_empty() {
                    return None;
                }
                let path = PathBuf::from(directory).join(&name);
                Some(SearchResult {
                    id: format!("everything:{}", path.display()),
                    title: name,
                    subtitle: "全盘索引 · 文件".into(),
                    detail: path.display().to_string(),
                    icon: "▤",
                    action_label: "打开文件",
                    action: SearchAction::OpenPath(path),
                })
            })
            .collect()
    }
}

#[cfg(not(windows))]
fn everything_results(_query: &str) -> Vec<SearchResult> {
    Vec::new()
}
fn workspace_results(query: &str, seen_paths: &mut HashSet<String>) -> Vec<SearchResult> {
    workspace_results_with_policy(query, seen_paths, 12, &search_policy())
}

fn workspace_results_with_policy(
    query: &str,
    seen_paths: &mut HashSet<String>,
    max_results: usize,
    policy: &SearchPolicy,
) -> Vec<SearchResult> {
    let Ok(current_dir) = std::env::current_dir() else {
        return Vec::new();
    };
    const MAX_RESULTS_PER_ROOT: usize = 25;
    let mut results = Vec::new();
    for (index, root) in search_roots(&current_dir).into_iter().enumerate() {
        let remaining = max_results.saturating_sub(results.len());
        if remaining == 0 {
            break;
        }
        let scope = if index == 0 {
            "当前工作目录"
        } else {
            "本地磁盘"
        };
        results.extend(filesystem_results_in_with_policy(
            &root,
            query,
            remaining.min(MAX_RESULTS_PER_ROOT),
            scope,
            seen_paths,
            policy,
        ));
    }
    results
}

fn workspace_results_in(root: &Path, query: &str) -> Vec<SearchResult> {
    let mut seen_paths = HashSet::new();
    filesystem_results_in(root, query, 12, "当前工作目录", &mut seen_paths)
}

fn filesystem_results_in(
    root: &Path,
    query: &str,
    max_results: usize,
    scope: &str,
    seen_paths: &mut HashSet<String>,
) -> Vec<SearchResult> {
    filesystem_results_in_with_policy(
        root,
        query,
        max_results,
        scope,
        seen_paths,
        &SearchPolicy::default(),
    )
}

fn filesystem_results_in_with_policy(
    root: &Path,
    query: &str,
    max_results: usize,
    scope: &str,
    seen_paths: &mut HashSet<String>,
    policy: &SearchPolicy,
) -> Vec<SearchResult> {
    // ponytail: bounded local walk; the shared TSV index provides full-disk breadth when available.
    const MAX_SCANNED_ENTRIES: usize = 50_000;
    if max_results == 0 || path_is_excluded(root, true, policy) {
        return Vec::new();
    }

    let mut pending = vec![root.to_path_buf()];
    let mut next_directory = 0;
    let mut results = Vec::new();
    let mut scanned = 0;

    while let Some(directory) = pending.get(next_directory).cloned() {
        next_directory += 1;
        let Ok(entries) = fs::read_dir(&directory) else {
            continue;
        };
        for entry in entries.filter_map(Result::ok) {
            scanned += 1;
            if scanned > MAX_SCANNED_ENTRIES {
                return results;
            }
            let path = entry.path();
            let Ok(file_type) = entry.file_type() else {
                continue;
            };
            if file_type.is_symlink() {
                continue;
            }

            let is_dir = file_type.is_dir();
            if path_is_excluded(&path, is_dir, policy) {
                continue;
            }
            let name = entry.file_name().to_string_lossy().into_owned();
            let relative = path
                .strip_prefix(root)
                .unwrap_or(&path)
                .to_string_lossy()
                .into_owned();
            if format!("{name} {relative}").to_lowercase().contains(query)
                && seen_paths.insert(path_key(&path))
            {
                results.push(SearchResult {
                    id: format!("path:{}", path.display()),
                    title: name,
                    subtitle: if is_dir {
                        format!("{scope} · 文件夹")
                    } else {
                        format!("{scope} · 文件")
                    },
                    detail: path.display().to_string(),
                    icon: if is_dir { "□" } else { "▤" },
                    action_label: if is_dir {
                        "打开文件夹"
                    } else {
                        "打开文件"
                    },
                    action: SearchAction::OpenPath(path.clone()),
                });
                if results.len() >= max_results {
                    return results;
                }
            }

            if is_dir {
                pending.push(path);
            }
        }
    }

    results
}
fn result_path(result: &SearchResult) -> Option<&Path> {
    match &result.action {
        SearchAction::OpenPath(path) => Some(path),
        _ => None,
    }
}

fn path_is_excluded(path: &Path, is_dir: bool, policy: &SearchPolicy) -> bool {
    let normalized = normalize_index_path(&path.to_string_lossy());
    if !is_dir && listed_extension(&normalized, &policy.include_extensions) {
        return false;
    }
    if policy.apply_defaults
        && path
            .to_string_lossy()
            .split(|character| character == '\\' || character == '/')
            .any(is_pruned_directory)
    {
        return true;
    }
    if policy.apply_defaults && !is_dir && default_excluded_extension(&normalized) {
        return true;
    }
    if policy.exclude_folders.iter().any(|folder| {
        if folder.contains('*') || folder.contains('?') {
            wildcard_matches(&normalized, folder)
        } else {
            normalized == *folder || normalized.starts_with(&format!("{folder}\\"))
        }
    }) {
        return true;
    }
    if policy
        .exclude_patterns
        .iter()
        .any(|pattern| pattern_matches(&normalized, pattern))
    {
        return true;
    }
    if is_dir {
        return false;
    }
    if listed_extension(&normalized, &policy.exclude_extensions) {
        return true;
    }
    false
}

fn listed_extension(path: &str, extensions: &[String]) -> bool {
    let name = path.rsplit('\\').next().unwrap_or(path);
    extensions.iter().any(|extension| {
        if extension.contains('*') || extension.contains('?') {
            wildcard_matches(name, extension)
        } else {
            name.len() > extension.len() && name.ends_with(extension)
        }
    })
}

fn pattern_matches(path: &str, pattern: &str) -> bool {
    let mut pattern = pattern.trim().to_ascii_lowercase();
    if let Some(rest) = pattern.strip_prefix("(?i)") {
        pattern = rest.to_owned();
    }
    if pattern.contains('*') || pattern.contains('?') {
        return wildcard_matches(path, &pattern);
    }
    if let Some(dot) = pattern.find(r"\.") {
        let suffix = &pattern[dot + 2..];
        if let Some(group) = suffix
            .strip_prefix('(')
            .and_then(|rest| rest.split_once(')'))
        {
            return group
                .0
                .split('|')
                .map(|extension| format!(".{extension}"))
                .any(|extension| path.ends_with(&extension));
        }
        let suffix = suffix.trim_end_matches('$');
        if !suffix.is_empty()
            && !suffix.contains('[')
            && !suffix.contains(']')
            && !suffix.contains('(')
            && !suffix.contains(')')
        {
            return path.ends_with(&format!(".{suffix}"));
        }
    }
    let literal = pattern
        .replace("\\/", "\\")
        .replace("\\\\", "\\")
        .replace("\\.", ".")
        .trim_matches(['^', '$', '(', ')'])
        .to_owned();
    literal
        .split('|')
        .filter(|item| !item.is_empty())
        .any(|item| path.contains(item))
}
fn path_key(path: &Path) -> String {
    let path = path.to_string_lossy().into_owned();
    #[cfg(windows)]
    {
        path.to_ascii_lowercase()
    }
    #[cfg(not(windows))]
    {
        path
    }
}

fn search_roots(current_dir: &Path) -> Vec<PathBuf> {
    let mut roots = Vec::new();
    push_search_root(&mut roots, current_dir.to_path_buf());

    #[cfg(windows)]
    {
        for base in ["PROGRAMDATA", "APPDATA"] {
            if let Some(base) = std::env::var_os(base) {
                push_search_root(
                    &mut roots,
                    PathBuf::from(base)
                        .join("Microsoft")
                        .join("Windows")
                        .join("Start Menu")
                        .join("Programs"),
                );
            }
        }
        if let Some(profile) = std::env::var_os("USERPROFILE") {
            let profile = PathBuf::from(profile);
            if profile.is_dir() {
                push_search_root(&mut roots, profile);
            }
        }
        for letter in b'A'..=b'Z' {
            let root = PathBuf::from(format!("{}:\\", letter as char));
            if root.is_dir() {
                push_search_root(&mut roots, root);
            }
        }
    }

    roots
}

fn push_search_root(roots: &mut Vec<PathBuf>, root: PathBuf) {
    let key = path_key(&root);
    if !roots.iter().any(|existing| path_key(existing) == key) {
        roots.push(root);
    }
}

fn is_pruned_directory(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        ".git"
            | "target"
            | "node_modules"
            | ".venv"
            | "venv"
            | "$recycle.bin"
            | "system volume information"
            | "appdata"
            | "cache"
            | "cache2"
            | "code cache"
            | "d3dscache"
            | "gpucache"
            | "shadercache"
            | "logs"
            | "log"
            | "tmp"
            | "temp"
            | "temp-index"
            | "tmp-index"
            | "site-packages"
            | "dist-packages"
    )
}
fn base64_encode(input: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut output = String::with_capacity(input.len().div_ceil(3) * 4);
    for chunk in input.chunks(3) {
        let value = ((chunk[0] as u32) << 16)
            | ((chunk.get(1).copied().unwrap_or(0) as u32) << 8)
            | chunk.get(2).copied().unwrap_or(0) as u32;
        output.push(TABLE[((value >> 18) & 0x3f) as usize] as char);
        output.push(TABLE[((value >> 12) & 0x3f) as usize] as char);
        output.push(if chunk.len() > 1 {
            TABLE[((value >> 6) & 0x3f) as usize] as char
        } else {
            '='
        });
        output.push(if chunk.len() > 2 {
            TABLE[(value & 0x3f) as usize] as char
        } else {
            '='
        });
    }
    output
}

fn url_encode(value: &str) -> String {
    value
        .bytes()
        .map(|byte| match byte {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                (byte as char).to_string()
            }
            _ => format!("%{byte:02X}"),
        })
        .collect()
}

fn pseudo_uuid() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_nanos())
        .unwrap_or_default();
    let value = format!("{nanos:032x}");
    format!(
        "{}-{}-4{}-a{}-{}",
        &value[0..8],
        &value[8..12],
        &value[13..16],
        &value[17..20],
        &value[20..32]
    )
}

#[cfg(test)]
mod tests {
    use std::{
        collections::HashSet,
        fs,
        path::PathBuf,
        sync::{Mutex, MutexGuard, OnceLock},
    };

    use super::{
        SearchAction, SearchPolicy, base64_encode, filesystem_results_in,
        filesystem_results_in_with_policy, index_results_with_policy, normalize_path, path_key,
        pattern_matches, pseudo_uuid, push_search_root, search_results, url_encode,
        workspace_results_in,
    };

    fn index_test_lock() -> MutexGuard<'static, ()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(())).lock().unwrap()
    }

    #[test]
    fn command_results_are_real_operations() {
        let results = search_results("json {\"a\":1}");
        assert_eq!(results[0].title, "格式化 JSON");
        assert!(results[0].detail.contains("\"a\": 1"));
        assert_eq!(base64_encode(b"hello"), "aGVsbG8=");
        assert_eq!(url_encode("a b"), "a%20b");
    }

    #[test]
    fn workspace_results_find_nested_files() {
        let root = std::env::temp_dir().join(format!("ariadne-search-{}", pseudo_uuid()));
        let file = root.join("nested").join("deeper").join("needle.txt");
        fs::create_dir_all(file.parent().unwrap()).unwrap();
        fs::write(&file, "test").unwrap();

        let results = workspace_results_in(&root, "needle");
        let result = results
            .iter()
            .find(|result| result.title == "needle.txt")
            .expect("nested file should be searchable");
        assert_eq!(result.detail, file.display().to_string());
        assert_eq!(result.action_label, "打开文件");
        assert!(matches!(result.action, SearchAction::OpenPath(ref path) if path == &file));

        let mut seen_paths = HashSet::from([path_key(&file)]);
        assert!(
            filesystem_results_in(&root, "needle", 12, "当前工作目录", &mut seen_paths).is_empty()
        );

        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn common_extension_patterns_are_excluded() {
        assert!(pattern_matches(
            r"c:\\temp\\draft.tmp",
            r"(?i)\.(tmp|part)$"
        ));
        assert!(!pattern_matches(
            r"c:\\docs\\draft.txt",
            r"(?i)\.(tmp|part)$"
        ));
    }
    #[test]
    fn configured_exclusion_path_is_not_descended() {
        let root = std::env::temp_dir().join(format!("ariadne-search-exclude-{}", pseudo_uuid()));
        let ignored = root.join("ignored").join("needle.txt");
        let visible = root.join("visible").join("needle.txt");
        fs::create_dir_all(ignored.parent().unwrap()).unwrap();
        fs::create_dir_all(visible.parent().unwrap()).unwrap();
        fs::write(&ignored, "test").unwrap();
        fs::write(&visible, "test").unwrap();
        let policy = SearchPolicy {
            exclude_folders: vec![normalize_path(&root.join("ignored").display().to_string())],
            ..Default::default()
        };
        let mut seen = HashSet::new();
        let results = filesystem_results_in_with_policy(
            &root,
            "needle",
            12,
            "当前工作目录",
            &mut seen,
            &policy,
        );
        assert!(
            results
                .iter()
                .any(|result| result.detail == visible.display().to_string())
        );
        assert!(
            !results
                .iter()
                .any(|result| result.detail == ignored.display().to_string())
        );
        fs::remove_dir_all(root).unwrap();
    }
    #[test]
    fn indexed_tsv_results_use_legacy_score_and_policy() {
        let root = std::env::temp_dir().join(format!("ariadne-search-index-{}", pseudo_uuid()));
        fs::create_dir_all(&root).unwrap();
        let index = root.join("file-index.tsv");
        fs::write(
            &index,
            "needle.txt\t0\tC:\\Docs\\needle.txt\nneedle.txt.bak\t0\tC:\\Docs\\needle.txt.bak\nxneedle.txt\t0\tC:\\Docs\\xneedle.txt\nneedle.txt.log\t0\tC:\\Docs\\needle.txt.log\nneedle.txt.tmp\t0\tC:\\Docs\\needle.txt.tmp\n",
        )
        .unwrap();
        let _guard = index_test_lock();
        unsafe { std::env::set_var("ARIADNE_FILE_INDEX", &index) };
        let policy = SearchPolicy {
            exclude_extensions: vec![".log".into()],
            exclude_patterns: vec!["*.tmp".into()],
            ..Default::default()
        };
        let mut seen = HashSet::new();
        let results = index_results_with_policy("needle.txt", &mut seen, 10, &policy);
        unsafe { std::env::remove_var("ARIADNE_FILE_INDEX") };
        fs::remove_dir_all(root).unwrap();

        let titles = results
            .iter()
            .map(|result| result.title.as_str())
            .collect::<Vec<_>>();
        assert_eq!(titles, ["needle.txt", "needle.txt.bak", "xneedle.txt"]);
        assert!(results.iter().all(|result| {
            matches!(result.action, SearchAction::OpenPath(ref path) if path.display().to_string().ends_with(result.title.as_str()))
        }));
    }

    #[test]
    fn indexed_tsv_keeps_chinese_calendar_after_invalid_utf8() {
        let root = std::env::temp_dir().join(format!("ariadne-search-calendar-{}", pseudo_uuid()));
        fs::create_dir_all(&root).unwrap();
        let index = root.join("file-index.tsv");
        let mut fixture = vec![0xff, b'\t', b'0', b'\t'];
        fixture.extend_from_slice(b"C:\\Bad\\broken.bin\n");
        fixture.extend_from_slice(
            "工作日历.xlsx\t0\tC:\\Users\\luwei\\Documents\\工作日历.xlsx\n".as_bytes(),
        );
        fs::write(&index, fixture).unwrap();
        let _guard = index_test_lock();
        unsafe { std::env::set_var("ARIADNE_FILE_INDEX", &index) };
        let mut seen = HashSet::new();
        let results =
            index_results_with_policy("工作日历", &mut seen, 10, &SearchPolicy::default());
        unsafe { std::env::remove_var("ARIADNE_FILE_INDEX") };
        fs::remove_dir_all(root).unwrap();

        let result = results
            .iter()
            .find(|result| result.title == "工作日历.xlsx")
            .expect("Chinese calendar row should remain searchable after malformed UTF-8");
        assert_eq!(result.detail, r"C:\Users\luwei\Documents\工作日历.xlsx");
        assert!(
            matches!(result.action, SearchAction::OpenPath(ref path) if path.display().to_string() == result.detail)
        );
    }

    #[test]
    fn search_roots_deduplicate_exact_paths() {
        let root = PathBuf::from("C:\\");
        let mut roots = Vec::new();
        push_search_root(&mut roots, root.clone());
        push_search_root(&mut roots, root.clone());
        assert_eq!(roots, vec![root]);
    }
}
