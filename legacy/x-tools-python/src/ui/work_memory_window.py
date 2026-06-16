from __future__ import annotations

import os
from datetime import datetime

from PyQt6.QtCore import QDateTime, Qt
from PyQt6.QtGui import QColor, QPixmap
from PyQt6.QtWidgets import (
    QApplication,
    QCheckBox,
    QComboBox,
    QDateTimeEdit,
    QFileDialog,
    QFrame,
    QGridLayout,
    QHBoxLayout,
    QLabel,
    QListWidgetItem,
    QLineEdit,
    QMessageBox,
    QPlainTextEdit,
    QScrollArea,
    QSizePolicy,
    QSpinBox,
    QVBoxLayout,
    QWidget,
)
from qfluentwidgets import ListWidget, PrimaryPushButton, PushButton, SearchLineEdit
from qframelesswindow import AcrylicWindow

from src.core.work_memory import (
    TYPE_LABELS,
    SOURCE_LABELS,
    update_work_memory_config,
    work_memory_config,
    work_memory_manager,
)
from src.platform.shell import open_parent, open_path


class WorkMemoryWindow(AcrylicWindow):
    def __init__(self, manager=None, parent=None):
        super().__init__(parent)
        self.manager = manager or work_memory_manager
        self._current_results = []
        self._status_text = ""

        self.setWindowTitle("工作记忆中心")
        self.setWindowFlag(Qt.WindowType.WindowStaysOnTopHint, False)
        self.titleBar.raise_()
        self.titleBar.setStyleSheet("QFrame { background: transparent; }")
        self.setMinimumSize(1160, 740)
        self.resize(1280, 820)

        self._build_ui()
        self.manager.entries_changed.connect(self.refresh_all)
        self.manager.status_changed.connect(self.refresh_status)
        self.refresh_all()

    def _build_ui(self):
        self.titleBar.raise_()
        self.container = QWidget(self)
        self.container.setObjectName("workMemoryShell")

        root = QVBoxLayout(self)
        root.setContentsMargins(0, 0, 0, 0)
        root.addWidget(self.container)

        layout = QVBoxLayout(self.container)
        layout.setContentsMargins(18, 18, 18, 18)
        layout.setSpacing(12)

        header_card = QFrame(self.container)
        header_card.setObjectName("workMemoryHeader")
        header_layout = QHBoxLayout(header_card)
        header_layout.setContentsMargins(20, 16, 20, 16)
        header_layout.setSpacing(16)

        title_block = QVBoxLayout()
        title_block.setContentsMargins(0, 0, 0, 0)
        title_block.setSpacing(4)

        self.title_label = QLabel("工作记忆中心", header_card)
        self.title_label.setObjectName("workMemoryTitle")
        title_block.addWidget(self.title_label)

        self.status_label = QLabel("", header_card)
        self.status_label.setObjectName("workMemoryStatusLine")
        self.status_label.setWordWrap(True)
        title_block.addWidget(self.status_label)
        header_layout.addLayout(title_block, 1)

        control_row = QHBoxLayout()
        control_row.setContentsMargins(0, 0, 0, 0)
        control_row.setSpacing(8)

        self.memory_toggle_btn = PushButton("工作记忆", header_card)
        self.memory_toggle_btn.clicked.connect(self.toggle_memory)
        control_row.addWidget(self.memory_toggle_btn)

        self.time_toggle_btn = PrimaryPushButton("屏幕时间机器", header_card)
        self.time_toggle_btn.clicked.connect(self.toggle_time_machine)
        control_row.addWidget(self.time_toggle_btn)

        self.privacy_btn = PushButton("隐私模式", header_card)
        self.privacy_btn.clicked.connect(self.toggle_privacy_mode)
        control_row.addWidget(self.privacy_btn)

        self.capture_btn = PushButton("手动补记屏幕", header_card)
        self.capture_btn.clicked.connect(self.capture_now)
        control_row.addWidget(self.capture_btn)
        header_layout.addLayout(control_row)
        layout.addWidget(header_card)

        stat_row = QHBoxLayout()
        stat_row.setContentsMargins(0, 0, 0, 0)
        stat_row.setSpacing(12)

        def add_stat_card(title):
            card = QFrame(self.container)
            card.setObjectName("workMemoryStatCard")
            card.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Fixed)
            card_layout = QVBoxLayout(card)
            card_layout.setContentsMargins(14, 12, 14, 12)
            card_layout.setSpacing(4)
            value_label = QLabel("-", card)
            value_label.setObjectName("workMemoryStatValue")
            title_label = QLabel(title, card)
            title_label.setObjectName("workMemoryStatTitle")
            card_layout.addWidget(value_label)
            card_layout.addWidget(title_label)
            stat_row.addWidget(card)
            return value_label

        self.memory_state_value = add_stat_card("工作记忆")
        self.capture_state_value = add_stat_card("最近截图")
        self.storage_state_value = add_stat_card("本地存储")
        self.ai_state_value = add_stat_card("AI / 检索")
        layout.addLayout(stat_row)

        settings_card = QFrame(self.container)
        settings_card.setObjectName("workMemoryControlCard")
        settings_outer = QVBoxLayout(settings_card)
        settings_outer.setContentsMargins(14, 12, 14, 14)
        settings_outer.setSpacing(10)

        settings_title = QLabel("采集与隐私", settings_card)
        settings_title.setObjectName("workMemorySectionTitle")
        settings_outer.addWidget(settings_title)

        settings_grid = QGridLayout()
        settings_grid.setContentsMargins(0, 0, 0, 0)
        settings_grid.setHorizontalSpacing(10)
        settings_grid.setVerticalSpacing(10)

        self.interval_spin = QSpinBox(self)
        self.interval_spin.setRange(10, 3600)
        self.interval_spin.setSuffix(" 秒")
        settings_grid.addWidget(QLabel("间隔"), 0, 0)
        settings_grid.addWidget(self.interval_spin, 0, 1)

        self.quality_spin = QSpinBox(self)
        self.quality_spin.setRange(30, 100)
        settings_grid.addWidget(QLabel("质量"), 0, 2)
        settings_grid.addWidget(self.quality_spin, 0, 3)

        self.scope_combo = QComboBox(self)
        self.scope_combo.addItem("全部屏幕", "all_screens")
        self.scope_combo.addItem("前台窗口", "foreground_window")
        settings_grid.addWidget(QLabel("范围"), 0, 4)
        settings_grid.addWidget(self.scope_combo, 0, 5)

        self.ocr_check = QCheckBox("自动 OCR")
        self.embedding_check = QCheckBox("embedding")
        self.ai_check = QCheckBox("AI 草稿")
        self.codex_check = QCheckBox("Codex 协作")
        settings_grid.addWidget(self.ocr_check, 0, 6)
        settings_grid.addWidget(self.embedding_check, 0, 7)
        settings_grid.addWidget(self.ai_check, 0, 8)
        settings_grid.addWidget(self.codex_check, 0, 9)

        self.exclude_apps_edit = QLineEdit(self)
        self.exclude_apps_edit.setPlaceholderText("排除应用，逗号分隔")
        settings_grid.addWidget(QLabel("排除应用"), 1, 0)
        settings_grid.addWidget(self.exclude_apps_edit, 1, 1, 1, 3)

        self.exclude_titles_edit = QLineEdit(self)
        self.exclude_titles_edit.setPlaceholderText("排除窗口关键词，逗号分隔")
        settings_grid.addWidget(QLabel("排除窗口"), 1, 4)
        settings_grid.addWidget(self.exclude_titles_edit, 1, 5, 1, 3)

        self.exclude_paths_edit = QLineEdit(self)
        self.exclude_paths_edit.setPlaceholderText("排除路径，逗号分隔")
        settings_grid.addWidget(QLabel("排除路径"), 2, 0)
        settings_grid.addWidget(self.exclude_paths_edit, 2, 1, 1, 7)

        self.save_config_btn = PushButton("保存配置")
        self.save_config_btn.clicked.connect(self.save_config_controls)
        settings_grid.addWidget(self.save_config_btn, 2, 8, 1, 2)
        settings_outer.addLayout(settings_grid)
        layout.addWidget(settings_card)

        filter_card = QFrame(self.container)
        filter_card.setObjectName("workMemoryFilterCard")
        filter_layout = QVBoxLayout(filter_card)
        filter_layout.setContentsMargins(14, 12, 14, 14)
        filter_layout.setSpacing(10)

        filter_title = QLabel("检索工作记忆", filter_card)
        filter_title.setObjectName("workMemorySectionTitle")
        filter_layout.addWidget(filter_title)

        filter_row = QHBoxLayout()
        filter_row.setContentsMargins(0, 0, 0, 0)
        filter_row.setSpacing(8)

        self.search_edit = SearchLineEdit(self)
        self.search_edit.setPlaceholderText("搜索 OCR、剪贴板、截图、笔记、路径、窗口、标签或自然语言问题...")
        self.search_edit.textChanged.connect(self.refresh_list)
        filter_row.addWidget(self.search_edit, 1)
        filter_layout.addLayout(filter_row)

        filter_options_row = QHBoxLayout()
        filter_options_row.setContentsMargins(0, 0, 0, 0)
        filter_options_row.setSpacing(8)

        self.source_filter = QComboBox(self)
        self.source_filter.currentIndexChanged.connect(self.refresh_list)
        filter_options_row.addWidget(self.source_filter)

        self.type_filter = QComboBox(self)
        self.type_filter.currentIndexChanged.connect(self.refresh_list)
        filter_options_row.addWidget(self.type_filter)

        self.favorite_filter = QComboBox(self)
        self.favorite_filter.addItem("全部", None)
        self.favorite_filter.addItem("仅收藏", True)
        self.favorite_filter.addItem("未收藏", False)
        self.favorite_filter.currentIndexChanged.connect(self.refresh_list)
        filter_options_row.addWidget(self.favorite_filter)

        self.app_filter = SearchLineEdit(self)
        self.app_filter.setPlaceholderText("应用")
        self.app_filter.textChanged.connect(self.refresh_list)
        filter_options_row.addWidget(self.app_filter)

        self.tag_filter = SearchLineEdit(self)
        self.tag_filter.setPlaceholderText("标签")
        self.tag_filter.textChanged.connect(self.refresh_list)
        filter_options_row.addWidget(self.tag_filter)

        self.daily_filter = QComboBox(self)
        self.daily_filter.addItem("日报状态", None)
        self.daily_filter.addItem("已纳入日报", True)
        self.daily_filter.addItem("未纳入日报", False)
        self.daily_filter.currentIndexChanged.connect(self.refresh_list)
        filter_options_row.addWidget(self.daily_filter)

        self.knowledge_filter = QComboBox(self)
        self.knowledge_filter.addItem("知识状态", None)
        self.knowledge_filter.addItem("已生成知识", True)
        self.knowledge_filter.addItem("未生成知识", False)
        self.knowledge_filter.currentIndexChanged.connect(self.refresh_list)
        filter_options_row.addWidget(self.knowledge_filter)

        filter_options_row.addStretch(1)
        filter_layout.addLayout(filter_options_row)
        layout.addWidget(filter_card)

        content_row = QHBoxLayout()
        content_row.setContentsMargins(0, 0, 0, 0)
        content_row.setSpacing(12)

        timeline_card = QFrame(self.container)
        timeline_card.setObjectName("workMemoryTimelineCard")
        timeline_layout = QVBoxLayout(timeline_card)
        timeline_layout.setContentsMargins(14, 12, 14, 14)
        timeline_layout.setSpacing(10)

        timeline_header = QHBoxLayout()
        timeline_header.setContentsMargins(0, 0, 0, 0)
        timeline_header.setSpacing(8)
        timeline_title = QLabel("时间线", timeline_card)
        timeline_title.setObjectName("workMemorySectionTitle")
        timeline_header.addWidget(timeline_title)
        timeline_header.addStretch(1)

        self.prev_btn = PushButton("上一条", timeline_card)
        self.prev_btn.clicked.connect(lambda: self.move_selection(-1))
        timeline_header.addWidget(self.prev_btn)

        self.next_btn = PushButton("下一条", timeline_card)
        self.next_btn.clicked.connect(lambda: self.move_selection(1))
        timeline_header.addWidget(self.next_btn)

        self.jump_time = QDateTimeEdit(QDateTime.currentDateTime(), timeline_card)
        self.jump_time.setDisplayFormat("yyyy-MM-dd HH:mm")
        timeline_header.addWidget(self.jump_time)

        self.jump_btn = PushButton("跳转", timeline_card)
        self.jump_btn.clicked.connect(self.jump_to_time)
        timeline_header.addWidget(self.jump_btn)
        timeline_layout.addLayout(timeline_header)

        self.timeline_list = ListWidget(self)
        self.timeline_list.setObjectName("workMemoryTimelineList")
        self.timeline_list.currentItemChanged.connect(
            lambda current, _previous: self.show_entry_preview(current)
        )
        self.timeline_list.itemDoubleClicked.connect(lambda _item: self.open_current_evidence())
        timeline_layout.addWidget(self.timeline_list, 1)
        content_row.addWidget(timeline_card, 3)

        self.detail_scroll = QScrollArea(self)
        self.detail_scroll.setObjectName("workMemoryDetailScroll")
        self.detail_scroll.setWidgetResizable(True)
        self.detail_scroll.setFrameShape(QScrollArea.Shape.NoFrame)

        self.detail_panel = QFrame(self)
        self.detail_panel.setObjectName("workMemoryDetailPanel")
        detail_layout = QVBoxLayout(self.detail_panel)
        detail_layout.setContentsMargins(16, 14, 16, 16)
        detail_layout.setSpacing(12)

        detail_header = QVBoxLayout()
        detail_header.setContentsMargins(0, 0, 0, 0)
        detail_header.setSpacing(3)
        detail_title = QLabel("证据预览", self.detail_panel)
        detail_title.setObjectName("workMemorySectionTitle")
        detail_caption = QLabel("查看原始截图、OCR、路径和可追溯证据", self.detail_panel)
        detail_caption.setObjectName("workMemoryCaption")
        detail_header.addWidget(detail_title)
        detail_header.addWidget(detail_caption)
        detail_layout.addLayout(detail_header)

        action_grid = QGridLayout()
        action_grid.setContentsMargins(0, 0, 0, 0)
        action_grid.setHorizontalSpacing(8)
        action_grid.setVerticalSpacing(8)

        self.copy_btn = PushButton("复制文本")
        self.copy_btn.clicked.connect(self.copy_current_text)
        action_grid.addWidget(self.copy_btn, 0, 0)

        self.open_btn = PushButton("打开证据")
        self.open_btn.clicked.connect(self.open_current_evidence)
        action_grid.addWidget(self.open_btn, 0, 1)

        self.folder_btn = PushButton("打开目录")
        self.folder_btn.clicked.connect(self.open_current_folder)
        action_grid.addWidget(self.folder_btn, 0, 2)

        self.favorite_btn = PushButton("收藏/取消")
        self.favorite_btn.clicked.connect(self.toggle_current_favorite)
        action_grid.addWidget(self.favorite_btn, 1, 0)

        self.sensitive_btn = PushButton("敏感标记")
        self.sensitive_btn.clicked.connect(self.toggle_current_sensitive)
        action_grid.addWidget(self.sensitive_btn, 1, 1)

        self.ocr_btn = PushButton("再次 OCR")
        self.ocr_btn.clicked.connect(self.ocr_current)
        action_grid.addWidget(self.ocr_btn, 1, 2)

        self.daily_btn = PushButton("生成日报")
        self.daily_btn.clicked.connect(self.generate_daily_report)
        action_grid.addWidget(self.daily_btn, 2, 0)

        self.knowledge_btn = PushButton("知识草稿")
        self.knowledge_btn.clicked.connect(self.generate_knowledge_draft)
        action_grid.addWidget(self.knowledge_btn, 2, 1)

        self.retro_btn = PushButton("问题复盘")
        self.retro_btn.clicked.connect(self.generate_retro_draft)
        action_grid.addWidget(self.retro_btn, 2, 2)

        self.learn_btn = PushButton("经验发现")
        self.learn_btn.clicked.connect(self.generate_experience_report)
        action_grid.addWidget(self.learn_btn, 3, 0)

        self.task_btn = PushButton("外部任务包")
        self.task_btn.clicked.connect(self.generate_task_package)
        action_grid.addWidget(self.task_btn, 3, 1)

        self.export_btn = PushButton("导出可见")
        self.export_btn.clicked.connect(self.export_visible)
        action_grid.addWidget(self.export_btn, 3, 2)

        self.skill_btn = PushButton("Skill 建议")
        self.skill_btn.clicked.connect(lambda: self.generate_asset_suggestion("skill"))
        action_grid.addWidget(self.skill_btn, 4, 0)

        self.workflow_btn = PushButton("工作流建议")
        self.workflow_btn.clicked.connect(
            lambda: self.generate_asset_suggestion("workflow")
        )
        action_grid.addWidget(self.workflow_btn, 4, 1)

        self.checklist_btn = PushButton("检查清单")
        self.checklist_btn.clicked.connect(
            lambda: self.generate_asset_suggestion("checklist")
        )
        action_grid.addWidget(self.checklist_btn, 4, 2)

        self.prompt_btn = PushButton("提示词模板")
        self.prompt_btn.clicked.connect(lambda: self.generate_asset_suggestion("prompt"))
        action_grid.addWidget(self.prompt_btn, 5, 0)

        self.import_btn = PushButton("导入材料")
        self.import_btn.clicked.connect(self.import_material)
        action_grid.addWidget(self.import_btn, 5, 1)

        self.clear_btn = PushButton("清理未收藏")
        self.clear_btn.clicked.connect(self.clear_unfavorited)
        action_grid.addWidget(self.clear_btn, 5, 2)

        self.clear_before_btn = PushButton("清理时间前")
        self.clear_before_btn.clicked.connect(self.clear_before_jump_time)
        action_grid.addWidget(self.clear_before_btn, 6, 0)

        self.delete_btn = PushButton("删除条目")
        self.delete_btn.clicked.connect(self.delete_current)
        action_grid.addWidget(self.delete_btn, 6, 1)

        self.close_btn = PushButton("关闭")
        self.close_btn.clicked.connect(self.hide)
        action_grid.addWidget(self.close_btn, 6, 2)

        self.preview_image = QLabel("未选择条目")
        self.preview_image.setObjectName("workMemoryPreviewImage")
        self.preview_image.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self.preview_image.setMinimumHeight(170)
        self.preview_image.setMaximumHeight(240)
        detail_layout.addWidget(self.preview_image)

        self.detail_label = QLabel("")
        self.detail_label.setObjectName("workMemoryDetailLabel")
        self.detail_label.setWordWrap(True)
        detail_layout.addWidget(self.detail_label)

        self.text_preview = QPlainTextEdit(self)
        self.text_preview.setReadOnly(True)
        self.text_preview.setMinimumHeight(140)
        detail_layout.addWidget(self.text_preview)

        note_row = QHBoxLayout()
        self.note_edit = SearchLineEdit(self)
        self.note_edit.setPlaceholderText("写一条手动笔记，回车或点击添加")
        self.note_edit.returnPressed.connect(self.add_note)
        note_row.addWidget(self.note_edit, 1)
        self.note_btn = PushButton("添加笔记")
        self.note_btn.clicked.connect(self.add_note)
        note_row.addWidget(self.note_btn)
        detail_layout.addLayout(note_row)
        detail_layout.addLayout(action_grid)

        self.feedback_label = QLabel("")
        self.feedback_label.setObjectName("workMemoryFeedback")
        detail_layout.addWidget(self.feedback_label)

        self.detail_scroll.setWidget(self.detail_panel)
        content_row.addWidget(self.detail_scroll, 2)
        layout.addLayout(content_row, 1)

        self.setStyleSheet(
            """
            WorkMemoryWindow {
                background-color: transparent;
                font-family: "Microsoft YaHei UI", "Segoe UI", sans-serif;
            }
            #workMemoryShell {
                background-color: #F4F7FB;
                border: 1px solid rgba(15, 23, 42, 28);
                border-radius: 0px;
            }
            QFrame#workMemoryHeader {
                background-color: #FFFFFF;
                border: 1px solid #E2E8F0;
                border-radius: 8px;
            }
            QFrame#workMemoryStatCard,
            QFrame#workMemoryControlCard,
            QFrame#workMemoryFilterCard,
            QFrame#workMemoryTimelineCard,
            QFrame#workMemoryDetailPanel {
                background-color: #FFFFFF;
                border: 1px solid #E2E8F0;
                border-radius: 8px;
            }
            QLabel {
                color: #1E293B;
            }
            QLabel#workMemoryTitle {
                color: #0F172A;
                font-size: 22px;
                font-weight: 700;
            }
            QLabel#workMemoryStatusLine,
            QLabel#workMemoryCaption,
            QLabel#workMemoryFeedback,
            QLabel#workMemoryStatTitle {
                color: #64748B;
                font-size: 12px;
            }
            QLabel#workMemorySectionTitle {
                color: #111827;
                font-size: 14px;
                font-weight: 700;
            }
            QLabel#workMemoryStatValue {
                color: #0F172A;
                font-size: 18px;
                font-weight: 700;
            }
            QScrollArea#workMemoryDetailScroll {
                background: transparent;
                border: none;
            }
            QLabel#workMemoryPreviewImage {
                background-color: #F8FAFC;
                color: #94A3B8;
                border: 1px dashed #CBD5E1;
                border-radius: 8px;
            }
            QLabel#workMemoryDetailLabel {
                color: #334155;
                font-size: 13px;
                line-height: 1.4;
            }
            QPlainTextEdit {
                background-color: #F8FAFC;
                color: #1E293B;
                border: 1px solid #E2E8F0;
                border-radius: 8px;
                padding: 8px;
                font-family: Consolas, "Microsoft YaHei UI";
                font-size: 12px;
            }
            ListWidget#workMemoryTimelineList {
                background-color: transparent;
                border: none;
                outline: none;
            }
            ListWidget#workMemoryTimelineList::item {
                color: #334155;
                min-height: 54px;
                padding: 9px 10px;
                border-radius: 8px;
            }
            ListWidget#workMemoryTimelineList::item:hover {
                background-color: #EFF6FF;
            }
            ListWidget#workMemoryTimelineList::item:selected {
                background-color: #DBEAFE;
                color: #0F172A;
            }
            SearchLineEdit, QLineEdit, QDateTimeEdit, QSpinBox {
                min-height: 32px;
                color: #1E293B;
                background-color: #FFFFFF;
                border: 1px solid #CBD5E1;
                border-radius: 7px;
                padding-left: 8px;
                padding-right: 8px;
            }
            SearchLineEdit:focus, QLineEdit:focus, QDateTimeEdit:focus, QSpinBox:focus {
                border: 1px solid #2563EB;
                background-color: #FFFFFF;
            }
            QComboBox {
                min-height: 32px;
                min-width: 110px;
                color: #1E293B;
                background-color: #FFFFFF;
                border: 1px solid #CBD5E1;
                border-radius: 7px;
                padding-left: 8px;
                padding-right: 8px;
            }
            QCheckBox {
                color: #334155;
                spacing: 6px;
            }
            QPushButton {
                min-height: 32px;
                border-radius: 7px;
                padding-left: 12px;
                padding-right: 12px;
            }
            QPushButton:hover {
                background-color: #EFF6FF;
            }
            QScrollBar:vertical {
                background: transparent;
                width: 10px;
                margin: 2px;
            }
            QScrollBar::handle:vertical {
                background: #CBD5E1;
                border-radius: 5px;
                min-height: 32px;
            }
            QScrollBar::handle:vertical:hover {
                background: #94A3B8;
            }
            QScrollBar::add-line:vertical,
            QScrollBar::sub-line:vertical {
                height: 0px;
            }
            """
        )
        self.titleBar.raise_()
        self.load_config_controls()

    @staticmethod
    def _format_time(ts):
        try:
            return datetime.fromtimestamp(float(ts)).strftime("%m-%d %H:%M:%S")
        except Exception:
            return ""

    def refresh_all(self):
        self.refresh_filter_options()
        self.refresh_status()
        self.refresh_list()

    def refresh_filter_options(self):
        current_source = self.source_filter.currentData() if self.source_filter.count() else ""
        current_type = self.type_filter.currentData() if self.type_filter.count() else ""

        self.source_filter.blockSignals(True)
        self.source_filter.clear()
        self.source_filter.addItem("全部来源", "")
        for key, label in self.manager.source_options():
            self.source_filter.addItem(label or key, key)
        index = self.source_filter.findData(current_source)
        self.source_filter.setCurrentIndex(index if index >= 0 else 0)
        self.source_filter.blockSignals(False)

        self.type_filter.blockSignals(True)
        self.type_filter.clear()
        self.type_filter.addItem("全部类型", "")
        for key, label in self.manager.type_options():
            self.type_filter.addItem(label or key, key)
        index = self.type_filter.findData(current_type)
        self.type_filter.setCurrentIndex(index if index >= 0 else 0)
        self.type_filter.blockSignals(False)

    def refresh_status(self):
        status = self.manager.status()
        storage = status["storage"]
        running = "记录中" if status["running"] else "已暂停"
        enabled = "启用" if status["enabled"] else "关闭"
        privacy = "隐私模式" if status["privacy_mode"] else "正常"
        lines = [
            f"{enabled} · 时间机器{running} · {privacy}",
            f"范围 {status['scope']} · 暂停原因 {status['pause_reason']}",
        ]
        self.status_label.setText("\n".join(lines))

        if hasattr(self, "memory_state_value"):
            self.memory_state_value.setText(f"{enabled} / {privacy}")
            self.capture_state_value.setText(str(status["last_capture_text"]))
            self.storage_state_value.setText(
                f"{storage['entry_count']} 条 · {storage['mb']:.2f} MB"
            )
            self.ai_state_value.setText(
                f"AI {status['ai_status']} · 向量 {status['embedding_status']}"
            )

        cfg = work_memory_config()
        self.memory_toggle_btn.setText("暂停工作记忆" if cfg.get("enabled", True) else "启用工作记忆")
        self.time_toggle_btn.setText(
            "暂停屏幕时间机器" if cfg.get("time_machine_enabled", False) else "开启屏幕时间机器"
        )
        self.privacy_btn.setText("退出隐私模式" if cfg.get("privacy_mode", False) else "进入隐私模式")

    @staticmethod
    def _join_list(values):
        if not isinstance(values, list):
            return ""
        return ", ".join(str(item) for item in values if str(item).strip())

    @staticmethod
    def _split_list(text):
        return [item.strip() for item in str(text or "").replace("，", ",").split(",") if item.strip()]

    def load_config_controls(self):
        cfg = work_memory_config()
        self.interval_spin.setValue(int(cfg.get("auto_capture_interval_seconds", 300) or 300))
        self.quality_spin.setValue(int(cfg.get("screenshot_quality", 90) or 90))
        index = self.scope_combo.findData(str(cfg.get("capture_scope", "all_screens")))
        self.scope_combo.setCurrentIndex(index if index >= 0 else 0)
        self.ocr_check.setChecked(bool(cfg.get("auto_ocr", False)))
        self.embedding_check.setChecked(bool(cfg.get("embedding_enabled", False)))
        self.ai_check.setChecked(bool(cfg.get("ai_enabled", False)))
        self.codex_check.setChecked(bool(cfg.get("codex_collaboration_enabled", False)))
        self.exclude_apps_edit.setText(self._join_list(cfg.get("exclude_apps", [])))
        self.exclude_titles_edit.setText(self._join_list(cfg.get("exclude_window_keywords", [])))
        self.exclude_paths_edit.setText(self._join_list(cfg.get("exclude_paths", [])))

    def save_config_controls(self):
        update_work_memory_config(
            {
                "auto_capture_interval_seconds": int(self.interval_spin.value()),
                "screenshot_quality": int(self.quality_spin.value()),
                "capture_scope": str(self.scope_combo.currentData() or "all_screens"),
                "auto_ocr": bool(self.ocr_check.isChecked()),
                "embedding_enabled": bool(self.embedding_check.isChecked()),
                "ai_enabled": bool(self.ai_check.isChecked()),
                "codex_collaboration_enabled": bool(self.codex_check.isChecked()),
                "exclude_apps": self._split_list(self.exclude_apps_edit.text()),
                "exclude_window_keywords": self._split_list(
                    self.exclude_titles_edit.text()
                ),
                "exclude_paths": self._split_list(self.exclude_paths_edit.text()),
            }
        )
        self.manager.sync_from_config()
        self.refresh_status()
        self.set_feedback("配置已保存")

    def refresh_list(self):
        query = self.search_edit.text().strip()
        source = str(self.source_filter.currentData() or "")
        content_type = str(self.type_filter.currentData() or "")
        favorite = self.favorite_filter.currentData()
        app_text = self.app_filter.text().strip()
        tag_text = self.tag_filter.text().strip()
        daily_state = self.daily_filter.currentData()
        knowledge_state = self.knowledge_filter.currentData()
        current_id = self.current_entry_id()
        self.timeline_list.clear()

        self._current_results = self.manager.search(
            query,
            source=source,
            content_type=content_type,
            app=app_text,
            tag=tag_text,
            favorite=favorite,
            included_in_daily=daily_state,
            knowledge_generated=knowledge_state,
            limit=300,
        )

        selected_row = 0
        for row, entry in enumerate(self._current_results):
            source_label = SOURCE_LABELS.get(entry.get("source", ""), entry.get("source", ""))
            type_label = TYPE_LABELS.get(
                entry.get("content_type", ""), entry.get("content_type", "")
            )
            reason = str(entry.get("match_reason", "")).strip()
            prefix = "★ " if entry.get("favorite") else ""
            sensitive = "  [敏感]" if entry.get("sensitive") else ""
            text = (
                f"{prefix}{entry.get('title', '工作记忆')}{sensitive}\n"
                f"{self._format_time(entry.get('created_at', 0))} · {source_label} · {type_label}"
            )
            if reason:
                text += f" · {reason}"
            item = QListWidgetItem(text)
            item.setData(Qt.ItemDataRole.UserRole, entry.get("id"))
            if entry.get("sensitive"):
                item.setForeground(QColor(255, 165, 130))
            elif entry.get("favorite"):
                item.setForeground(QColor(255, 214, 102))
            elif entry.get("content_type") in {"image", "screenshot", "ocr_text"}:
                item.setForeground(QColor(140, 210, 255))
            self.timeline_list.addItem(item)
            if current_id and entry.get("id") == current_id:
                selected_row = row

        if self.timeline_list.count() > 0:
            self.timeline_list.setCurrentRow(selected_row)
        else:
            self.preview_image.clear()
            self.preview_image.setText("没有匹配的工作记忆")
            self.detail_label.setText("")
            self.text_preview.clear()

    def current_entry_id(self):
        item = self.timeline_list.currentItem()
        if item is None:
            return ""
        return str(item.data(Qt.ItemDataRole.UserRole) or "")

    def current_entry(self):
        entry_id = self.current_entry_id()
        if not entry_id:
            return None
        return self.manager.get_entry(entry_id)

    def visible_entry_ids(self):
        return [str(entry.get("id", "")) for entry in self._current_results if entry.get("id")]

    def show_entry_preview(self, item):
        if item is None:
            return
        entry = self.manager.get_entry(str(item.data(Qt.ItemDataRole.UserRole) or ""))
        if not entry:
            self.preview_image.clear()
            self.preview_image.setText("条目已失效")
            self.detail_label.setText("")
            self.text_preview.clear()
            return

        image_path = str(entry.get("thumbnail_path") or entry.get("image_path") or "").strip()
        pixmap = QPixmap(image_path) if image_path else QPixmap()
        if pixmap.isNull():
            self.preview_image.clear()
            self.preview_image.setText("无图片预览")
        else:
            scaled = pixmap.scaled(
                self.preview_image.size(),
                Qt.AspectRatioMode.KeepAspectRatio,
                Qt.TransformationMode.SmoothTransformation,
            )
            self.preview_image.setPixmap(scaled)

        tags = ", ".join(entry.get("tags", []))
        risk = ", ".join(entry.get("risk_flags", [])) or "无"
        metadata = entry.get("metadata", {}) if isinstance(entry.get("metadata"), dict) else {}
        detail_lines = [
            f"时间: {self._format_time(entry.get('created_at', 0))}",
            f"来源: {SOURCE_LABELS.get(entry.get('source', ''), entry.get('source', ''))}",
            f"类型: {TYPE_LABELS.get(entry.get('content_type', ''), entry.get('content_type', ''))}",
            f"窗口: {entry.get('window_title') or '无'}",
            f"应用: {entry.get('process_name') or entry.get('app_name') or '无'}",
            f"标签: {tags or '无'}",
            f"敏感: {'是' if entry.get('sensitive') else '否'} · 风险: {risk}",
            f"AI: {'允许' if entry.get('ai_allowed') else '不允许'} · 向量化: {'允许' if entry.get('vector_allowed') else '不允许'}",
            f"OCR: {metadata.get('ocr_status', '无')}",
            f"文件: {entry.get('file_path') or entry.get('image_path') or '无'}",
        ]
        self.detail_label.setText("\n".join(detail_lines))

        text_parts = [
            entry.get("summary", ""),
            entry.get("text", ""),
            entry.get("ocr_text", ""),
            f"匹配原因: {entry.get('match_reason', '')}" if entry.get("match_reason") else "",
            f"关联证据: {', '.join(entry.get('relations', []))}" if entry.get("relations") else "",
        ]
        self.text_preview.setPlainText("\n\n".join(part for part in text_parts if str(part).strip()))
        self.feedback_label.setText("")

    def resizeEvent(self, event):
        super().resizeEvent(event)
        if hasattr(self, "timeline_list") and hasattr(self, "preview_image"):
            self.show_entry_preview(self.timeline_list.currentItem())

    def set_feedback(self, text):
        self.feedback_label.setText(str(text or ""))

    def toggle_memory(self):
        cfg = work_memory_config()
        self.manager.set_enabled(not bool(cfg.get("enabled", True)))
        self.refresh_status()

    def toggle_time_machine(self):
        cfg = work_memory_config()
        self.manager.set_time_machine_enabled(not bool(cfg.get("time_machine_enabled", False)))
        self.refresh_status()

    def toggle_privacy_mode(self):
        cfg = work_memory_config()
        self.manager.set_privacy_mode(not bool(cfg.get("privacy_mode", False)))
        self.refresh_status()

    def capture_now(self):
        entry = self.manager.capture_current_screen(source="time_machine", manual=True)
        self.refresh_all()
        self.set_feedback("已补记当前屏幕" if entry else "未补记: " + self.manager.status()["pause_reason"])

    def move_selection(self, delta):
        count = self.timeline_list.count()
        if count <= 0:
            return
        row = self.timeline_list.currentRow()
        self.timeline_list.setCurrentRow(max(0, min(count - 1, row + delta)))

    def jump_to_time(self):
        target = self.jump_time.dateTime().toSecsSinceEpoch()
        best_row = 0
        best_delta = None
        for row, entry in enumerate(self._current_results):
            delta = abs(float(entry.get("created_at", 0) or 0) - target)
            if best_delta is None or delta < best_delta:
                best_delta = delta
                best_row = row
        if self.timeline_list.count() > 0:
            self.timeline_list.setCurrentRow(best_row)

    def add_note(self):
        text = self.note_edit.text().strip()
        if not text:
            return
        entry = self.manager.add_manual_note(text)
        self.note_edit.clear()
        self.refresh_all()
        self.set_feedback("已添加手动笔记" if entry else "添加笔记失败")

    def copy_current_text(self):
        entry = self.current_entry()
        if not entry:
            return
        text = entry.get("text") or entry.get("ocr_text") or entry.get("summary") or entry.get("title")
        QApplication.clipboard().setText(str(text))
        self.set_feedback("已复制")

    def _evidence_path(self, entry):
        if not entry:
            return ""
        for key in ["file_path", "image_path", "thumbnail_path"]:
            path = str(entry.get(key, "")).strip()
            if path and os.path.exists(path):
                return path
        return ""

    def open_current_evidence(self):
        entry = self.current_entry()
        path = self._evidence_path(entry)
        if path and open_path(path):
            self.set_feedback("已打开证据")
        else:
            self.set_feedback("没有可打开的本地证据")

    def open_current_folder(self):
        entry = self.current_entry()
        path = self._evidence_path(entry)
        if path and open_parent(path):
            self.set_feedback("已打开目录")
        else:
            self.set_feedback("没有可打开的目录")

    def toggle_current_favorite(self):
        entry_id = self.current_entry_id()
        if not entry_id:
            return
        pinned = self.manager.toggle_favorite(entry_id)
        self.refresh_list()
        self.set_feedback("已收藏" if pinned else "已取消收藏")

    def toggle_current_sensitive(self):
        entry_id = self.current_entry_id()
        if not entry_id:
            return
        sensitive = self.manager.toggle_sensitive(entry_id)
        self.refresh_list()
        self.set_feedback("已标记敏感，AI/向量化默认禁用" if sensitive else "已取消敏感标记")

    def ocr_current(self):
        entry_id = self.current_entry_id()
        if not entry_id:
            return
        ok, message = self.manager.perform_ocr(entry_id)
        self.refresh_all()
        self.set_feedback("OCR 完成" if ok else f"OCR 不可用: {message}")

    def _selected_or_visible_ids(self):
        current = self.current_entry_id()
        return [current] if current else self.visible_entry_ids()

    def generate_daily_report(self):
        ids = self.visible_entry_ids()
        entry = self.manager.generate_daily_report(ids)
        self.refresh_all()
        self.set_feedback("已生成日报草稿" if entry else "没有可生成日报的条目")

    def generate_knowledge_draft(self):
        entry = self.manager.generate_knowledge_draft(self._selected_or_visible_ids())
        self.refresh_all()
        self.set_feedback("已生成知识草稿" if entry else "没有可生成知识草稿的条目")

    def generate_retro_draft(self):
        entry = self.manager.generate_retro_draft(self._selected_or_visible_ids())
        self.refresh_all()
        self.set_feedback("已生成问题复盘草稿" if entry else "没有可生成复盘的条目")

    def generate_experience_report(self):
        cfg = work_memory_config()
        days = int(cfg.get("experience_discovery_period_days", 7) or 7)
        entry = self.manager.generate_experience_report(days)
        self.refresh_all()
        self.set_feedback("已生成经验发现报告" if entry else "没有可分析的工作记忆")

    def generate_task_package(self):
        entry = self.manager.generate_external_agent_task(self._selected_or_visible_ids())
        self.refresh_all()
        self.set_feedback("已生成外部代理任务包" if entry else "没有可生成任务包的条目")

    def generate_asset_suggestion(self, asset_type):
        entry = self.manager.generate_asset_suggestion(
            self._selected_or_visible_ids(), asset_type
        )
        self.refresh_all()
        labels = {
            "skill": "Skill 建议草稿",
            "workflow": "工作流建议草稿",
            "checklist": "检查清单草稿",
            "prompt": "提示词模板草稿",
        }
        self.set_feedback(
            f"已生成{labels.get(asset_type, '能力资产草稿')}" if entry else "没有可生成建议的条目"
        )

    def import_material(self):
        paths, _selected_filter = QFileDialog.getOpenFileNames(
            self,
            "导入材料",
            os.path.expanduser("~"),
            "材料文件 (*.md *.txt *.json *.log *.py *.yaml *.yml *.toml *.sql *.png *.jpg *.jpeg *.bmp *.webp);;所有文件 (*.*)",
        )
        if not paths:
            return
        count = 0
        for path in paths:
            if self.manager.import_file(path):
                count += 1
        self.refresh_all()
        self.set_feedback(f"已导入 {count} 个材料")

    def export_visible(self):
        ids = self.visible_entry_ids()
        if not ids:
            return
        reply = QMessageBox.question(
            self,
            "导出确认",
            f"导出当前可见的 {len(ids)} 条工作记忆？敏感条目默认不会导出。",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return
        path = self.manager.export_package(entry_ids=ids)
        self.set_feedback(f"已导出: {path}")

    def clear_unfavorited(self):
        reply = QMessageBox.question(
            self,
            "清理确认",
            "清理所有未收藏工作记忆？该操作会删除工作记忆自有截图副本。",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return
        count = self.manager.clear_unfavorited()
        self.refresh_all()
        self.set_feedback(f"已清理 {count} 条未收藏记录")

    def clear_before_jump_time(self):
        cutoff = self.jump_time.dateTime().toSecsSinceEpoch()
        reply = QMessageBox.question(
            self,
            "清理确认",
            f"清理 {datetime.fromtimestamp(cutoff).strftime('%Y-%m-%d %H:%M')} 之前的未收藏工作记忆？",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return
        count = self.manager.clear_unfavorited(before_ts=float(cutoff))
        self.refresh_all()
        self.set_feedback(f"已清理 {count} 条时间范围记录")

    def delete_current(self):
        entry_id = self.current_entry_id()
        if not entry_id:
            return
        reply = QMessageBox.question(
            self,
            "删除确认",
            "删除当前工作记忆条目？",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return
        ok = self.manager.delete_entry(entry_id)
        self.refresh_all()
        self.set_feedback("已删除" if ok else "删除失败")

    def closeEvent(self, event):
        self.hide()
        event.ignore()


if __name__ == "__main__":
    app = QApplication([])
    update_work_memory_config({"enabled": True})
    window = WorkMemoryWindow()
    window.show()
    app.exec()
