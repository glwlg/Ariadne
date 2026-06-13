from __future__ import annotations

import copy
import hashlib
import json
import os
import re
import shutil
import time
import uuid
import zipfile
from collections import Counter, defaultdict
from datetime import datetime, timedelta
from difflib import SequenceMatcher
from pathlib import Path
from typing import Any

from PyQt6.QtCore import QObject, QRect, Qt, QTimer, pyqtSignal
from PyQt6.QtGui import QColor, QPainter, QPixmap
from PyQt6.QtWidgets import QApplication

from src.core.config import config_manager
from src.core.logger import get_logger


logger = get_logger(__name__)

APPDATA_DIR = os.getenv("APPDATA") or os.path.expanduser("~")
XTOOLS_DIR = os.path.join(APPDATA_DIR, "x-tools")
WORK_MEMORY_DIR = os.path.join(XTOOLS_DIR, "work_memory")
ENTRY_FILE = os.path.join(WORK_MEMORY_DIR, "entries.json")
IMAGE_DIR = os.path.join(WORK_MEMORY_DIR, "images")
THUMB_DIR = os.path.join(WORK_MEMORY_DIR, "thumbs")
EXPORT_DIR = os.path.join(WORK_MEMORY_DIR, "exports")
DRAFT_DIR = os.path.join(WORK_MEMORY_DIR, "drafts")
TASK_DIR = os.path.join(WORK_MEMORY_DIR, "agent_tasks")


DEFAULT_WORK_MEMORY_CONFIG = {
    "enabled": True,
    "time_machine_enabled": False,
    "auto_capture_interval_seconds": 300,
    "capture_scope": "all_screens",
    "screenshot_quality": 90,
    "multi_monitor": "combined",
    "source_clipboard": True,
    "source_capture_history": True,
    "source_manual_note": True,
    "source_search_favorite": True,
    "source_actions": True,
    "auto_ocr": False,
    "embedding_enabled": False,
    "ai_enabled": False,
    "ai_provider": "disabled",
    "ai_base_url": "",
    "ai_model": "",
    "embedding_provider": "disabled",
    "embedding_base_url": "",
    "embedding_model": "",
    "vector_store_type": "disabled",
    "vector_store_uri": "",
    "vector_collection": "x_tools_work_memory",
    "opscore_sync_enabled": False,
    "agents_sdk_enabled": False,
    "trace_mode": "off",
    "experience_discovery_enabled": True,
    "experience_discovery_period_days": 7,
    "skill_suggestion_enabled": True,
    "workflow_suggestion_enabled": True,
    "external_agent_enabled": True,
    "codex_collaboration_enabled": False,
    "external_agent_task_dir": TASK_DIR,
    "retention_days": 30,
    "thumbnail_retention_days": 90,
    "max_storage_mb": 1024,
    "keep_favorites_forever": True,
    "privacy_mode": False,
    "pause_on_idle": True,
    "idle_pause_seconds": 600,
    "pause_on_lock": True,
    "exclude_apps": [
        "1password.exe",
        "bitwarden.exe",
        "keepass.exe",
        "lastpass.exe",
        "credentialuibroker.exe",
        "lockapp.exe",
        "logonui.exe",
        "mstsc.exe",
        "remotehelp.exe",
    ],
    "exclude_window_keywords": [
        "password",
        "passwd",
        "token",
        "secret",
        "otp",
        "验证码",
        "密码",
        "登录",
        "登陆",
        "支付",
        "付款",
        "隐私",
        "无痕",
        "private",
        "incognito",
        "remote desktop",
        "远程桌面",
        "堡垒机",
        "vpn",
        "sso",
    ],
    "exclude_paths": [],
    "exclude_content_patterns": [],
    "sensitive_rules_enabled": True,
    "allow_sensitive_export": False,
}


SOURCE_LABELS = {
    "time_machine": "屏幕时间机器",
    "capture": "截图历史",
    "clipboard": "剪贴板",
    "note": "手动笔记",
    "favorite": "手动收藏",
    "file": "文件",
    "import": "导入材料",
    "action": "操作轨迹",
    "daily_report": "日报草稿",
    "knowledge_draft": "知识草稿",
    "retro_draft": "复盘草稿",
    "experience_report": "经验发现",
    "workflow_suggestion": "工作流建议",
    "skill_suggestion": "Skill 建议",
    "task_package": "外部代理任务包",
}


TYPE_LABELS = {
    "text": "文本",
    "code": "代码",
    "command": "命令",
    "error_log": "错误日志",
    "json": "JSON",
    "yaml": "YAML",
    "sql": "SQL",
    "url": "URL",
    "ip_port": "IP/端口",
    "file_path": "文件路径",
    "image": "图片",
    "screenshot": "截图",
    "qr": "二维码",
    "ocr_text": "OCR 文本",
    "document": "文档",
    "table": "表格",
    "todo": "待办",
    "issue": "问题记录",
    "note": "笔记",
    "daily_report": "日报",
    "knowledge_draft": "知识草稿",
    "retro_draft": "复盘草稿",
    "experience_report": "经验发现",
    "workflow_suggestion": "工作流建议",
    "skill_suggestion": "Skill 建议",
    "task_package": "任务包",
}


CONTENT_SYNONYMS = {
    "报错": {"error", "exception", "traceback", "错误", "失败", "日志"},
    "错误": {"error", "exception", "traceback", "失败", "报错"},
    "截图": {"screenshot", "capture", "screen", "图片", "图像"},
    "图片": {"image", "screenshot", "截图", "图像"},
    "剪贴板": {"clipboard", "copy", "复制", "粘贴"},
    "复制": {"clipboard", "剪贴板", "copy"},
    "命令": {"command", "shell", "powershell", "cmd"},
    "接口": {"api", "url", "endpoint", "路径"},
    "日报": {"daily", "report", "summary", "总结"},
    "知识": {"knowledge", "draft", "wiki", "经验"},
    "任务包": {"agent", "codex", "task", "外部代理"},
    "工作流": {"workflow", "macro", "自动化", "重复"},
    "敏感": {"password", "token", "secret", "cookie", "authorization"},
}


SENSITIVE_PATTERNS = [
    ("password", re.compile(r"(?i)\b(pass(word)?|pwd)\b\s*[:=]\s*\S+")),
    ("token", re.compile(r"(?i)\b(token|access_token|refresh_token)\b\s*[:=]\s*\S+")),
    ("api_key", re.compile(r"(?i)\b(api[-_ ]?key|apikey)\b\s*[:=]\s*\S+")),
    ("secret", re.compile(r"(?i)\b(secret|client_secret)\b\s*[:=]\s*\S+")),
    ("cookie", re.compile(r"(?i)\bcookie\s*[:=]\s*[^;\n]+")),
    ("authorization", re.compile(r"(?i)\bauthorization\s*:\s*(bearer|basic)\s+\S+")),
    ("ssh_key", re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----")),
    (
        "db_connection",
        re.compile(r"(?i)\b(postgres|postgresql|mysql|mongodb|redis)://[^\s]+"),
    ),
    ("openai_key", re.compile(r"\bsk-[A-Za-z0-9_-]{16,}\b")),
    (
        "internal_address",
        re.compile(
            r"\b(10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[0-1])(?:\.\d{1,3}){2})\b"
        ),
    ),
]


def work_memory_config() -> dict[str, Any]:
    raw = config_manager.get_value("work_memory", {})
    merged = copy.deepcopy(DEFAULT_WORK_MEMORY_CONFIG)
    if isinstance(raw, dict):
        merged.update(raw)
    return merged


def update_work_memory_config(values: dict[str, Any]) -> dict[str, Any]:
    current = work_memory_config()
    current.update(values)
    config_manager.set_value("work_memory", current)
    return current


def _now() -> float:
    return time.time()


def _safe_float(value, default=0.0) -> float:
    try:
        return float(value)
    except Exception:
        return default


def _safe_int(value, default=0) -> int:
    try:
        return int(value)
    except Exception:
        return default


def _format_time(ts: float | int | str) -> str:
    try:
        return datetime.fromtimestamp(float(ts)).strftime("%Y-%m-%d %H:%M:%S")
    except Exception:
        return ""


def _read_text_file(path: str, limit=120_000) -> str:
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            return f.read(limit)
    except Exception:
        return ""


def _write_text_file(path: str, text: str) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


def _entry_source_label(entry: dict[str, Any]) -> str:
    return SOURCE_LABELS.get(str(entry.get("source", "")), str(entry.get("source", "")))


def _entry_type_label(entry: dict[str, Any]) -> str:
    return TYPE_LABELS.get(
        str(entry.get("content_type", "")), str(entry.get("content_type", ""))
    )


def _contains_json(text: str) -> bool:
    value = text.strip()
    if not value or value[0] not in "[{":
        return False
    try:
        json.loads(value)
        return True
    except Exception:
        return False


def classify_content(text: str, image_path: str = "", source: str = "") -> str:
    lowered = str(text or "").strip().lower()
    if image_path:
        return "screenshot" if source in {"capture", "time_machine"} else "image"
    if not lowered:
        return "text"
    if _contains_json(text):
        return "json"
    if re.search(r"(?im)^\s*(select|insert|update|delete|with|create|alter)\b", text):
        return "sql"
    if re.search(r"(?im)^\s*[-\w.]+\s*:\s*.+$", text) and not re.search(
        r"https?://", text
    ):
        return "yaml"
    if re.search(r"https?://[^\s]+", text):
        return "url"
    if re.search(r"\b\d{1,3}(?:\.\d{1,3}){3}(?::\d{2,5})?\b", text):
        return "ip_port"
    if re.search(r"(?i)([a-z]:\\|/home/|/var/|\\\\)[^\n]+", text):
        return "file_path"
    if re.search(r"(?im)\b(traceback|exception|error|failed|fatal|报错|错误|失败)\b", text):
        return "error_log"
    if re.search(r"(?im)^\s*(todo|fixme|待办|未完成)\b", text):
        return "todo"
    if re.search(r"(?im)^\s*(git|uv|python|pip|npm|pnpm|yarn|docker|kubectl|ssh|scp)\b", text):
        return "command"
    if re.search(r"(?m)^\s*(def|class|function|const|let|var|import|from)\b", text):
        return "code"
    return "text"


def detect_sensitive(text: str) -> tuple[bool, list[str]]:
    value = str(text or "")
    flags = []
    for label, pattern in SENSITIVE_PATTERNS:
        if pattern.search(value):
            flags.append(label)
    return bool(flags), flags


class WorkMemoryManager(QObject):
    entries_changed = pyqtSignal()
    status_changed = pyqtSignal()

    def __init__(self):
        super().__init__()
        self._entries: list[dict[str, Any]] = []
        self._timer: QTimer | None = None
        self._is_running = False
        self._last_capture_at = 0.0
        self._last_pause_reason = ""
        self._last_screen_signature = ""
        self._clipboard_manager = None
        self._capture_manager = None
        self._source_connections: set[str] = set()
        self._ocr_engine = None
        self._ocr_error = ""
        self._load()

    def _ensure_dirs(self) -> None:
        for path in [WORK_MEMORY_DIR, IMAGE_DIR, THUMB_DIR, EXPORT_DIR, DRAFT_DIR, TASK_DIR]:
            os.makedirs(path, exist_ok=True)

    def _load(self) -> None:
        self._ensure_dirs()
        if not os.path.exists(ENTRY_FILE):
            self._entries = []
            return
        try:
            with open(ENTRY_FILE, "r", encoding="utf-8") as f:
                raw = json.load(f)
            if not isinstance(raw, list):
                self._entries = []
                return
            self._entries = [self._normalize_entry(item) for item in raw if isinstance(item, dict)]
        except Exception as exc:
            logger.warning("Failed to load work memory entries: %s", exc)
            self._entries = []

    def _save(self) -> None:
        self._ensure_dirs()
        try:
            with open(ENTRY_FILE, "w", encoding="utf-8") as f:
                json.dump(self._entries, f, ensure_ascii=False, indent=2)
        except Exception as exc:
            logger.warning("Failed to save work memory entries: %s", exc)

    def _normalize_entry(self, raw: dict[str, Any]) -> dict[str, Any]:
        created_at = _safe_float(raw.get("created_at"), _now())
        text = str(raw.get("text", ""))
        ocr_text = str(raw.get("ocr_text", ""))
        image_path = str(raw.get("image_path", "")).strip()
        source = str(raw.get("source", "note")).strip() or "note"
        content_type = str(raw.get("content_type", "")).strip()
        if not content_type:
            content_type = classify_content(text or ocr_text, image_path=image_path, source=source)

        risk_flags = [str(item) for item in raw.get("risk_flags", []) if str(item).strip()]
        sensitive = bool(raw.get("sensitive", False))
        if not sensitive and (text or ocr_text):
            sensitive, risk_flags = detect_sensitive("\n".join([text, ocr_text]))

        tags = [str(item).strip() for item in raw.get("tags", []) if str(item).strip()]
        for tag in self._tags_for_type(content_type):
            if tag not in tags:
                tags.append(tag)
        for flag in risk_flags:
            tag = f"risk:{flag}"
            if tag not in tags:
                tags.append(tag)

        entry = {
            "id": str(raw.get("id", "")).strip() or str(uuid.uuid4()),
            "created_at": created_at,
            "updated_at": _safe_float(raw.get("updated_at"), created_at),
            "event_time": _safe_float(raw.get("event_time"), created_at),
            "last_accessed_at": _safe_float(raw.get("last_accessed_at"), 0),
            "source": source,
            "source_id": str(raw.get("source_id", "")).strip(),
            "source_ref": str(raw.get("source_ref", "")).strip(),
            "content_type": content_type,
            "title": str(raw.get("title", "")).strip(),
            "summary": str(raw.get("summary", "")).strip(),
            "text": text,
            "ocr_text": ocr_text,
            "image_path": image_path,
            "thumbnail_path": str(raw.get("thumbnail_path", "")).strip(),
            "file_path": str(raw.get("file_path", "")).strip(),
            "app_name": str(raw.get("app_name", "")).strip(),
            "process_name": str(raw.get("process_name", "")).strip(),
            "window_title": str(raw.get("window_title", "")).strip(),
            "trigger_command": str(raw.get("trigger_command", "")).strip(),
            "query": str(raw.get("query", "")).strip(),
            "tags": tags,
            "favorite": bool(raw.get("favorite", False)),
            "pinned": bool(raw.get("pinned", False)),
            "hidden": bool(raw.get("hidden", False)),
            "sensitive": sensitive,
            "ai_allowed": bool(raw.get("ai_allowed", not sensitive)),
            "vector_allowed": bool(raw.get("vector_allowed", not sensitive)),
            "export_allowed": bool(raw.get("export_allowed", True)),
            "included_in_daily": bool(raw.get("included_in_daily", False)),
            "knowledge_generated": bool(raw.get("knowledge_generated", False)),
            "status": str(raw.get("status", "")).strip(),
            "risk_flags": risk_flags,
            "relations": [str(item) for item in raw.get("relations", []) if str(item).strip()],
            "metadata": raw.get("metadata", {}) if isinstance(raw.get("metadata"), dict) else {},
        }
        if not entry["title"]:
            entry["title"] = self._build_title(entry)
        if not entry["summary"]:
            entry["summary"] = self._build_summary(entry)
        return entry

    @staticmethod
    def _tags_for_type(content_type: str) -> list[str]:
        tags = []
        if content_type in TYPE_LABELS:
            tags.append(content_type)
        if content_type in {"error_log", "issue"}:
            tags.append("issue")
        if content_type in {"screenshot", "image", "ocr_text"}:
            tags.append("visual")
        if content_type in {"command", "code", "json", "sql", "yaml"}:
            tags.append("technical")
        return tags

    @staticmethod
    def _preview_text(value: str, limit=120) -> str:
        text = " ".join(str(value or "").replace("\r", " ").replace("\n", " ").split())
        if len(text) > limit:
            return text[: limit - 3] + "..."
        return text

    def _build_title(self, entry: dict[str, Any]) -> str:
        source_label = _entry_source_label(entry)
        type_label = _entry_type_label(entry)
        text = entry.get("text") or entry.get("ocr_text") or entry.get("summary")
        if text:
            return f"{source_label} · {self._preview_text(text, 72)}"
        if entry.get("file_path"):
            return f"{source_label} · {os.path.basename(str(entry.get('file_path')))}"
        if entry.get("image_path"):
            return f"{source_label} · {type_label}"
        return f"{source_label} · {type_label}"

    def _build_summary(self, entry: dict[str, Any]) -> str:
        text = entry.get("text") or entry.get("ocr_text")
        if text:
            return self._preview_text(text, 220)
        parts = []
        if entry.get("window_title"):
            parts.append(f"窗口: {entry.get('window_title')}")
        if entry.get("app_name") or entry.get("process_name"):
            parts.append(f"应用: {entry.get('app_name') or entry.get('process_name')}")
        if entry.get("file_path"):
            parts.append(f"文件: {entry.get('file_path')}")
        if entry.get("image_path"):
            meta = entry.get("metadata", {})
            size = meta.get("size") or ""
            parts.append(f"图片: {size}".strip())
        return "\n".join(parts).strip()

    def _existing_by_ref(self, source_ref: str) -> dict[str, Any] | None:
        if not source_ref:
            return None
        for entry in self._entries:
            if entry.get("source_ref") == source_ref:
                return entry
        return None

    def add_entry(self, payload: dict[str, Any], *, force=False) -> dict[str, Any] | None:
        cfg = work_memory_config()
        if not force and not cfg.get("enabled", True):
            return None
        excluded = self._payload_exclusion_reason(payload)
        if excluded:
            self._last_pause_reason = excluded
            self.status_changed.emit()
            return None

        entry = self._normalize_entry(payload)
        source_ref = str(entry.get("source_ref", "")).strip()
        existing = self._existing_by_ref(source_ref)
        if existing is not None:
            changed = False
            for key in [
                "title",
                "summary",
                "text",
                "ocr_text",
                "image_path",
                "thumbnail_path",
                "file_path",
                "app_name",
                "process_name",
                "window_title",
                "metadata",
            ]:
                if entry.get(key) and existing.get(key) != entry.get(key):
                    existing[key] = entry.get(key)
                    changed = True
            if changed:
                existing["updated_at"] = _now()
                self._save()
                self.entries_changed.emit()
            return dict(existing)

        self._entries.insert(0, entry)
        self._save()
        self.entries_changed.emit()
        return dict(entry)

    def _payload_exclusion_reason(self, payload: dict[str, Any]) -> str:
        cfg = work_memory_config()
        paths = [
            str(payload.get("file_path", "")).strip(),
            str(payload.get("image_path", "")).strip(),
            str(payload.get("app_name", "")).strip(),
        ]
        excluded_paths = [str(item).strip().lower() for item in cfg.get("exclude_paths", [])]
        for path in paths:
            lowered = path.lower()
            for excluded in excluded_paths:
                if excluded and excluded in lowered:
                    return f"命中排除路径: {excluded}"

        text = "\n".join(
            [
                str(payload.get("text", "")),
                str(payload.get("ocr_text", "")),
                str(payload.get("summary", "")),
                str(payload.get("title", "")),
            ]
        )
        for pattern in cfg.get("exclude_content_patterns", []):
            expr = str(pattern).strip()
            if not expr:
                continue
            try:
                if re.search(expr, text, re.IGNORECASE):
                    return f"命中排除内容规则: {expr}"
            except re.error:
                if expr.lower() in text.lower():
                    return f"命中排除内容规则: {expr}"
        return ""

    def get_entries(self, include_hidden=False) -> list[dict[str, Any]]:
        entries = self._entries if include_hidden else [e for e in self._entries if not e.get("hidden")]
        return [copy.deepcopy(entry) for entry in entries]

    def get_entry(self, entry_id: str) -> dict[str, Any] | None:
        key = str(entry_id).strip()
        if not key:
            return None
        for entry in self._entries:
            if entry.get("id") == key:
                return copy.deepcopy(entry)
        return None

    def _mutate_entry(self, entry_id: str, mutator) -> dict[str, Any] | None:
        key = str(entry_id).strip()
        if not key:
            return None
        for entry in self._entries:
            if entry.get("id") == key:
                mutator(entry)
                entry["updated_at"] = _now()
                self._save()
                self.entries_changed.emit()
                return copy.deepcopy(entry)
        return None

    def toggle_favorite(self, entry_id: str) -> bool:
        updated = self._mutate_entry(
            entry_id, lambda entry: entry.__setitem__("favorite", not bool(entry.get("favorite")))
        )
        return bool(updated and updated.get("favorite"))

    def toggle_sensitive(self, entry_id: str) -> bool:
        def mutate(entry):
            entry["sensitive"] = not bool(entry.get("sensitive"))
            if entry["sensitive"]:
                entry["ai_allowed"] = False
                entry["vector_allowed"] = False
                if "manual_sensitive" not in entry["risk_flags"]:
                    entry["risk_flags"].append("manual_sensitive")
            else:
                entry["risk_flags"] = [
                    flag for flag in entry.get("risk_flags", []) if flag != "manual_sensitive"
                ]

        updated = self._mutate_entry(entry_id, mutate)
        return bool(updated and updated.get("sensitive"))

    def set_ai_allowed(self, entry_id: str, allowed: bool) -> bool:
        updated = self._mutate_entry(
            entry_id,
            lambda entry: entry.__setitem__(
                "ai_allowed", bool(allowed) and not bool(entry.get("sensitive"))
            ),
        )
        return bool(updated and updated.get("ai_allowed"))

    def add_tags(self, entry_id: str, tags: list[str]) -> bool:
        normalized = [str(tag).strip() for tag in tags if str(tag).strip()]
        if not normalized:
            return False

        def mutate(entry):
            current = list(entry.get("tags", []))
            for tag in normalized:
                if tag not in current:
                    current.append(tag)
            entry["tags"] = current

        return self._mutate_entry(entry_id, mutate) is not None

    def mark_included_in_daily(self, entry_ids: list[str]) -> None:
        keys = set(str(item).strip() for item in entry_ids if str(item).strip())
        for entry in self._entries:
            if entry.get("id") in keys:
                entry["included_in_daily"] = True
                entry["updated_at"] = _now()
        self._save()
        self.entries_changed.emit()

    def mark_knowledge_generated(self, entry_ids: list[str]) -> None:
        keys = set(str(item).strip() for item in entry_ids if str(item).strip())
        for entry in self._entries:
            if entry.get("id") in keys:
                entry["knowledge_generated"] = True
                entry["updated_at"] = _now()
        self._save()
        self.entries_changed.emit()

    def delete_entry(self, entry_id: str) -> bool:
        key = str(entry_id).strip()
        if not key:
            return False
        for index, entry in enumerate(self._entries):
            if entry.get("id") == key:
                removed = self._entries.pop(index)
                self._safe_remove_owned_file(str(removed.get("image_path", "")))
                self._safe_remove_owned_file(str(removed.get("thumbnail_path", "")))
                self._safe_remove_owned_file(str(removed.get("file_path", "")), only_drafts=True)
                self._save()
                self.entries_changed.emit()
                return True
        return False

    @staticmethod
    def _safe_remove_owned_file(path: str, *, only_drafts=False) -> None:
        if not path:
            return
        try:
            target = os.path.abspath(path)
            allowed_roots = [os.path.abspath(WORK_MEMORY_DIR)]
            if only_drafts:
                allowed_roots = [os.path.abspath(DRAFT_DIR), os.path.abspath(TASK_DIR)]
            if any(target.startswith(root + os.sep) for root in allowed_roots) and os.path.exists(target):
                os.remove(target)
        except Exception:
            pass

    def clear_unfavorited(self, before_ts: float | None = None) -> int:
        cfg = work_memory_config()
        keep_favorites = bool(cfg.get("keep_favorites_forever", True))
        removed = []
        kept = []
        for entry in self._entries:
            if keep_favorites and entry.get("favorite"):
                kept.append(entry)
                continue
            if before_ts is not None and float(entry.get("created_at", 0) or 0) >= before_ts:
                kept.append(entry)
                continue
            removed.append(entry)
        if not removed:
            return 0
        self._entries = kept
        for entry in removed:
            self._safe_remove_owned_file(str(entry.get("image_path", "")))
            self._safe_remove_owned_file(str(entry.get("thumbnail_path", "")))
        self._save()
        self.entries_changed.emit()
        return len(removed)

    def apply_retention_policy(self) -> int:
        cfg = work_memory_config()
        days = max(1, _safe_int(cfg.get("retention_days"), 30))
        cutoff = _now() - days * 86400
        return self.clear_unfavorited(before_ts=cutoff)

    def _context_text(self, entry: dict[str, Any]) -> str:
        metadata = entry.get("metadata", {}) if isinstance(entry.get("metadata"), dict) else {}
        values = [
            entry.get("title", ""),
            entry.get("summary", ""),
            entry.get("text", ""),
            entry.get("ocr_text", ""),
            entry.get("file_path", ""),
            entry.get("image_path", ""),
            entry.get("app_name", ""),
            entry.get("process_name", ""),
            entry.get("window_title", ""),
            entry.get("trigger_command", ""),
            entry.get("query", ""),
            " ".join(entry.get("tags", [])),
            " ".join(str(v) for v in metadata.values() if isinstance(v, (str, int, float))),
            _entry_source_label(entry),
            _entry_type_label(entry),
        ]
        return "\n".join(str(value) for value in values if str(value).strip())

    @staticmethod
    def _tokens(text: str) -> set[str]:
        value = str(text or "").lower()
        raw_tokens = set(re.findall(r"[a-z0-9_./:\\-]{2,}|[\u4e00-\u9fff]{1,}", value))
        expanded = set(raw_tokens)
        for token in list(raw_tokens):
            if token in CONTENT_SYNONYMS:
                expanded.update(CONTENT_SYNONYMS[token])
        return expanded

    def _match_score(self, entry: dict[str, Any], query: str) -> tuple[float, str]:
        if not query.strip():
            return 1.0, "按时间排序"
        haystack = self._context_text(entry)
        query_lower = query.lower()
        hay_lower = haystack.lower()
        score = 0.0
        reasons = []
        if query_lower in hay_lower:
            score += 120
            reasons.append("关键词直接命中")
        query_tokens = self._tokens(query)
        hay_tokens = self._tokens(haystack)
        overlap = query_tokens & hay_tokens
        if overlap:
            score += len(overlap) * 18
            reasons.append("词语/同义意图匹配: " + ", ".join(sorted(list(overlap))[:5]))
        title_ratio = SequenceMatcher(None, query_lower, str(entry.get("title", "")).lower()).ratio()
        summary_ratio = SequenceMatcher(
            None, query_lower, str(entry.get("summary", "")).lower()
        ).ratio()
        fuzzy = max(title_ratio, summary_ratio)
        if fuzzy >= 0.38:
            score += fuzzy * 45
            reasons.append("自然语言近似匹配")
        if entry.get("favorite"):
            score += 12
        if entry.get("last_accessed_at"):
            score += 3
        if not reasons:
            return 0.0, ""
        return score, "；".join(reasons)

    def search(
        self,
        query: str = "",
        *,
        source: str = "",
        content_type: str = "",
        app: str = "",
        tag: str = "",
        favorite: bool | None = None,
        included_in_daily: bool | None = None,
        knowledge_generated: bool | None = None,
        limit: int = 100,
    ) -> list[dict[str, Any]]:
        results = []
        q = str(query or "").strip()
        for entry in self._entries:
            if entry.get("hidden"):
                continue
            if source and entry.get("source") != source:
                continue
            if content_type and entry.get("content_type") != content_type:
                continue
            if app:
                app_text = f"{entry.get('app_name', '')} {entry.get('process_name', '')}".lower()
                if app.lower() not in app_text:
                    continue
            if tag and tag not in entry.get("tags", []):
                continue
            if favorite is not None and bool(entry.get("favorite")) != bool(favorite):
                continue
            if included_in_daily is not None and bool(entry.get("included_in_daily")) != bool(
                included_in_daily
            ):
                continue
            if knowledge_generated is not None and bool(entry.get("knowledge_generated")) != bool(
                knowledge_generated
            ):
                continue

            score, reason = self._match_score(entry, q)
            if q and score <= 0:
                continue
            item = copy.deepcopy(entry)
            item["match_score"] = score
            item["match_reason"] = reason
            results.append(item)

        results.sort(
            key=lambda item: (
                -float(item.get("match_score", 0)),
                0 if item.get("favorite") else 1,
                -float(item.get("created_at", 0)),
            )
        )
        return results[: max(1, int(limit or 1))]

    def as_search_results(self, query: str = "", limit=25) -> list[dict[str, Any]]:
        rows = []
        for entry in self.search(query, limit=limit):
            time_text = _format_time(entry.get("created_at", 0))
            prefix = "★ " if entry.get("favorite") else ""
            sensitive = " [敏感]" if entry.get("sensitive") else ""
            rows.append(
                {
                    "type": "work_memory_entry",
                    "name": f"{prefix}{entry.get('title', '工作记忆')}{sensitive}",
                    "path": str(entry.get("id", "")),
                    "work_memory_id": str(entry.get("id", "")),
                    "work_memory_source": str(entry.get("source", "")),
                    "work_memory_source_label": _entry_source_label(entry),
                    "work_memory_content_type": str(entry.get("content_type", "")),
                    "work_memory_type_label": _entry_type_label(entry),
                    "work_memory_summary": str(entry.get("summary", "")),
                    "work_memory_text": str(entry.get("text", "")),
                    "work_memory_ocr_text": str(entry.get("ocr_text", "")),
                    "work_memory_image_path": str(entry.get("image_path", "")),
                    "work_memory_thumbnail_path": str(entry.get("thumbnail_path", "")),
                    "work_memory_file_path": str(entry.get("file_path", "")),
                    "work_memory_created_at": float(entry.get("created_at", 0) or 0),
                    "work_memory_time": time_text,
                    "work_memory_window_title": str(entry.get("window_title", "")),
                    "work_memory_app_name": str(entry.get("app_name", "")),
                    "work_memory_tags": list(entry.get("tags", [])),
                    "work_memory_sensitive": bool(entry.get("sensitive", False)),
                    "work_memory_favorite": bool(entry.get("favorite", False)),
                    "work_memory_match_reason": str(entry.get("match_reason", "")),
                }
            )
        return rows

    def source_options(self) -> list[tuple[str, str]]:
        seen = sorted(set(str(entry.get("source", "")) for entry in self._entries if entry.get("source")))
        return [(key, SOURCE_LABELS.get(key, key)) for key in seen]

    def type_options(self) -> list[tuple[str, str]]:
        seen = sorted(
            set(str(entry.get("content_type", "")) for entry in self._entries if entry.get("content_type"))
        )
        return [(key, TYPE_LABELS.get(key, key)) for key in seen]

    def tag_options(self) -> list[str]:
        tags = sorted(set(tag for entry in self._entries for tag in entry.get("tags", [])))
        return tags

    def add_manual_note(self, text: str, *, tags: list[str] | None = None) -> dict[str, Any] | None:
        content = str(text or "").strip()
        if not content:
            return None
        sensitive, flags = detect_sensitive(content)
        return self.add_entry(
            {
                "source": "note",
                "source_ref": f"note:{uuid.uuid4()}",
                "content_type": classify_content(content),
                "title": "手动笔记 · " + self._preview_text(content, 60),
                "summary": self._preview_text(content, 220),
                "text": content,
                "tags": tags or ["note"],
                "sensitive": sensitive,
                "risk_flags": flags,
                "ai_allowed": not sensitive,
                "vector_allowed": not sensitive,
            },
            force=True,
        )

    def add_action_record(
        self,
        action: str,
        *,
        query: str = "",
        result_title: str = "",
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, Any] | None:
        command = str(action or "").strip()
        if not command:
            return None
        return self.add_entry(
            {
                "source": "action",
                "source_ref": f"action:{uuid.uuid4()}",
                "content_type": "command",
                "title": "操作轨迹 · " + self._preview_text(result_title or command, 72),
                "summary": f"动作: {command}\n查询: {query}".strip(),
                "text": command,
                "trigger_command": command,
                "query": query,
                "tags": ["action", "trace"],
                "metadata": metadata or {},
            }
        )

    def add_favorite_item(self, item: dict[str, Any], *, query: str = "") -> dict[str, Any] | None:
        if not isinstance(item, dict):
            return None
        item_type = str(item.get("type", "")).strip()
        source_ref = f"favorite:{item_type}:{item.get('path', '')}:{item.get('name', '')}"
        text = str(
            item.get("work_memory_text")
            or item.get("clipboard_text")
            or item.get("qr_text")
            or item.get("path")
            or item.get("name")
            or ""
        )
        image_path = str(
            item.get("work_memory_image_path")
            or item.get("capture_image_path")
            or item.get("clipboard_image_path")
            or ""
        ).strip()
        file_path = str(item.get("path", "")).strip()
        if item_type in {"file", "app", "custom_launch"}:
            text = file_path or text
        content_type = classify_content(text, image_path=image_path, source="favorite")
        sensitive, flags = detect_sensitive(text)
        return self.add_entry(
            {
                "source": "favorite",
                "source_ref": source_ref,
                "source_id": str(item.get("path", "")),
                "content_type": content_type,
                "title": "手动收藏 · " + str(item.get("name", "")).strip(),
                "summary": self._preview_text(text, 260),
                "text": text,
                "image_path": image_path,
                "file_path": file_path if os.path.exists(file_path) else "",
                "query": query,
                "favorite": True,
                "tags": ["favorite", item_type] if item_type else ["favorite"],
                "sensitive": sensitive,
                "risk_flags": flags,
                "ai_allowed": not sensitive,
                "vector_allowed": not sensitive,
                "metadata": {"original_item": self._json_safe_item(item)},
            },
            force=True,
        )

    @staticmethod
    def _json_safe_item(item: dict[str, Any]) -> dict[str, Any]:
        safe = {}
        for key, value in item.items():
            if key == "plugin":
                continue
            if isinstance(value, (str, int, float, bool)) or value is None:
                safe[key] = value
            elif isinstance(value, (list, tuple)):
                safe[key] = [str(v) for v in value]
            elif isinstance(value, dict):
                safe[key] = {str(k): str(v) for k, v in value.items()}
            else:
                safe[key] = str(value)
        return safe

    def import_file(self, path: str) -> dict[str, Any] | None:
        source_path = str(path or "").strip()
        if not source_path or not os.path.exists(source_path):
            return None
        suffix = os.path.splitext(source_path)[1].lower()
        text = ""
        image_path = ""
        copied_path = ""
        if suffix in {".txt", ".md", ".json", ".log", ".py", ".yaml", ".yml", ".toml", ".sql"}:
            text = _read_text_file(source_path)
        elif suffix in {".png", ".jpg", ".jpeg", ".bmp", ".webp"}:
            copied_path = self._copy_into_work_memory(source_path, IMAGE_DIR)
            image_path = copied_path
        else:
            text = os.path.basename(source_path)
        sensitive, flags = detect_sensitive(text)
        return self.add_entry(
            {
                "source": "import",
                "source_ref": f"import:{source_path}:{os.path.getmtime(source_path)}",
                "content_type": classify_content(text, image_path=image_path, source="import"),
                "title": "导入材料 · " + os.path.basename(source_path),
                "summary": self._preview_text(text or source_path, 240),
                "text": text,
                "image_path": image_path,
                "file_path": copied_path or source_path,
                "tags": ["import", suffix.strip(".")],
                "sensitive": sensitive,
                "risk_flags": flags,
                "ai_allowed": not sensitive,
                "vector_allowed": not sensitive,
            },
            force=True,
        )

    @staticmethod
    def _copy_into_work_memory(path: str, target_dir: str) -> str:
        os.makedirs(target_dir, exist_ok=True)
        suffix = os.path.splitext(path)[1] or ".bin"
        target = os.path.join(target_dir, f"{uuid.uuid4()}{suffix}")
        shutil.copy2(path, target)
        return target

    def add_clipboard_entry(self, entry: dict[str, Any]) -> dict[str, Any] | None:
        cfg = work_memory_config()
        if not cfg.get("source_clipboard", True):
            return None
        entry_id = str(entry.get("id", "")).strip()
        if not entry_id:
            return None
        entry_type = str(entry.get("type", "")).strip()
        text = str(entry.get("text", ""))
        image_path = str(entry.get("image_path", "")).strip()
        content_type = classify_content(text, image_path=image_path, source="clipboard")
        sensitive, flags = detect_sensitive(text)
        return self.add_entry(
            {
                "source": "clipboard",
                "source_id": entry_id,
                "source_ref": f"clipboard:{entry_id}",
                "event_time": _safe_float(entry.get("created_at"), _now()),
                "content_type": content_type,
                "title": ("剪贴板图片" if entry_type == "image" else "剪贴板文本"),
                "summary": self._preview_text(text, 240)
                if text
                else f"图片 {entry.get('width', 0)}x{entry.get('height', 0)}",
                "text": text,
                "image_path": image_path,
                "thumbnail_path": self._thumbnail_for_image(image_path),
                "tags": ["clipboard", entry_type],
                "favorite": bool(entry.get("pinned", False)),
                "sensitive": sensitive,
                "risk_flags": flags,
                "ai_allowed": not sensitive,
                "vector_allowed": not sensitive,
                "metadata": {
                    "size": f"{entry.get('width', 0)}x{entry.get('height', 0)}",
                    "signature": str(entry.get("signature", "")),
                },
            }
        )

    def add_capture_history_entry(self, entry: dict[str, Any]) -> dict[str, Any] | None:
        cfg = work_memory_config()
        if not cfg.get("source_capture_history", True):
            return None
        entry_id = str(entry.get("id", "")).strip()
        image_path = str(entry.get("image_path", "")).strip()
        if not entry_id or not image_path:
            return None
        actions = [str(action) for action in entry.get("actions", []) if str(action).strip()]
        return self.add_entry(
            {
                "source": "capture",
                "source_id": entry_id,
                "source_ref": f"capture:{entry_id}",
                "event_time": _safe_float(entry.get("created_at"), _now()),
                "content_type": "screenshot",
                "title": "截图历史 · " + _format_time(entry.get("created_at", 0)),
                "summary": " / ".join(actions) or "x-tools 截图",
                "image_path": image_path,
                "thumbnail_path": self._thumbnail_for_image(image_path),
                "file_path": str(entry.get("saved_path", "")).strip(),
                "tags": ["capture", "screenshot"],
                "favorite": bool(entry.get("pinned", False)),
                "metadata": {
                    "size": f"{entry.get('width', 0)}x{entry.get('height', 0)}",
                    "actions": actions,
                    "source": str(entry.get("source", "")),
                },
            }
        )

    def sync_clipboard_history(self, manager=None) -> int:
        if manager is None:
            try:
                from src.core.clipboard_history import clipboard_history_manager

                manager = clipboard_history_manager
            except Exception:
                return 0
        count = 0
        try:
            for entry in manager.get_entries(limit=500):
                if self.add_clipboard_entry(entry):
                    count += 1
        except Exception as exc:
            logger.warning("Failed to sync clipboard history into work memory: %s", exc)
        return count

    def sync_capture_history(self, manager=None) -> int:
        if manager is None:
            try:
                from src.core.capture_history import capture_history_manager

                manager = capture_history_manager
            except Exception:
                return 0
        count = 0
        try:
            for entry in manager.get_entries(limit=500):
                if self.add_capture_history_entry(entry):
                    count += 1
        except Exception as exc:
            logger.warning("Failed to sync capture history into work memory: %s", exc)
        return count

    def start(self, clipboard_manager=None, capture_manager=None) -> None:
        if clipboard_manager is not None:
            self._clipboard_manager = clipboard_manager
            if "clipboard" not in self._source_connections:
                try:
                    clipboard_manager.entries_changed.connect(
                        lambda: self.sync_clipboard_history(clipboard_manager)
                    )
                    self._source_connections.add("clipboard")
                except Exception:
                    pass
            self.sync_clipboard_history(clipboard_manager)

        if capture_manager is not None:
            self._capture_manager = capture_manager
            if "capture" not in self._source_connections:
                try:
                    capture_manager.entries_changed.connect(
                        lambda: self.sync_capture_history(capture_manager)
                    )
                    self._source_connections.add("capture")
                except Exception:
                    pass
            self.sync_capture_history(capture_manager)

        self.sync_from_config()

    def sync_from_config(self) -> None:
        cfg = work_memory_config()
        should_run = bool(cfg.get("enabled", True)) and bool(
            cfg.get("time_machine_enabled", False)
        )
        if should_run:
            self.start_time_machine()
        else:
            self.stop_time_machine(reason="屏幕时间机器已暂停")

    def set_enabled(self, enabled: bool) -> None:
        update_work_memory_config({"enabled": bool(enabled)})
        self.sync_from_config()
        self.status_changed.emit()

    def set_time_machine_enabled(self, enabled: bool) -> None:
        update_work_memory_config({"time_machine_enabled": bool(enabled)})
        self.sync_from_config()
        self.status_changed.emit()

    def set_privacy_mode(self, enabled: bool) -> None:
        update_work_memory_config({"privacy_mode": bool(enabled)})
        self.sync_from_config()
        self.status_changed.emit()

    def start_time_machine(self) -> None:
        app = QApplication.instance()
        if app is None:
            self._last_pause_reason = "Qt 应用尚未启动"
            self._is_running = False
            self.status_changed.emit()
            return
        cfg = work_memory_config()
        if cfg.get("privacy_mode", False):
            self._last_pause_reason = "隐私模式已开启"
            self._is_running = False
            self.status_changed.emit()
            return
        interval_seconds = max(10, _safe_int(cfg.get("auto_capture_interval_seconds"), 300))
        if self._timer is None:
            self._timer = QTimer(self)
            self._timer.timeout.connect(self._auto_capture_tick)
        self._timer.setInterval(interval_seconds * 1000)
        if not self._timer.isActive():
            self._timer.start()
        self._is_running = True
        self._last_pause_reason = ""
        self.status_changed.emit()

    def stop_time_machine(self, reason="") -> None:
        if self._timer is not None and self._timer.isActive():
            self._timer.stop()
        self._is_running = False
        if reason:
            self._last_pause_reason = reason
        self.status_changed.emit()

    def _auto_capture_tick(self) -> None:
        self.capture_current_screen(source="time_machine", manual=False)

    def _current_window_context(self) -> dict[str, str]:
        if os.name != "nt":
            return {}
        try:
            import ctypes
            import psutil

            hwnd = ctypes.windll.user32.GetForegroundWindow()
            length = ctypes.windll.user32.GetWindowTextLengthW(hwnd)
            buffer = ctypes.create_unicode_buffer(length + 1)
            ctypes.windll.user32.GetWindowTextW(hwnd, buffer, length + 1)
            pid = ctypes.c_ulong()
            ctypes.windll.user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))
            process_name = ""
            app_name = ""
            try:
                proc = psutil.Process(pid.value)
                process_name = proc.name()
                app_name = proc.exe()
            except Exception:
                pass
            return {
                "hwnd": str(int(hwnd)),
                "window_title": buffer.value,
                "process_name": process_name,
                "app_name": app_name or process_name,
            }
        except Exception:
            return {}

    def _is_context_excluded(self, context: dict[str, str]) -> str:
        cfg = work_memory_config()
        if cfg.get("privacy_mode", False):
            return "隐私模式已开启"
        process_name = str(context.get("process_name", "")).lower()
        app_name = str(context.get("app_name", "")).lower()
        title = str(context.get("window_title", "")).lower()
        excluded_apps = [str(item).lower().strip() for item in cfg.get("exclude_apps", [])]
        for item in excluded_apps:
            if item and (item == process_name or item in app_name):
                return f"命中排除应用: {item}"
        for keyword in cfg.get("exclude_window_keywords", []):
            text = str(keyword).lower().strip()
            if text and text in title:
                return f"命中排除窗口: {keyword}"
        for path in cfg.get("exclude_paths", []):
            text = str(path).lower().strip()
            if text and text in app_name:
                return f"命中排除路径: {path}"
        return ""

    def _grab_combined_screenshot(self) -> QPixmap | None:
        app = QApplication.instance()
        if app is None:
            return None
        screens = app.screens()
        if not screens:
            return None
        rect = QRect()
        for screen in screens:
            rect = rect.united(screen.geometry()) if not rect.isNull() else screen.geometry()
        if rect.isNull() or rect.width() <= 0 or rect.height() <= 0:
            return None
        combined = QPixmap(rect.size())
        combined.fill(QColor("transparent"))
        painter = QPainter(combined)
        for screen in screens:
            shot = screen.grabWindow(0)
            if not shot.isNull():
                top_left = screen.geometry().topLeft() - rect.topLeft()
                painter.drawPixmap(top_left, shot)
        painter.end()
        return combined

    def _grab_configured_screenshot(self, context: dict[str, str]) -> QPixmap | None:
        cfg = work_memory_config()
        app = QApplication.instance()
        if app is None:
            return None
        if str(cfg.get("capture_scope", "all_screens")) == "foreground_window":
            try:
                hwnd = int(context.get("hwnd", "0") or 0)
            except Exception:
                hwnd = 0
            if hwnd:
                screen = app.primaryScreen()
                if screen is not None:
                    shot = screen.grabWindow(hwnd)
                    if not shot.isNull():
                        return shot
        return self._grab_combined_screenshot()

    def capture_current_screen(self, *, source="time_machine", manual=False) -> dict[str, Any] | None:
        cfg = work_memory_config()
        if not cfg.get("enabled", True):
            self._last_pause_reason = "工作记忆总开关已关闭"
            self.status_changed.emit()
            return None

        context = self._current_window_context()
        excluded = self._is_context_excluded(context)
        if excluded:
            self._last_pause_reason = excluded
            self.status_changed.emit()
            return None

        pixmap = self._grab_configured_screenshot(context)
        if pixmap is None or pixmap.isNull():
            self._last_pause_reason = "自动截图不可用"
            self.status_changed.emit()
            return None

        self._ensure_dirs()
        stamp = datetime.now().strftime("%Y%m%d-%H%M%S")
        image_path = os.path.join(IMAGE_DIR, f"screen-{stamp}-{uuid.uuid4().hex[:8]}.png")
        if not pixmap.save(image_path, "PNG"):
            self._last_pause_reason = "截图保存失败"
            self.status_changed.emit()
            return None

        signature = self._file_sha1(image_path)
        if signature and signature == self._last_screen_signature:
            os.remove(image_path)
            self._merge_duplicate_screen()
            self._last_pause_reason = "重复画面已合并"
            self.status_changed.emit()
            return None
        self._last_screen_signature = signature

        thumb = self._thumbnail_for_image(image_path)
        metadata = {
            "size": f"{pixmap.width()}x{pixmap.height()}",
            "scope": cfg.get("capture_scope", "all_screens"),
            "manual": bool(manual),
            "signature": signature,
            "duplicate_count": 1,
            "ocr_status": "pending" if cfg.get("auto_ocr", False) else "disabled",
            "vector_status": "disabled"
            if not cfg.get("embedding_enabled", False)
            else "pending",
        }
        entry = self.add_entry(
            {
                "source": source,
                "source_ref": f"{source}:{signature}:{stamp}",
                "content_type": "screenshot",
                "title": ("手动补记屏幕" if manual else "屏幕时间机器") + " · " + _format_time(_now()),
                "summary": f"范围: {metadata['scope']}，尺寸: {metadata['size']}",
                "image_path": image_path,
                "thumbnail_path": thumb,
                "app_name": context.get("app_name", ""),
                "process_name": context.get("process_name", ""),
                "window_title": context.get("window_title", ""),
                "tags": ["time-machine", "screenshot"] if source == "time_machine" else ["screenshot"],
                "metadata": metadata,
            },
            force=True,
        )
        self._last_capture_at = _now()
        self._last_pause_reason = ""

        if entry and cfg.get("auto_ocr", False) and not entry.get("sensitive"):
            self.perform_ocr(entry["id"])

        self.status_changed.emit()
        return entry

    def _merge_duplicate_screen(self) -> None:
        for entry in self._entries:
            if entry.get("source") == "time_machine":
                metadata = entry.setdefault("metadata", {})
                metadata["duplicate_count"] = int(metadata.get("duplicate_count", 1) or 1) + 1
                entry["updated_at"] = _now()
                self._save()
                self.entries_changed.emit()
                return

    @staticmethod
    def _file_sha1(path: str) -> str:
        try:
            digest = hashlib.sha1()
            with open(path, "rb") as f:
                for chunk in iter(lambda: f.read(1024 * 1024), b""):
                    digest.update(chunk)
            return digest.hexdigest()
        except Exception:
            return ""

    def _thumbnail_for_image(self, path: str) -> str:
        if not path or not os.path.exists(path):
            return ""
        try:
            os.makedirs(THUMB_DIR, exist_ok=True)
            digest = self._file_sha1(path) or uuid.uuid4().hex
            target = os.path.join(THUMB_DIR, f"{digest}.png")
            if os.path.exists(target):
                return target
            pixmap = QPixmap(path)
            if pixmap.isNull():
                return ""
            thumb = pixmap.scaledToWidth(360)
            if thumb.height() > 240:
                thumb = pixmap.scaled(
                    360,
                    240,
                    Qt.AspectRatioMode.KeepAspectRatio,
                    Qt.TransformationMode.SmoothTransformation,
                )
            if thumb.save(target, "PNG"):
                return target
        except Exception as exc:
            logger.warning("Failed to build work memory thumbnail: %s", exc)
        return ""

    def _load_ocr_engine(self):
        if self._ocr_engine is not None:
            return self._ocr_engine
        if self._ocr_error:
            return None
        try:
            from rapidocr_onnxruntime import RapidOCR

            self._ocr_engine = RapidOCR()
            return self._ocr_engine
        except Exception as exc:
            self._ocr_error = str(exc)
            logger.warning("RapidOCR unavailable: %s", exc)
            return None

    def perform_ocr(self, entry_id: str) -> tuple[bool, str]:
        entry = self.get_entry(entry_id)
        if not entry:
            return False, "条目不存在"
        image_path = str(entry.get("image_path", "")).strip()
        if not image_path or not os.path.exists(image_path):
            return False, "图片文件不存在"
        if entry.get("sensitive"):
            return False, "敏感条目默认不执行 OCR"
        engine = self._load_ocr_engine()
        if engine is None:
            message = self._ocr_error or "OCR 组件不可用"
            self._mutate_entry(
                entry_id,
                lambda item: item.setdefault("metadata", {}).__setitem__(
                    "ocr_status", f"unavailable: {message}"
                ),
            )
            return False, message
        try:
            result, _elapsed = engine(image_path)
            texts = []
            if isinstance(result, list):
                for row in result:
                    if isinstance(row, (list, tuple)) and len(row) >= 2:
                        texts.append(str(row[1]))
                    elif isinstance(row, dict):
                        texts.append(str(row.get("text", "")))
            ocr_text = "\n".join(text for text in texts if text.strip()).strip()
            sensitive, flags = detect_sensitive(ocr_text)

            def mutate(item):
                item["ocr_text"] = ocr_text
                item["content_type"] = "ocr_text" if ocr_text else item.get("content_type", "screenshot")
                item["summary"] = self._preview_text(ocr_text, 240) or item.get("summary", "")
                item.setdefault("metadata", {})["ocr_status"] = "done" if ocr_text else "empty"
                if sensitive:
                    item["sensitive"] = True
                    item["ai_allowed"] = False
                    item["vector_allowed"] = False
                    item["risk_flags"] = sorted(set(item.get("risk_flags", []) + flags))

            self._mutate_entry(entry_id, mutate)
            return True, ocr_text or "未识别到文字"
        except Exception as exc:
            self._mutate_entry(
                entry_id,
                lambda item: item.setdefault("metadata", {}).__setitem__(
                    "ocr_status", f"failed: {exc}"
                ),
            )
            return False, str(exc)

    def _entries_for_draft(self, entry_ids: list[str] | None, start_ts=None, end_ts=None) -> list[dict[str, Any]]:
        if entry_ids:
            keys = set(str(item).strip() for item in entry_ids if str(item).strip())
            items = [entry for entry in self._entries if entry.get("id") in keys]
        else:
            start_value = _safe_float(start_ts, _now() - 86400)
            end_value = _safe_float(end_ts, _now())
            items = [
                entry
                for entry in self._entries
                if start_value <= float(entry.get("created_at", 0) or 0) <= end_value
            ]
        items = [entry for entry in items if not entry.get("hidden")]
        items.sort(key=lambda item: float(item.get("created_at", 0) or 0))
        return [copy.deepcopy(entry) for entry in items]

    @staticmethod
    def _evidence_line(entry: dict[str, Any], index: int) -> str:
        time_text = _format_time(entry.get("created_at", 0))
        label = f"E{index}"
        title = entry.get("title", "工作记忆")
        source = _entry_source_label(entry)
        return f"- [{label}] {time_text} · {source} · {title} · id={entry.get('id')}"

    def _draft_header(self, title: str, entries: list[dict[str, Any]]) -> str:
        return "\n".join(
            [
                f"# {title}",
                "",
                f"生成时间: {_format_time(_now())}",
                f"引用条目: {len(entries)} 条",
                "说明: 这是本地生成的可编辑草稿，未同步到外部系统。",
                "",
            ]
        )

    def generate_daily_report(
        self, entry_ids: list[str] | None = None, *, start_ts=None, end_ts=None
    ) -> dict[str, Any] | None:
        entries = self._entries_for_draft(entry_ids, start_ts, end_ts)
        title = "工作记忆日报草稿"
        evidence = [self._evidence_line(entry, idx) for idx, entry in enumerate(entries, 1)]
        important_commands = [
            entry for entry in entries if entry.get("content_type") in {"command", "sql", "json", "error_log"}
        ]
        unfinished = [
            entry
            for entry in entries
            if entry.get("content_type") == "todo" or "待办" in self._context_text(entry)
        ]
        risks = [entry for entry in entries if entry.get("sensitive") or entry.get("risk_flags")]
        body = [
            self._draft_header(title, entries),
            "## 今日处理的问题",
            *(f"- {entry.get('summary') or entry.get('title')}" for entry in entries[:12]),
            "",
            "## 关键材料",
            *(f"- {entry.get('title')} ({_entry_source_label(entry)})" for entry in entries[:16]),
            "",
            "## 重要命令、链接、日志、SQL、JSON",
            *(
                f"- {entry.get('title')}: {self._preview_text(entry.get('text') or entry.get('summary'), 160)}"
                for entry in important_commands[:12]
            ),
            "",
            "## 未完成事项",
            *(f"- {entry.get('summary') or entry.get('title')}" for entry in unfinished[:10]),
            "" if unfinished else "- 暂未从本地记忆中识别到明确待办",
            "",
            "## 风险提醒",
            *(
                f"- {entry.get('title')}: {', '.join(entry.get('risk_flags', [])) or '敏感标记'}"
                for entry in risks[:10]
            ),
            "" if risks else "- 未发现已标记敏感或风险条目",
            "",
            "## 可复用动作建议",
            *self._workflow_suggestion_lines(entries),
            "",
            "## 证据",
            *evidence,
            "",
        ]
        text = "\n".join(body)
        path = self._save_draft_file("daily-report", text)
        entry = self.add_entry(
            {
                "source": "daily_report",
                "source_ref": f"daily:{uuid.uuid4()}",
                "content_type": "daily_report",
                "title": title,
                "summary": f"基于 {len(entries)} 条工作记忆生成",
                "text": text,
                "file_path": path,
                "relations": [entry.get("id", "") for entry in entries],
                "tags": ["daily", "draft"],
            },
            force=True,
        )
        self.mark_included_in_daily([entry.get("id", "") for entry in entries])
        return entry

    def generate_knowledge_draft(self, entry_ids: list[str]) -> dict[str, Any] | None:
        entries = self._entries_for_draft(entry_ids)
        if not entries:
            return None
        title = "知识条目草稿"
        first = entries[0]
        evidence = [self._evidence_line(entry, idx) for idx, entry in enumerate(entries, 1)]
        body = [
            self._draft_header(title, entries),
            "## 标题",
            first.get("title", "待命名知识条目"),
            "",
            "## 问题描述",
            first.get("summary") or first.get("text") or "待补充",
            "",
            "## 适用场景",
            "- 来自 x-tools 本地工作记忆中相同问题、窗口、截图、剪贴板或文件上下文。",
            "",
            "## 处理步骤",
            "- 根据证据补全排查与处理步骤。",
            "",
            "## 注意事项",
            "- 同步到 OpsCore 或其他知识库前必须再次检查敏感内容。",
            "",
            "## 相关命令",
            *(
                f"- {self._preview_text(entry.get('text'), 180)}"
                for entry in entries
                if entry.get("content_type") in {"command", "sql", "json"}
            ),
            "",
            "## 标签",
            ", ".join(sorted(set(tag for entry in entries for tag in entry.get("tags", [])))) or "待补充",
            "",
            "## 敏感内容提示",
            "- " + ("存在敏感标记，默认不外发。" if any(e.get("sensitive") for e in entries) else "未发现敏感标记。"),
            "",
            "## 证据",
            *evidence,
            "",
        ]
        text = "\n".join(body)
        path = self._save_draft_file("knowledge-draft", text)
        entry = self.add_entry(
            {
                "source": "knowledge_draft",
                "source_ref": f"knowledge:{uuid.uuid4()}",
                "content_type": "knowledge_draft",
                "title": title + " · " + self._preview_text(first.get("title", ""), 50),
                "summary": f"基于 {len(entries)} 条证据生成",
                "text": text,
                "file_path": path,
                "relations": [entry.get("id", "") for entry in entries],
                "tags": ["knowledge", "draft"],
            },
            force=True,
        )
        self.mark_knowledge_generated([entry.get("id", "") for entry in entries])
        return entry

    def generate_retro_draft(self, entry_ids: list[str]) -> dict[str, Any] | None:
        entries = self._entries_for_draft(entry_ids)
        if not entries:
            return None
        evidence = [self._evidence_line(entry, idx) for idx, entry in enumerate(entries, 1)]
        title = "问题复盘草稿"
        body = [
            self._draft_header(title, entries),
            "## 问题背景",
            entries[0].get("summary") or entries[0].get("title") or "待补充",
            "",
            "## 现象",
            "- 根据截图、剪贴板、日志或笔记补充现象。",
            "",
            "## 关键证据",
            *evidence,
            "",
            "## 排查过程",
            "- 待用户编辑补充。",
            "",
            "## 临时处理",
            "- 待用户编辑补充。",
            "",
            "## 最终结论",
            "- 待用户编辑补充。",
            "",
            "## 后续事项",
            "- 评估是否沉淀为知识条目、检查清单、工作流或 skill。",
            "",
        ]
        text = "\n".join(body)
        path = self._save_draft_file("retro-draft", text)
        return self.add_entry(
            {
                "source": "retro_draft",
                "source_ref": f"retro:{uuid.uuid4()}",
                "content_type": "retro_draft",
                "title": title,
                "summary": f"基于 {len(entries)} 条证据生成",
                "text": text,
                "file_path": path,
                "relations": [entry.get("id", "") for entry in entries],
                "tags": ["retro", "draft"],
            },
            force=True,
        )

    def _workflow_suggestion_lines(self, entries: list[dict[str, Any]]) -> list[str]:
        commands = Counter(
            str(entry.get("trigger_command") or entry.get("text") or "").strip()
            for entry in entries
            if entry.get("content_type") == "command"
        )
        lines = []
        for command, count in commands.most_common(5):
            if command and count >= 2:
                lines.append(f"- `{command}` 出现 {count} 次，可考虑沉淀为工作流草稿。")
        source_counter = Counter(entry.get("source") for entry in entries)
        for source, count in source_counter.most_common(3):
            if source and count >= 5:
                lines.append(f"- {_entry_source_label({'source': source})} 条目较多，可定期整理。")
        return lines or ["- 暂未发现足够重复的低风险动作。"]

    def generate_experience_report(self, days=7) -> dict[str, Any] | None:
        cutoff = _now() - max(1, int(days or 7)) * 86400
        entries = [entry for entry in self._entries if float(entry.get("created_at", 0) or 0) >= cutoff]
        entries.sort(key=lambda item: float(item.get("created_at", 0) or 0))
        source_counter = Counter(entry.get("source") for entry in entries)
        type_counter = Counter(entry.get("content_type") for entry in entries)
        tag_counter = Counter(tag for entry in entries for tag in entry.get("tags", []))
        evidence = [self._evidence_line(entry, idx) for idx, entry in enumerate(entries[:30], 1)]
        workflow_lines = self._workflow_suggestion_lines(entries)
        title = "经验发现报告"
        body = [
            self._draft_header(title, entries),
            "## 重复出现的问题",
            *(
                f"- {tag}: {count} 次"
                for tag, count in tag_counter.most_common(10)
                if tag in {"error_log", "issue", "risk:internal_address", "risk:token", "technical"}
            ),
            "",
            "## 重复执行的流程",
            *workflow_lines,
            "",
            "## 高价值但未沉淀的知识",
            *[
                f"- {entry.get('title')}"
                for entry in entries
                if not entry.get("knowledge_generated") and entry.get("favorite")
            ][:10],
            "",
            "## 经常被复制或搜索的材料",
            *(
                f"- {_entry_source_label({'source': source})}: {count} 条"
                for source, count in source_counter.most_common()
            ),
            "",
            "## 可能需要自动化的任务",
            *workflow_lines,
            "",
            "## 适合形成 skill 或外部代理任务",
            "- 对反复出现的检查、转换、整理和项目内修改需求，先生成任务包并由用户确认后交给外部代理。",
            "",
            "## 本地统计",
            f"- 来源分布: {dict(source_counter)}",
            f"- 类型分布: {dict(type_counter)}",
            "",
            "## 证据",
            *evidence,
            "",
        ]
        text = "\n".join(body)
        path = self._save_draft_file("experience-report", text)
        return self.add_entry(
            {
                "source": "experience_report",
                "source_ref": f"experience:{uuid.uuid4()}",
                "content_type": "experience_report",
                "title": title,
                "summary": f"最近 {days} 天，{len(entries)} 条工作记忆",
                "text": text,
                "file_path": path,
                "relations": [entry.get("id", "") for entry in entries[:30]],
                "tags": ["experience", "learn", "draft"],
            },
            force=True,
        )

    def generate_asset_suggestion(self, entry_ids: list[str], asset_type="skill") -> dict[str, Any] | None:
        entries = self._entries_for_draft(entry_ids)
        if not entries:
            return None
        labels = {
            "skill": ("Skill 建议草稿", "skill_suggestion", "skill"),
            "workflow": ("工作流建议草稿", "workflow_suggestion", "workflow"),
            "checklist": ("检查清单草稿", "skill_suggestion", "checklist"),
            "prompt": ("提示词模板草稿", "skill_suggestion", "prompt"),
        }
        title, source, tag = labels.get(asset_type, labels["skill"])
        evidence = [self._evidence_line(entry, idx) for idx, entry in enumerate(entries, 1)]
        text = "\n".join(
            [
                self._draft_header(title, entries),
                "## 发现原因",
                "- 这些工作记忆在时间、来源、关键词或操作意图上存在可复用模式。",
                "",
                "## 推荐承载形式",
                f"- {tag}",
                "",
                "## 输入材料",
                *evidence,
                "",
                "## 预期输出",
                "- 可复用的步骤、提示词、检查清单、x-tools 工作流或 Codex skill 草稿。",
                "",
                "## 权限与风险",
                "- 默认只生成草稿，不自动创建、修改或启用能力资产。",
                "- 涉及写文件、运行命令、同步或外发时必须由用户确认。",
                "",
                "## 验收口径",
                "- 用户能看懂适用场景、输入、输出、风险边界和证据来源。",
                "",
            ]
        )
        path = self._save_draft_file(tag + "-suggestion", text)
        return self.add_entry(
            {
                "source": source,
                "source_ref": f"{source}:{uuid.uuid4()}",
                "content_type": source,
                "title": title,
                "summary": f"基于 {len(entries)} 条证据生成",
                "text": text,
                "file_path": path,
                "relations": [entry.get("id", "") for entry in entries],
                "tags": [tag, "draft"],
            },
            force=True,
        )

    def generate_external_agent_task(self, entry_ids: list[str], goal: str = "") -> dict[str, Any] | None:
        entries = self._entries_for_draft(entry_ids)
        if not entries:
            return None
        cfg = work_memory_config()
        task_dir = str(cfg.get("external_agent_task_dir") or TASK_DIR)
        os.makedirs(task_dir, exist_ok=True)
        evidence = [self._evidence_line(entry, idx) for idx, entry in enumerate(entries, 1)]
        title = "外部代理任务包"
        text = "\n".join(
            [
                self._draft_header(title, entries),
                "## 目标",
                goal.strip() or "请基于以下工作记忆完成用户确认后的开发、验证或文档任务。",
                "",
                "## 背景",
                "- 本任务包由 x-tools 本地工作记忆生成。",
                "- 其中的证据只来自用户本机已记录、复制、截图、笔记或收藏的材料。",
                "",
                "## 相关工作记忆",
                *evidence,
                "",
                "## 用户偏好",
                "- 不盲从初始想法，应适当提出更好的技术路径。",
                "- 涉及浏览器调试时优先使用内置 browser 技能。",
                "",
                "## 输入材料",
                *(
                    f"- {entry.get('title')}: {self._preview_text(entry.get('summary') or entry.get('text') or entry.get('ocr_text'), 180)}"
                    for entry in entries
                ),
                "",
                "## 期望产物",
                "- 可审阅的实现、测试、文档或 skill/工作流草稿。",
                "",
                "## 风险边界",
                "- 不允许绕过用户授权访问生产系统、堡垒机、VPN、SSO 或远程桌面。",
                "- 不允许无确认执行外发、删除、同步、写 Hosts 或高风险命令。",
                "- 敏感条目默认不能发给外部服务。",
                "",
                "## 权限",
                "- 是否允许修改文件: 需要用户在外部代理中确认。",
                "- 是否允许运行命令: 需要用户在外部代理中确认。",
                "- 是否允许访问网络: 需要用户在外部代理中确认。",
                "",
                "## 验收标准",
                "- 能说明修改内容、证据来源、测试结果和残余风险。",
                "- 用户可在本地验证产物，不依赖未授权外部系统。",
                "",
            ]
        )
        path = os.path.join(task_dir, f"agent-task-{datetime.now().strftime('%Y%m%d-%H%M%S')}.md")
        _write_text_file(path, text)
        return self.add_entry(
            {
                "source": "task_package",
                "source_ref": f"task:{uuid.uuid4()}",
                "content_type": "task_package",
                "title": title,
                "summary": f"已生成任务包: {path}",
                "text": text,
                "file_path": path,
                "relations": [entry.get("id", "") for entry in entries],
                "tags": ["agent", "codex", "task-package"],
            },
            force=True,
        )

    @staticmethod
    def _save_draft_file(prefix: str, text: str) -> str:
        os.makedirs(DRAFT_DIR, exist_ok=True)
        path = os.path.join(DRAFT_DIR, f"{prefix}-{datetime.now().strftime('%Y%m%d-%H%M%S')}.md")
        _write_text_file(path, text)
        return path

    def export_package(
        self,
        *,
        entry_ids: list[str] | None = None,
        start_ts=None,
        end_ts=None,
        tag: str = "",
    ) -> str:
        cfg = work_memory_config()
        if entry_ids:
            keys = set(str(item).strip() for item in entry_ids if str(item).strip())
            entries = [entry for entry in self._entries if entry.get("id") in keys]
        else:
            entries = self._entries_for_draft(None, start_ts or 0, end_ts or _now())
        if tag:
            entries = [entry for entry in entries if tag in entry.get("tags", [])]
        if not cfg.get("allow_sensitive_export", False):
            entries = [entry for entry in entries if not entry.get("sensitive")]
        os.makedirs(EXPORT_DIR, exist_ok=True)
        path = os.path.join(EXPORT_DIR, f"work-memory-export-{datetime.now().strftime('%Y%m%d-%H%M%S')}.zip")
        readme = "\n".join(
            [
                "# x-tools 工作记忆导出包",
                "",
                f"导出时间: {_format_time(_now())}",
                f"条目数量: {len(entries)}",
                "说明: 敏感条目默认不导出，除非用户在配置中显式允许。",
                "",
            ]
        )
        with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as zf:
            zf.writestr("README.md", readme)
            zf.writestr("entries.json", json.dumps(entries, ensure_ascii=False, indent=2))
            summary_lines = [readme, "## 条目摘要"]
            for index, entry in enumerate(entries, 1):
                summary_lines.append(self._evidence_line(entry, index))
            zf.writestr("summary.md", "\n".join(summary_lines))
            for entry in entries:
                for key in ["image_path", "thumbnail_path", "file_path"]:
                    item_path = str(entry.get(key, "")).strip()
                    if not item_path or not os.path.exists(item_path):
                        continue
                    try:
                        arcname = os.path.join("files", entry.get("id", ""), os.path.basename(item_path))
                        zf.write(item_path, arcname=arcname)
                    except Exception:
                        pass
        return path

    def storage_status(self) -> dict[str, Any]:
        total = 0
        for root, _dirs, files in os.walk(WORK_MEMORY_DIR):
            for filename in files:
                try:
                    total += os.path.getsize(os.path.join(root, filename))
                except Exception:
                    pass
        pending_ocr = 0
        pending_vector = 0
        for entry in self._entries:
            metadata = entry.get("metadata", {}) if isinstance(entry.get("metadata"), dict) else {}
            if metadata.get("ocr_status") == "pending":
                pending_ocr += 1
            if metadata.get("vector_status") == "pending":
                pending_vector += 1
        return {
            "entry_count": len(self._entries),
            "bytes": total,
            "mb": total / 1024 / 1024,
            "pending_ocr": pending_ocr,
            "pending_vector": pending_vector,
        }

    def status(self) -> dict[str, Any]:
        cfg = work_memory_config()
        storage = self.storage_status()
        return {
            "enabled": bool(cfg.get("enabled", True)),
            "time_machine_enabled": bool(cfg.get("time_machine_enabled", False)),
            "running": bool(self._is_running),
            "interval_seconds": max(10, _safe_int(cfg.get("auto_capture_interval_seconds"), 300)),
            "scope": str(cfg.get("capture_scope", "all_screens")),
            "last_capture_at": self._last_capture_at,
            "last_capture_text": _format_time(self._last_capture_at) if self._last_capture_at else "尚未截图",
            "pause_reason": self._last_pause_reason or "无",
            "privacy_mode": bool(cfg.get("privacy_mode", False)),
            "ai_status": "启用" if cfg.get("ai_enabled", False) else "未启用，本地草稿降级",
            "embedding_status": "启用" if cfg.get("embedding_enabled", False) else "未启用，关键词/近似搜索降级",
            "milvus_status": "启用" if cfg.get("vector_store_type") == "milvus" else "未启用，本地索引降级",
            "storage": storage,
        }


work_memory_manager = WorkMemoryManager()
