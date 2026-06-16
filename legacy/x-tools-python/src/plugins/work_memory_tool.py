from __future__ import annotations

import os

from PyQt6.QtWidgets import QApplication

from src.core.plugin_base import PluginBase
from src.core.work_memory import SOURCE_LABELS, work_memory_manager
from src.platform.shell import open_parent, open_path
from src.ui.work_memory_window import WorkMemoryWindow


class WorkMemoryPlugin(PluginBase):
    required_capabilities = ("clipboard", "open_path", "screenshot")

    def __init__(self):
        self.window = None

    def get_name(self):
        return "工作记忆"

    def get_description(self):
        return "搜索工作记忆、查看屏幕时间机器、生成日报/知识草稿和外部代理任务包"

    def get_keywords(self):
        return ["mem", "memory", "timeline", "day", "note", "recall", "trace", "learn"]

    def get_command_schema(self):
        return {
            "usage": "mem [query] / note <content> / day / learn",
            "examples": [
                "mem 报错",
                "timeline",
                "note 这个报错和网关配置有关",
                "recall 上午看到的 token 报错",
                "day",
                "learn",
            ],
            "params": [
                {
                    "name": "query",
                    "label": "查询或动作",
                    "placeholder": "留空打开中心；输入 note/day/learn/capture/pause/resume",
                    "required": False,
                }
            ],
        }

    def is_direct_action(self):
        return True

    def _ensure_window(self):
        if self.window is None:
            self.window = WorkMemoryWindow(manager=work_memory_manager)

    @staticmethod
    def _strip_keyword(query):
        text = str(query or "").strip()
        lowered = text.lower()
        for keyword in ["mem", "memory", "timeline", "day", "note", "recall", "trace", "learn"]:
            if lowered == keyword:
                return keyword, ""
            prefix = keyword + " "
            if lowered.startswith(prefix):
                return keyword, text[len(prefix) :].strip()
        return "", text

    def execute(self, query):
        keyword, text = self._strip_keyword(query)
        lowered = text.lower()

        work_memory_manager.sync_clipboard_history()
        work_memory_manager.sync_capture_history()

        results = []

        def cmd(name, action, hint=""):
            results.append(
                {
                    "name": name,
                    "path": action,
                    "type": "work_memory_cmd",
                    "work_memory_hint": hint,
                }
            )

        cmd("打开工作记忆中心", "open_center", "时间线、搜索、筛选、草稿和时间机器控制")
        cmd("手动补记当前屏幕", "capture_now", "把当前屏幕写入屏幕时间机器时间线")
        cmd("生成可见范围日报草稿", "daily_visible", "基于最近工作记忆生成本地可编辑日报")
        cmd("生成经验发现报告", "experience", "发现重复问题、流程经验和自动化机会")

        if keyword == "timeline" or lowered in {"timeline", "time", "时间线"}:
            cmd("打开屏幕时间机器", "open_center", "查看自动截图和状态")
            return self._with_plugin(results)

        if keyword == "day" or lowered in {"day", "daily", "日报"}:
            return self._with_plugin(results[:1] + [results[2]])

        if keyword == "learn" or lowered in {"learn", "经验", "发现"}:
            return self._with_plugin(results[:1] + [results[3]])

        if lowered in {"pause", "暂停"}:
            return self._with_plugin(
                [
                    {
                        "name": "暂停屏幕时间机器",
                        "path": "pause_time_machine",
                        "type": "work_memory_cmd",
                        "work_memory_hint": "暂停后台自动截图",
                    }
                ]
            )

        if lowered in {"resume", "start", "恢复", "开启"}:
            return self._with_plugin(
                [
                    {
                        "name": "开启屏幕时间机器",
                        "path": "resume_time_machine",
                        "type": "work_memory_cmd",
                        "work_memory_hint": "按配置间隔后台截图",
                    }
                ]
            )

        if lowered in {"capture", "shot", "补记", "截图"}:
            return self._with_plugin([results[1]])

        if keyword == "note" or lowered.startswith("note "):
            content = text[5:].strip() if lowered.startswith("note ") else text
            if content:
                return self._with_plugin(
                    [
                        {
                            "name": "添加工作记忆笔记",
                            "path": "note:" + content,
                            "type": "work_memory_cmd",
                            "work_memory_hint": content,
                        }
                    ]
                )

        if text:
            results.extend(work_memory_manager.as_search_results(text, limit=30))

        return self._with_plugin(results)

    def _with_plugin(self, results):
        for item in results:
            if isinstance(item, dict):
                item["plugin"] = self
        return results

    def get_preview_actions(self, item):
        item_type = str(item.get("type", ""))
        if item_type == "work_memory_cmd":
            return [
                {
                    "id": "plugin",
                    "label": "执行",
                    "icon": "run",
                    "command": str(item.get("path", "")),
                    "hide": False,
                    "refresh": True,
                }
            ]

        if item_type != "work_memory_entry":
            return []

        entry_id = str(item.get("work_memory_id") or item.get("path", "")).strip()
        if not entry_id:
            return []
        entry = work_memory_manager.get_entry(entry_id) or {}
        favorite = bool(entry.get("favorite"))
        sensitive = bool(entry.get("sensitive"))
        image_path = str(entry.get("image_path", "")).strip()
        file_path = str(entry.get("file_path", "")).strip()
        actions = [
            {
                "id": "plugin",
                "label": "查看证据",
                "icon": "open",
                "command": f"open:{entry_id}",
                "hide": False,
            },
            {
                "id": "plugin",
                "label": "复制文本",
                "icon": "copy",
                "command": f"copy:{entry_id}",
                "hide": False,
            },
            {
                "id": "plugin",
                "label": "取消收藏" if favorite else "收藏",
                "icon": "pin",
                "command": f"favorite:{entry_id}",
                "hide": False,
                "refresh": True,
            },
            {
                "id": "plugin",
                "label": "取消敏感" if sensitive else "标记敏感",
                "icon": "pin",
                "command": f"sensitive:{entry_id}",
                "hide": False,
                "refresh": True,
            },
            {
                "id": "plugin",
                "label": "知识草稿",
                "icon": "run",
                "command": f"knowledge:{entry_id}",
                "hide": False,
                "refresh": True,
            },
            {
                "id": "plugin",
                "label": "外部任务包",
                "icon": "run",
                "command": f"task:{entry_id}",
                "hide": False,
                "refresh": True,
            },
        ]
        if image_path or file_path:
            actions.append(
                {
                    "id": "plugin",
                    "label": "打开目录",
                    "icon": "folder",
                    "command": f"folder:{entry_id}",
                    "hide": False,
                }
            )
        if image_path:
            actions.append(
                {
                    "id": "plugin",
                    "label": "再次 OCR",
                    "icon": "run",
                    "command": f"ocr:{entry_id}",
                    "hide": False,
                    "refresh": True,
                }
            )
        return actions

    def handle_action(self, action):
        action_key = str(action or "").strip()
        if not action_key:
            return ""

        if action_key == "open_center":
            self._ensure_window()
            self.window.refresh_all()
            self.window.show()
            self.window.raise_()
            self.window.activateWindow()
            return "已打开工作记忆中心"

        if action_key == "capture_now":
            entry = work_memory_manager.capture_current_screen(
                source="time_machine", manual=True
            )
            return "已补记当前屏幕" if entry else "未补记: " + work_memory_manager.status()["pause_reason"]

        if action_key == "pause_time_machine":
            work_memory_manager.set_time_machine_enabled(False)
            return "已暂停屏幕时间机器"

        if action_key == "resume_time_machine":
            work_memory_manager.set_time_machine_enabled(True)
            return "已开启屏幕时间机器"

        if action_key == "daily_visible":
            entry = work_memory_manager.generate_daily_report()
            return "已生成日报草稿" if entry else "没有可生成日报的工作记忆"

        if action_key == "experience":
            entry = work_memory_manager.generate_experience_report()
            return "已生成经验发现报告" if entry else "没有可分析的工作记忆"

        if action_key.startswith("note:"):
            content = action_key.split(":", 1)[1].strip()
            entry = work_memory_manager.add_manual_note(content)
            return "已添加工作记忆笔记" if entry else "添加笔记失败"

        prefix, _, value = action_key.partition(":")
        entry_id = value.strip()
        if prefix in {
            "open",
            "folder",
            "copy",
            "favorite",
            "sensitive",
            "ocr",
            "daily",
            "knowledge",
            "retro",
            "task",
            "skill",
            "workflow",
        }:
            return self._handle_entry_action(prefix, entry_id)

        return ""

    def _entry_path(self, entry):
        if not entry:
            return ""
        for key in ["file_path", "image_path", "thumbnail_path"]:
            path = str(entry.get(key, "")).strip()
            if path and os.path.exists(path):
                return path
        return ""

    def _handle_entry_action(self, action, entry_id):
        entry = work_memory_manager.get_entry(entry_id)
        if not entry:
            return "工作记忆条目不存在"

        if action == "open":
            path = self._entry_path(entry)
            if path and open_path(path):
                return "已打开证据"
            self._ensure_window()
            self.window.show()
            return "已打开工作记忆中心"

        if action == "folder":
            path = self._entry_path(entry)
            if path and open_parent(path):
                return "已打开证据目录"
            return "没有可打开的目录"

        if action == "copy":
            text = entry.get("text") or entry.get("ocr_text") or entry.get("summary") or entry.get("title")
            clipboard = QApplication.clipboard()
            if clipboard is None:
                return "剪贴板不可用"
            clipboard.setText(str(text))
            return "已复制"

        if action == "favorite":
            pinned = work_memory_manager.toggle_favorite(entry_id)
            return "已收藏" if pinned else "已取消收藏"

        if action == "sensitive":
            sensitive = work_memory_manager.toggle_sensitive(entry_id)
            return "已标记敏感" if sensitive else "已取消敏感标记"

        if action == "ocr":
            ok, message = work_memory_manager.perform_ocr(entry_id)
            return "OCR 完成" if ok else f"OCR 不可用: {message}"

        if action == "daily":
            draft = work_memory_manager.generate_daily_report([entry_id])
            return "已生成日报草稿" if draft else "生成日报失败"

        if action == "knowledge":
            draft = work_memory_manager.generate_knowledge_draft([entry_id])
            return "已生成知识草稿" if draft else "生成知识草稿失败"

        if action == "retro":
            draft = work_memory_manager.generate_retro_draft([entry_id])
            return "已生成复盘草稿" if draft else "生成复盘失败"

        if action == "task":
            task = work_memory_manager.generate_external_agent_task([entry_id])
            return "已生成外部代理任务包" if task else "生成任务包失败"

        if action == "skill":
            suggestion = work_memory_manager.generate_asset_suggestion([entry_id], "skill")
            return "已生成 skill 建议草稿" if suggestion else "生成建议失败"

        if action == "workflow":
            suggestion = work_memory_manager.generate_asset_suggestion([entry_id], "workflow")
            return "已生成工作流建议草稿" if suggestion else "生成建议失败"

        return ""

    def on_enter(self):
        pass

    def on_exit(self):
        pass
