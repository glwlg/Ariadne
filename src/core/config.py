import json
import copy
import sys
import os
from src.core.logger import get_logger
from src.core.workflow_schema import DEFAULT_WORKFLOWS, normalize_workflows
from src.platform.startup import set_startup_enabled


logger = get_logger(__name__)

CONFIG_DIR = os.path.join(os.getenv("APPDATA") or os.path.expanduser("~"), "x-tools")
if not os.path.exists(CONFIG_DIR):
    os.makedirs(CONFIG_DIR)

CONFIG_FILE = os.path.join(CONFIG_DIR, "config.json")
DEFAULT_HOTKEYS = {
    "toggle_window": "alt+q",
    "screenshot": "alt+a",
    "pin_clipboard": "alt+v",
}

DEFAULT_CONFIG = {
    "run_on_startup": False,
    "theme": "Dark",
    "plugins_enabled": {},
    "hotkeys": DEFAULT_HOTKEYS.copy(),
    "screenshot_auto_save": False,
    "screenshot_auto_copy": False,
    "screenshot_auto_pin": False,
    "screenshot_save_dir": os.path.join(
        os.path.expanduser("~"), "Pictures", "x-tools-screenshots"
    ),
    "screenshot_filename_template": "x-tools_{date}_{time}",
    "workflows": copy.deepcopy(DEFAULT_WORKFLOWS),
    "custom_launch_items": [],
    "work_memory": {
        "enabled": True,
        "time_machine_enabled": False,
        "auto_capture_interval_seconds": 300,
        "capture_scope": "all_screens",
        "screenshot_quality": 90,
        "source_clipboard": True,
        "source_capture_history": True,
        "source_manual_note": True,
        "source_search_favorite": True,
        "source_actions": True,
        "auto_ocr": False,
        "embedding_enabled": False,
        "ai_enabled": False,
        "opscore_sync_enabled": False,
        "agents_sdk_enabled": False,
        "trace_mode": "off",
        "experience_discovery_enabled": True,
        "skill_suggestion_enabled": True,
        "workflow_suggestion_enabled": True,
        "external_agent_enabled": True,
        "codex_collaboration_enabled": False,
        "retention_days": 30,
        "thumbnail_retention_days": 90,
        "max_storage_mb": 1024,
        "privacy_mode": False,
        "exclude_apps": [
            "1password.exe",
            "bitwarden.exe",
            "keepass.exe",
            "lastpass.exe",
            "credentialuibroker.exe",
            "lockapp.exe",
            "logonui.exe",
            "mstsc.exe",
        ],
        "exclude_window_keywords": [
            "password",
            "token",
            "secret",
            "验证码",
            "密码",
            "登录",
            "支付",
            "隐私",
            "无痕",
            "远程桌面",
            "堡垒机",
            "vpn",
            "sso",
        ],
        "exclude_paths": [],
        "exclude_content_patterns": [],
        "allow_sensitive_export": False,
    },
}


def _deepcopy_default_config():
    return copy.deepcopy(DEFAULT_CONFIG)


THEME_FILE = os.path.join(CONFIG_DIR, "themes.json")
DEFAULT_THEMES = {
    "Dark": {
        "window_bg": "#1E1E1E",
        "input_bg": "#2D2D2D",
        "text_color": "#E0E0E0",
        "text_dim": "#909090",
        "highlight": "#007ACC",
        "border": "#3E3E3E",
        "selection_bg": "#094771",
        "selection_text": "#FFFFFF",
        "scrollbar_bg": "transparent",
        "scrollbar_handle": "#424242",
    },
    "Light": {
        "window_bg": "#F5F5F5",
        "input_bg": "#FFFFFF",
        "text_color": "#212121",
        "text_dim": "#757575",
        "highlight": "#0078D7",
        "border": "#DCDCDC",
        "selection_bg": "#CCE8FF",
        "selection_text": "#000000",
        "scrollbar_bg": "transparent",
        "scrollbar_handle": "#C1C1C1",
    },
}


class ConfigManager:
    def __init__(self):
        self.config = self.load_config()
        self.themes = self.load_themes()

    def load_config(self):
        if not os.path.exists(CONFIG_FILE):
            with open(CONFIG_FILE, "w") as f:
                json.dump(_deepcopy_default_config(), f, indent=4)
            return _deepcopy_default_config()

        try:
            with open(CONFIG_FILE, "r") as f:
                loaded = json.load(f)
                if not isinstance(loaded, dict):
                    return _deepcopy_default_config()

                merged = _deepcopy_default_config()
                merged.update(loaded)

                if not isinstance(merged.get("hotkeys"), dict):
                    merged["hotkeys"] = DEFAULT_HOTKEYS.copy()

                if not isinstance(merged.get("plugins_enabled"), dict):
                    merged["plugins_enabled"] = {}

                if not isinstance(merged.get("custom_launch_items"), list):
                    merged["custom_launch_items"] = []

                merged["workflows"] = normalize_workflows(merged.get("workflows"))

                if not isinstance(merged.get("work_memory"), dict):
                    merged["work_memory"] = copy.deepcopy(DEFAULT_CONFIG["work_memory"])
                else:
                    work_memory_defaults = copy.deepcopy(DEFAULT_CONFIG["work_memory"])
                    work_memory_defaults.update(merged["work_memory"])
                    merged["work_memory"] = work_memory_defaults

                return merged
        except:
            return _deepcopy_default_config()

    def save_config(self):
        try:
            with open(CONFIG_FILE, "w") as f:
                json.dump(self.config, f, indent=4)
        except Exception as e:
            logger.warning("Error saving config: %s", e)

    def load_themes(self):
        if not os.path.exists(THEME_FILE):
            with open(THEME_FILE, "w") as f:
                json.dump(DEFAULT_THEMES, f, indent=4)
            return DEFAULT_THEMES.copy()

        try:
            with open(THEME_FILE, "r") as f:
                themes = json.load(f)
                # Ensure all default themes and their keys exist
                for theme_name, default_theme_data in DEFAULT_THEMES.items():
                    if theme_name not in themes:
                        themes[theme_name] = default_theme_data.copy()
                    else:
                        # Merge keys for existing themes
                        for key, value in default_theme_data.items():
                            if key not in themes[theme_name]:
                                themes[theme_name][key] = value
                return themes
        except:
            return DEFAULT_THEMES.copy()

    def save_themes(self):
        try:
            with open(THEME_FILE, "w") as f:
                json.dump(self.themes, f, indent=4)
        except Exception as e:
            logger.warning("Error saving themes: %s", e)

    def get_theme_name(self):
        return self.config.get("theme", "Dark")

    def get_theme(self, theme_name=None):
        if not theme_name:
            theme_name = self.config.get("theme", "Dark")
        return self.themes.get(theme_name, self.themes.get("Dark"))

    def get_hotkey(self, action: str) -> str:
        """Get the hotkey string for an action, falling back to default."""
        hotkeys = self.config.get("hotkeys", {})
        return hotkeys.get(action, DEFAULT_HOTKEYS.get(action, ""))

    def set_hotkey(self, action: str, key_str: str):
        """Set the hotkey string for an action and save."""
        if "hotkeys" not in self.config:
            self.config["hotkeys"] = DEFAULT_HOTKEYS.copy()
        self.config["hotkeys"][action] = key_str
        self.save_config()

    def get_value(self, key: str, default=None):
        return self.config.get(key, default)

    def set_value(self, key: str, value):
        self.config[key] = value
        self.save_config()

    def get_workflows(self):
        return normalize_workflows(self.config.get("workflows"))

    def set_workflows(self, workflows):
        self.config["workflows"] = normalize_workflows(workflows)
        self.save_config()

    def set_startup(self, enable=True):
        app_name = "x-tools"
        exe_path = sys.executable  # For packaged app, sys.executable is the exe path

        # If running as script, use python exe + script path? Or just warn.
        # For development (uv run), sys.executable is python.exe.
        # We should use sys.argv[0] if possible but complex.
        # But user asked for packaged app, so let's implement for packaged mainly.
        # However, for testing, we can use the script path.
        if getattr(sys, "frozen", False):
            path_to_run = f'"{exe_path}"'
        else:
            script_path = os.path.abspath(sys.argv[0])
            path_to_run = f'"{exe_path}" "{script_path}"'  # Run via python

        if set_startup_enabled(app_name, path_to_run, enable):
            self.config["run_on_startup"] = enable
            self.save_config()
            return True

        logger.warning("Startup integration is not available on this platform.")
        return False


config_manager = ConfigManager()
