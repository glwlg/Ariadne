import json
import os
import time
import uuid
from datetime import datetime

from PyQt6.QtCore import QObject, QTimer, pyqtSignal
from PyQt6.QtGui import QImage, QPixmap
from PyQt6.QtWidgets import QApplication

from src.core.logger import get_logger


logger = get_logger(__name__)

APPDATA_DIR = os.getenv("APPDATA") or os.path.expanduser("~")
XTOOLS_DIR = os.path.join(APPDATA_DIR, "x-tools")
HISTORY_FILE = os.path.join(XTOOLS_DIR, "capture_history.json")
IMAGE_DIR = os.path.join(XTOOLS_DIR, "capture_images")


class CaptureHistoryManager(QObject):
    entries_changed = pyqtSignal()

    def __init__(self):
        super().__init__()
        self._entries = []
        self._is_applying = False
        self._max_entries = 300
        self._load()

    def _load(self):
        os.makedirs(XTOOLS_DIR, exist_ok=True)
        os.makedirs(IMAGE_DIR, exist_ok=True)

        if not os.path.exists(HISTORY_FILE):
            self._entries = []
            return

        try:
            with open(HISTORY_FILE, "r", encoding="utf-8") as f:
                raw = json.load(f)

            entries = []
            if isinstance(raw, list):
                for item in raw:
                    if not isinstance(item, dict):
                        continue

                    image_path = str(item.get("image_path", "")).strip()
                    if not image_path or not os.path.exists(image_path):
                        continue

                    entry = {
                        "id": str(item.get("id", "")).strip() or str(uuid.uuid4()),
                        "image_path": image_path,
                        "saved_path": str(item.get("saved_path", "")).strip(),
                        "created_at": float(item.get("created_at", time.time())),
                        "source": str(item.get("source", "")).strip(),
                        "actions": [
                            str(action)
                            for action in item.get("actions", [])
                            if str(action).strip()
                        ],
                        "pinned": bool(item.get("pinned", False)),
                        "width": int(item.get("width", 0) or 0),
                        "height": int(item.get("height", 0) or 0),
                    }
                    entries.append(entry)

            self._entries = entries[: self._max_entries]
        except Exception as e:
            logger.warning("Failed to load capture history: %s", e)
            self._entries = []

    def _save(self):
        try:
            os.makedirs(XTOOLS_DIR, exist_ok=True)
            os.makedirs(IMAGE_DIR, exist_ok=True)
            with open(HISTORY_FILE, "w", encoding="utf-8") as f:
                json.dump(self._entries, f, ensure_ascii=False, indent=2)
        except Exception as e:
            logger.warning("Failed to save capture history: %s", e)

    @staticmethod
    def _pixmap_from_image_like(image_like):
        if isinstance(image_like, QPixmap) and not image_like.isNull():
            return image_like
        if isinstance(image_like, QImage) and not image_like.isNull():
            return QPixmap.fromImage(image_like)
        return None

    def add_capture(self, image_like, source="", saved_path="", actions=None):
        pixmap = self._pixmap_from_image_like(image_like)
        if pixmap is None:
            return None

        os.makedirs(IMAGE_DIR, exist_ok=True)
        capture_id = str(uuid.uuid4())
        image_path = os.path.join(IMAGE_DIR, f"{capture_id}.png")
        if not pixmap.save(image_path, "PNG"):
            return None

        action_list = []
        if isinstance(actions, (list, tuple, set)):
            action_list = [str(action) for action in actions if str(action).strip()]

        entry = {
            "id": capture_id,
            "image_path": image_path,
            "saved_path": str(saved_path or "").strip(),
            "created_at": time.time(),
            "source": str(source or "").strip(),
            "actions": action_list,
            "pinned": False,
            "width": pixmap.width(),
            "height": pixmap.height(),
        }

        self._entries.insert(0, entry)
        self._trim()
        self._save()
        self.entries_changed.emit()
        return dict(entry)

    def _trim(self):
        while len(self._entries) > self._max_entries:
            removed = self._entries.pop()
            self._safe_remove_history_image(removed.get("image_path", ""))

    @staticmethod
    def _safe_remove_history_image(path):
        if not path:
            return

        try:
            base = os.path.abspath(IMAGE_DIR)
            target = os.path.abspath(path)
            if target.startswith(base + os.sep) and os.path.exists(target):
                os.remove(target)
        except Exception:
            pass

    @staticmethod
    def _format_time(ts):
        try:
            return datetime.fromtimestamp(float(ts)).strftime("%Y-%m-%d %H:%M:%S")
        except Exception:
            return ""

    def get_entries(self, query="", limit=300):
        text = str(query or "").strip().lower()
        items = self._entries
        if text:
            filtered = []
            for entry in items:
                haystack = " ".join(
                    [
                        f"{entry.get('width', 0)}x{entry.get('height', 0)}",
                        str(entry.get("source", "")),
                        str(entry.get("saved_path", "")),
                        self._format_time(entry.get("created_at", 0)),
                        " ".join(str(action) for action in entry.get("actions", [])),
                        "截图 捕获 capture screenshot shot image 图片",
                    ]
                ).lower()
                if text in haystack:
                    filtered.append(entry)
            items = filtered

        sorted_items = sorted(
            items,
            key=lambda e: (
                0 if e.get("pinned", False) else 1,
                -float(e.get("created_at", 0)),
            ),
        )
        return [dict(item) for item in sorted_items[: max(1, int(limit or 1))]]

    def get_entry(self, entry_id):
        key = str(entry_id).strip()
        if not key:
            return None
        for entry in self._entries:
            if entry.get("id") == key:
                return dict(entry)
        return None

    def toggle_pin(self, entry_id):
        key = str(entry_id).strip()
        if not key:
            return False
        for entry in self._entries:
            if entry.get("id") == key:
                entry["pinned"] = not bool(entry.get("pinned", False))
                self._save()
                self.entries_changed.emit()
                return bool(entry["pinned"])
        return False

    def delete_entry(self, entry_id):
        key = str(entry_id).strip()
        if not key:
            return False

        for idx, entry in enumerate(self._entries):
            if entry.get("id") == key:
                removed = self._entries.pop(idx)
                self._safe_remove_history_image(removed.get("image_path", ""))
                self._save()
                self.entries_changed.emit()
                return True
        return False

    def clear_unpinned(self):
        remaining = []
        removed_count = 0
        for entry in self._entries:
            if entry.get("pinned", False):
                remaining.append(entry)
            else:
                removed_count += 1
                self._safe_remove_history_image(entry.get("image_path", ""))

        if removed_count > 0:
            self._entries = remaining
            self._save()
            self.entries_changed.emit()
        return removed_count

    def copy_entry_to_clipboard(self, entry_id):
        entry = self.get_entry(entry_id)
        if not entry:
            return False

        clipboard = QApplication.clipboard()
        if clipboard is None:
            return False

        image = QImage(str(entry.get("image_path", "")))
        if image.isNull():
            return False

        self._is_applying = True
        try:
            clipboard.setImage(image)
        finally:
            QTimer.singleShot(120, self._clear_apply_flag)
        return True

    def _clear_apply_flag(self):
        self._is_applying = False

    @staticmethod
    def _entry_display_name(entry):
        prefix = "★ " if entry.get("pinned", False) else ""
        size = f"{entry.get('width', 0)}x{entry.get('height', 0)}"
        time_text = CaptureHistoryManager._format_time(entry.get("created_at", 0))
        return f"{prefix}截图 {size}  {time_text}".strip()

    def as_search_results(self, query="", limit=25):
        results = []
        for entry in self.get_entries(query=query, limit=limit):
            results.append(
                {
                    "type": "capture_entry",
                    "name": self._entry_display_name(entry),
                    "path": str(entry.get("id", "")),
                    "capture_id": str(entry.get("id", "")),
                    "capture_image_path": str(entry.get("image_path", "")),
                    "capture_saved_path": str(entry.get("saved_path", "")),
                    "capture_pinned": bool(entry.get("pinned", False)),
                    "capture_size": f"{entry.get('width', 0)}x{entry.get('height', 0)}",
                    "capture_created_at": float(entry.get("created_at", 0) or 0),
                    "capture_actions": list(entry.get("actions", [])),
                    "capture_source": str(entry.get("source", "")),
                }
            )
        return results


capture_history_manager = CaptureHistoryManager()
