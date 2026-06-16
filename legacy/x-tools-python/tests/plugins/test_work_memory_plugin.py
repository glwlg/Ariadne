import copy
import os
import tempfile
import unittest

os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")

from PyQt6.QtWidgets import QApplication

from src.core.config import config_manager
from src.core import work_memory as work_memory_module
from src.plugins import work_memory_tool as work_memory_tool_module
from src.plugins.work_memory_tool import WorkMemoryPlugin


app = QApplication.instance() or QApplication([])


class TestWorkMemoryPlugin(unittest.TestCase):
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
        self.manager.sync_clipboard_history = lambda *args, **kwargs: 0
        self.manager.sync_capture_history = lambda *args, **kwargs: 0
        self.old_plugin_manager = work_memory_tool_module.work_memory_manager
        work_memory_tool_module.work_memory_manager = self.manager
        self.plugin = WorkMemoryPlugin()

    def tearDown(self):
        work_memory_tool_module.work_memory_manager = self.old_plugin_manager
        for key, value in self.old_paths.items():
            setattr(work_memory_module, key, value)
        config_manager.config["work_memory"] = self.old_config
        self.tmp.cleanup()

    def test_note_command_adds_memory_and_search_returns_entry(self):
        message = self.plugin.handle_action("note:gateway failed again")

        self.assertEqual(message, "已添加工作记忆笔记")
        results = self.plugin.execute("mem gateway")
        entry_results = [item for item in results if item["type"] == "work_memory_entry"]
        self.assertEqual(len(entry_results), 1)
        self.assertEqual(entry_results[0]["work_memory_source"], "note")

    def test_entry_preview_actions_are_plugin_actions(self):
        entry = self.manager.add_manual_note("gateway failed again")
        result = self.manager.as_search_results("gateway")[0]
        result["plugin"] = self.plugin

        actions = self.plugin.get_preview_actions(result)
        labels = [action["label"] for action in actions]

        self.assertIn("查看证据", labels)
        self.assertIn("复制文本", labels)
        self.assertIn("知识草稿", labels)
        self.assertNotIn("打开文件", labels)
        self.assertNotIn("打开所在文件夹", labels)

        message = self.plugin.handle_action(f"copy:{entry['id']}")
        self.assertEqual(message, "已复制")
        self.assertEqual(QApplication.clipboard().text(), "gateway failed again")

    def test_generates_task_package_from_entry(self):
        entry = self.manager.add_manual_note("需要把重复排查沉淀成 skill")

        message = self.plugin.handle_action(f"task:{entry['id']}")

        self.assertEqual(message, "已生成外部代理任务包")
        task_entries = self.manager.search("外部代理任务包")
        self.assertTrue(task_entries)
        self.assertTrue(os.path.exists(task_entries[0]["file_path"]))


if __name__ == "__main__":
    unittest.main()
