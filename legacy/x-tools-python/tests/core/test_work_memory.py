import copy
import os
import tempfile
import unittest
import zipfile

os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")

from PyQt6.QtGui import QColor, QPixmap
from PyQt6.QtWidgets import QApplication

from src.core.config import config_manager
from src.core import work_memory as work_memory_module


app = QApplication.instance() or QApplication([])


class TestWorkMemoryManager(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.old_paths = {
            "WORK_MEMORY_DIR": work_memory_module.WORK_MEMORY_DIR,
            "ENTRY_FILE": work_memory_module.ENTRY_FILE,
            "IMAGE_DIR": work_memory_module.IMAGE_DIR,
            "THUMB_DIR": work_memory_module.THUMB_DIR,
            "EXPORT_DIR": work_memory_module.EXPORT_DIR,
            "DRAFT_DIR": work_memory_module.DRAFT_DIR,
            "TASK_DIR": work_memory_module.TASK_DIR,
        }
        work_memory_module.WORK_MEMORY_DIR = self.tmp.name
        work_memory_module.ENTRY_FILE = os.path.join(self.tmp.name, "entries.json")
        work_memory_module.IMAGE_DIR = os.path.join(self.tmp.name, "images")
        work_memory_module.THUMB_DIR = os.path.join(self.tmp.name, "thumbs")
        work_memory_module.EXPORT_DIR = os.path.join(self.tmp.name, "exports")
        work_memory_module.DRAFT_DIR = os.path.join(self.tmp.name, "drafts")
        work_memory_module.TASK_DIR = os.path.join(self.tmp.name, "agent_tasks")

        self.old_config = copy.deepcopy(config_manager.config.get("work_memory", {}))
        config = copy.deepcopy(work_memory_module.DEFAULT_WORK_MEMORY_CONFIG)
        config["external_agent_task_dir"] = work_memory_module.TASK_DIR
        config_manager.config["work_memory"] = config

        self.manager = work_memory_module.WorkMemoryManager()

    def tearDown(self):
        for key, value in self.old_paths.items():
            setattr(work_memory_module, key, value)
        config_manager.config["work_memory"] = self.old_config
        self.tmp.cleanup()

    def _pixmap_path(self):
        pixmap = QPixmap(24, 16)
        pixmap.fill(QColor("#336699"))
        path = os.path.join(self.tmp.name, "shot.png")
        self.assertTrue(pixmap.save(path, "PNG"))
        return path

    def test_note_sensitive_detection_and_search(self):
        entry = self.manager.add_manual_note("password=abc123 gateway token=secret")

        self.assertIsNotNone(entry)
        self.assertTrue(entry["sensitive"])
        self.assertFalse(entry["ai_allowed"])
        self.assertFalse(entry["vector_allowed"])

        results = self.manager.search("gateway")
        self.assertEqual(results[0]["id"], entry["id"])
        self.assertIn("关键词", results[0]["match_reason"])

    def test_clipboard_and_capture_entries_sync_to_search_results(self):
        clip = self.manager.add_clipboard_entry(
            {
                "id": "clip-1",
                "type": "text",
                "text": "Traceback: gateway failed",
                "created_at": 100,
                "pinned": True,
            }
        )
        capture = self.manager.add_capture_history_entry(
            {
                "id": "cap-1",
                "image_path": self._pixmap_path(),
                "created_at": 200,
                "source": "manual",
                "actions": ["copy"],
                "width": 24,
                "height": 16,
            }
        )

        self.assertEqual(clip["source"], "clipboard")
        self.assertTrue(clip["favorite"])
        self.assertEqual(capture["content_type"], "screenshot")
        self.assertTrue(os.path.exists(capture["thumbnail_path"]))

        result_types = {item["work_memory_source"] for item in self.manager.as_search_results("gateway")}
        self.assertIn("clipboard", result_types)

    def test_drafts_task_package_and_export_keep_evidence(self):
        first = self.manager.add_manual_note("排查 gateway 502 报错")
        second = self.manager.add_manual_note("todo: 补充知识库")
        ids = [first["id"], second["id"]]

        daily = self.manager.generate_daily_report(ids)
        knowledge = self.manager.generate_knowledge_draft(ids)
        retro = self.manager.generate_retro_draft(ids)
        experience = self.manager.generate_experience_report(days=30)
        task = self.manager.generate_external_agent_task(ids, goal="实现一个检查清单")
        export_path = self.manager.export_package(entry_ids=ids)

        for draft in [daily, knowledge, retro, experience, task]:
            self.assertIsNotNone(draft)
            self.assertTrue(os.path.exists(draft["file_path"]))
            self.assertIn("证据", draft["text"])

        self.assertTrue(os.path.exists(export_path))
        with zipfile.ZipFile(export_path) as zf:
            self.assertIn("README.md", zf.namelist())
            self.assertIn("entries.json", zf.namelist())

    def test_context_exclusion_prefers_privacy_rules(self):
        config_manager.config["work_memory"]["privacy_mode"] = True
        reason = self.manager._is_context_excluded({"process_name": "notepad.exe"})
        self.assertIn("隐私模式", reason)

        config_manager.config["work_memory"]["privacy_mode"] = False
        reason = self.manager._is_context_excluded({"window_title": "请输入密码"})
        self.assertIn("排除窗口", reason)

    def test_exclude_content_pattern_blocks_entry_before_indexing(self):
        config_manager.config["work_memory"]["exclude_content_patterns"] = ["do-not-index"]

        entry = self.manager.add_manual_note("this should do-not-index")

        self.assertIsNone(entry)
        self.assertEqual(self.manager.search("do-not-index"), [])


if __name__ == "__main__":
    unittest.main()
