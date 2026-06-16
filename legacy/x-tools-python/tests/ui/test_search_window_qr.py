import os
import tempfile
import unittest
from unittest.mock import Mock, patch

os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")

from PyQt6.QtWidgets import QApplication


_APP = QApplication.instance() or QApplication([])

from src.plugins.qr_tool import QRCodePlugin
from src.ui.search_window import SearchWindow


class TestSearchWindowQRCodeResult(unittest.TestCase):
    def _preview_window(self):
        window = SearchWindow.__new__(SearchWindow)
        window.plugin_mode = None
        return window

    def test_qr_action_uses_original_text_not_generated_file_path(self):
        plugin = Mock()
        window = type(
            "SearchWindowShim",
            (),
            {
                "plugin_mode": None,
                "_record_item_usage": Mock(),
                "hide": Mock(),
            },
        )()

        SearchWindow.handle_item_action(
            window,
            {
                "type": "qr_generate",
                "path": r"C:\tmp\qr.png",
                "qr_text": "hello world",
                "plugin": plugin,
            },
        )

        plugin.handle_action.assert_called_once_with("hello world")

    def test_qr_preview_uses_generated_image_pixmap(self):
        plugin = QRCodePlugin()
        result = plugin.execute("hello world")[0]
        result["plugin"] = plugin

        pixmap = SearchWindow._preview_pixmap_for_item(object(), result, 220)

        self.assertIsNotNone(pixmap)
        self.assertFalse(pixmap.isNull())
        self.assertLessEqual(pixmap.width(), 220)
        self.assertLessEqual(pixmap.height(), 220)

    def test_qr_result_has_no_file_location_text(self):
        result = QRCodePlugin().execute("hello world")[0]

        self.assertEqual(SearchWindow._item_location_text(object(), result), "")

    def test_copy_result_preview_only_offers_copy_action(self):
        window = self._preview_window()
        actions = window._preview_actions_for_item(
            {
                "type": "copy_result",
                "name": "编码结果: MTIz",
                "path": "MTIz",
            }
        )

        self.assertEqual([action["label"] for action in actions], ["复制结果"])

    def test_copy_preview_action_uses_button_feedback_not_tray(self):
        window = type(
            "SearchWindowShim",
            (),
            {
                "_current_preview_data": {
                    "type": "copy_result",
                    "name": "编码结果: MTIz",
                    "path": "MTIz",
                },
                "_record_item_usage": Mock(),
                "_show_preview_action_feedback": Mock(),
                "_show_tray_message": Mock(),
            },
        )()

        SearchWindow._trigger_preview_action(
            window,
            {"id": "copy_value", "label": "复制结果", "target": "MTIz"},
            0,
        )

        self.assertEqual(QApplication.clipboard().text(), "MTIz")
        window._record_item_usage.assert_called_once_with(window._current_preview_data)
        window._show_preview_action_feedback.assert_called_once_with(0, "已复制")
        window._show_tray_message.assert_not_called()

    def test_plugin_preview_actions_replace_file_affordances(self):
        window = self._preview_window()
        plugin = Mock()
        plugin.get_preview_actions.return_value = [
            {"label": "插件动作", "command": "open_hosts", "icon": "open"}
        ]

        actions = window._preview_actions_for_item(
            {
                "type": "hosts_cmd",
                "name": "打开 Hosts 管理",
                "path": "open_hosts",
                "plugin": plugin,
            }
        )

        self.assertEqual([action["label"] for action in actions], ["插件动作"])
        self.assertNotIn("打开文件", [action["label"] for action in actions])
        self.assertNotIn("打开所在文件夹", [action["label"] for action in actions])

    def test_file_preview_keeps_file_actions(self):
        window = self._preview_window()
        fd, path = tempfile.mkstemp()
        os.close(fd)
        try:
            actions = window._preview_actions_for_item(
                {"type": "file", "name": os.path.basename(path), "path": path}
            )
        finally:
            os.unlink(path)

        labels = [action["label"] for action in actions]
        self.assertEqual(labels[:3], ["打开文件", "打开所在文件夹", "复制路径"])
        self.assertIn("加入记忆", labels)

    def test_remember_preview_action_adds_result_to_work_memory(self):
        window = type(
            "SearchWindowShim",
            (),
            {
                "_current_preview_data": {
                    "type": "file",
                    "name": "demo.txt",
                    "path": r"C:\tmp\demo.txt",
                },
                "search_bar": Mock(text=Mock(return_value="demo")),
                "_record_item_usage": Mock(),
                "_show_preview_action_feedback": Mock(),
            },
        )()

        with patch(
            "src.ui.search_window.work_memory_manager.add_favorite_item",
            return_value={"id": "mem-1"},
        ) as add_mock:
            SearchWindow._trigger_preview_action(
                window,
                {"id": "remember", "label": "加入记忆"},
                0,
            )

        add_mock.assert_called_once_with(window._current_preview_data, query="demo")
        window._show_preview_action_feedback.assert_called_once_with(0, "已加入")


if __name__ == "__main__":
    unittest.main()
