import os
import tempfile
import unittest

from PyQt6.QtGui import QColor, QPixmap
from PyQt6.QtWidgets import QApplication

from src.core import capture_history as capture_history_module


app = QApplication.instance() or QApplication([])


class TestCaptureHistoryManager(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.old_xtools_dir = capture_history_module.XTOOLS_DIR
        self.old_history_file = capture_history_module.HISTORY_FILE
        self.old_image_dir = capture_history_module.IMAGE_DIR
        capture_history_module.XTOOLS_DIR = self.tmp.name
        capture_history_module.HISTORY_FILE = os.path.join(
            self.tmp.name, "capture_history.json"
        )
        capture_history_module.IMAGE_DIR = os.path.join(self.tmp.name, "capture_images")
        self.manager = capture_history_module.CaptureHistoryManager()

    def tearDown(self):
        capture_history_module.XTOOLS_DIR = self.old_xtools_dir
        capture_history_module.HISTORY_FILE = self.old_history_file
        capture_history_module.IMAGE_DIR = self.old_image_dir
        self.tmp.cleanup()

    def _pixmap(self):
        pixmap = QPixmap(12, 8)
        pixmap.fill(QColor("#FFFFFF"))
        return pixmap

    def test_add_capture_persists_history_copy_and_search_result(self):
        entry = self.manager.add_capture(
            self._pixmap(),
            source="manual",
            saved_path="C:/tmp/shot.png",
            actions=["copy", "save"],
        )

        self.assertIsNotNone(entry)
        self.assertTrue(os.path.exists(entry["image_path"]))
        self.assertEqual(entry["width"], 12)
        self.assertEqual(entry["height"], 8)

        results = self.manager.as_search_results("12x8")
        self.assertEqual(len(results), 1)
        self.assertEqual(results[0]["type"], "capture_entry")
        self.assertEqual(results[0]["capture_saved_path"], "C:/tmp/shot.png")

    def test_delete_removes_history_image(self):
        entry = self.manager.add_capture(self._pixmap())
        image_path = entry["image_path"]

        self.assertTrue(self.manager.delete_entry(entry["id"]))
        self.assertFalse(os.path.exists(image_path))
        self.assertEqual(self.manager.get_entries(), [])


if __name__ == "__main__":
    unittest.main()
