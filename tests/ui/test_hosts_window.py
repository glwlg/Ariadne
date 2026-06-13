import os
import unittest

os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")

from PyQt6.QtGui import QColor, QPalette
from PyQt6.QtWidgets import QApplication


_APP = QApplication.instance() or QApplication([])

from src.ui.hosts_window import CodeEditor


class TestHostsCodeEditor(unittest.TestCase):
    def test_light_theme_uses_dark_text_palette_for_caret(self):
        editor = CodeEditor()
        try:
            editor.set_theme(
                {
                    "text_dim": "#6B7280",
                    "selection_bg": "#DCEBFF",
                    "selection_text": "#111827",
                },
                dark=False,
            )

            self.assertEqual(
                editor.palette().color(QPalette.ColorRole.Text),
                QColor("#2C2C3A"),
            )
            self.assertEqual(
                editor.palette().color(QPalette.ColorRole.WindowText),
                QColor("#2C2C3A"),
            )
            self.assertEqual(editor.cursor_color, QColor("#2C2C3A"))
            self.assertEqual(
                editor.viewport().palette().color(QPalette.ColorRole.Text),
                QColor("#2C2C3A"),
            )
            self.assertGreaterEqual(editor.cursorWidth(), 2)
        finally:
            editor.close()


if __name__ == "__main__":
    unittest.main()
