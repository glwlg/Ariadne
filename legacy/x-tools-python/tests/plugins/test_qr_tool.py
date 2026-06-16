import unittest

from src.plugins.qr_tool import QRCodePlugin


class TestQRCodePlugin(unittest.TestCase):
    def test_execute_returns_memory_only_result_with_original_text(self):
        plugin = QRCodePlugin()

        results = plugin.execute("hello world")

        self.assertEqual(len(results), 1)
        result = results[0]
        self.assertEqual(result["name"], "生成二维码")
        self.assertEqual(result["type"], "qr_generate")
        self.assertEqual(result["qr_text"], "hello world")
        self.assertEqual(result["path"], "")


if __name__ == "__main__":
    unittest.main()
