#!/usr/bin/env python3
"""
DKST Dictionary Editor (PyQt6)

Edits these files:
  dictionary_en.txt, dictionary_ko.txt, dictionary_es.txt,
  dictionary_pt.txt, dictionary_fr.txt

File location:
  - Linux/Windows: same folder as this script
  - macOS: ~/Documents/DKST LLM Chat/

Text format per line:
  key, replacement

Rules:
  - key is case-sensitive
  - on save: remove blank rows
  - on save: remove duplicate keys, keeping the first occurrence and removing later ones
  - search box filters in real-time
"""

import os
import sys
import signal
from dataclasses import dataclass

from PyQt6.QtCore import Qt, QSortFilterProxyModel, QRegularExpression
from PyQt6.QtGui import QAction, QKeySequence, QStandardItem, QStandardItemModel
from PyQt6.QtWidgets import (
    QApplication,
    QMainWindow,
    QWidget,
    QHBoxLayout,
    QVBoxLayout,
    QLabel,
    QLineEdit,
    QComboBox,
    QPushButton,
    QMessageBox,
    QTableView,
    QAbstractItemView,
)

@dataclass(frozen=True)
class DictOption:
    code: str
    label: str
    filename: str

DICT_OPTIONS = [
    DictOption("EN", "EN (English)", "dictionary_en.txt"),
    DictOption("KO", "KO (한국어)", "dictionary_ko.txt"),
    DictOption("ES", "ES (Español)", "dictionary_es.txt"),
    DictOption("PT", "PT (Português)", "dictionary_pt.txt"),
    DictOption("FR", "FR (Français)", "dictionary_fr.txt"),
]


def is_macos() -> bool:
    return sys.platform == "darwin"


def get_base_dir() -> str:
    if is_macos():
        return os.path.expanduser("~/Documents/DKST LLM Chat")
    return os.path.abspath(os.path.dirname(__file__))


def ensure_dir(path: str) -> None:
    os.makedirs(path, exist_ok=True)


def parse_line(line: str):
    """
    Parse 'key, replacement' splitting only at the first comma.
    Returns (key, value) or None if invalid/blank.
    """
    raw = line.strip()
    if not raw:
        return None
    if raw.startswith("#"):
        return None

    if "," not in raw:
        return None

    key, val = raw.split(",", 1)
    key = key.strip()
    val = val.strip()

    if not key and not val:
        return None
    return key, val


class ContainsFilterProxy(QSortFilterProxyModel):
    def __init__(self, parent=None):
        super().__init__(parent)
        self._pattern = ""

    def set_filter_text(self, text: str):
        self._pattern = (text or "").strip()
        self.invalidateFilter()

    def filterAcceptsRow(self, source_row: int, source_parent) -> bool:
        if not self._pattern:
            return True

        model = self.sourceModel()
        pat = self._pattern.lower()

        for col in range(model.columnCount()):
            idx = model.index(source_row, col, source_parent)
            data = model.data(idx, Qt.ItemDataRole.DisplayRole)
            if data and pat in str(data).lower():
                return True
        return False


class DictionaryEditor(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("DKST Dictionary Editor")
        self.resize(900, 600)

        self.base_dir = get_base_dir()
        ensure_dir(self.base_dir)

        self.current_option: DictOption | None = None
        self.current_path: str | None = None
        self.dirty = False

        root = QWidget()
        self.setCentralWidget(root)

        outer = QVBoxLayout(root)
        outer.setContentsMargins(12, 12, 12, 12)
        outer.setSpacing(10)

        top_bar = QHBoxLayout()
        top_bar.setSpacing(10)

        self.search = QLineEdit()
        self.search.setPlaceholderText("Search...")
        self.search.textChanged.connect(self.on_search_changed)
        top_bar.addWidget(QLabel("Search"))
        top_bar.addWidget(self.search, 1)

        self.lang_combo = QComboBox()
        for opt in DICT_OPTIONS:
            self.lang_combo.addItem(opt.label, opt)
        self.lang_combo.currentIndexChanged.connect(self.on_lang_changed)
        top_bar.addWidget(self.lang_combo)

        outer.addLayout(top_bar)

        self.model = QStandardItemModel(0, 2)
        self.model.setHorizontalHeaderLabels(["Key", "Replacement"])
        self.model.itemChanged.connect(self.on_item_changed)

        self.proxy = ContainsFilterProxy(self)
        self.proxy.setSourceModel(self.model)

        self.table = QTableView()
        self.table.setModel(self.proxy)
        self.table.setSelectionBehavior(QAbstractItemView.SelectionBehavior.SelectRows)
        self.table.setSelectionMode(QAbstractItemView.SelectionMode.SingleSelection)
        self.table.setEditTriggers(
            QAbstractItemView.EditTrigger.DoubleClicked
            | QAbstractItemView.EditTrigger.SelectedClicked
            | QAbstractItemView.EditTrigger.EditKeyPressed
        )
        self.table.horizontalHeader().setStretchLastSection(True)
        self.table.verticalHeader().setVisible(False)
        outer.addWidget(self.table, 1)

        btn_bar = QHBoxLayout()
        btn_bar.setSpacing(10)

        self.btn_add = QPushButton("Add")
        self.btn_add.clicked.connect(self.add_row)
        btn_bar.addWidget(self.btn_add)

        self.btn_delete = QPushButton("Delete")
        self.btn_delete.clicked.connect(self.delete_selected)
        btn_bar.addWidget(self.btn_delete)

        btn_bar.addStretch(1)

        self.btn_reload = QPushButton("Reload")
        self.btn_reload.clicked.connect(self.reload_current)
        btn_bar.addWidget(self.btn_reload)

        self.btn_save = QPushButton("Save")
        self.btn_save.clicked.connect(self.save_current)
        btn_bar.addWidget(self.btn_save)

        outer.addLayout(btn_bar)

        self._setup_shortcuts()
        self.load_from_combo_initial()

    def _setup_shortcuts(self):
        save_action = QAction(self)
        save_action.setShortcut(QKeySequence.StandardKey.Save)
        save_action.triggered.connect(self.save_current)
        self.addAction(save_action)

        find_action = QAction(self)
        find_action.setShortcut(QKeySequence.StandardKey.Find)
        find_action.triggered.connect(lambda: self.search.setFocus())
        self.addAction(find_action)

    def on_item_changed(self, _item):
        self.dirty = True

    def on_search_changed(self, text: str):
        self.proxy.set_filter_text(text)

    def load_from_combo_initial(self):
        opt = self.lang_combo.currentData()
        if isinstance(opt, DictOption):
            self.set_current_option(opt)
            self.load_file(opt)

    def on_lang_changed(self, _index: int):
        opt = self.lang_combo.currentData()
        if not isinstance(opt, DictOption):
            return

        if self.dirty:
            res = QMessageBox.question(
                self,
                "Unsaved Changes",
                "You have unsaved changes. Save before switching?",
                QMessageBox.StandardButton.Yes
                | QMessageBox.StandardButton.No
                | QMessageBox.StandardButton.Cancel,
            )
            if res == QMessageBox.StandardButton.Cancel:
                self.lang_combo.blockSignals(True)
                try:
                    self._restore_combo_to_current()
                finally:
                    self.lang_combo.blockSignals(False)
                return
            if res == QMessageBox.StandardButton.Yes:
                if not self.save_current():
                    self.lang_combo.blockSignals(True)
                    try:
                        self._restore_combo_to_current()
                    finally:
                        self.lang_combo.blockSignals(False)
                    return

        self.set_current_option(opt)
        self.load_file(opt)

    def _restore_combo_to_current(self):
        if not self.current_option:
            return
        for i in range(self.lang_combo.count()):
            data = self.lang_combo.itemData(i)
            if isinstance(data, DictOption) and data.code == self.current_option.code:
                self.lang_combo.setCurrentIndex(i)
                return

    def set_current_option(self, opt: DictOption):
        self.current_option = opt
        self.current_path = os.path.join(self.base_dir, opt.filename)

    def clear_model(self):
        self.model.blockSignals(True)
        try:
            self.model.setRowCount(0)
        finally:
            self.model.blockSignals(False)

    def load_file(self, opt: DictOption):
        self.clear_model()

        path = os.path.join(self.base_dir, opt.filename)
        self.current_path = path

        if not os.path.exists(path):
            with open(path, "w", encoding="utf-8") as f:
                f.write("")
            self.dirty = False
            self.setWindowTitle(f"DKST Dictionary Editor - {opt.label}")
            return

        rows = []
        try:
            with open(path, "r", encoding="utf-8") as f:
                for line in f:
                    parsed = parse_line(line)
                    if not parsed:
                        continue
                    rows.append(parsed)
        except Exception as e:
            QMessageBox.critical(self, "Load Failed", f"Could not load file:\n{path}\n\n{e}")
            self.dirty = False
            return

        self.model.blockSignals(True)
        try:
            for key, val in rows:
                self._append_row_items(key, val)
        finally:
            self.model.blockSignals(False)

        self.dirty = False
        self.search.clear()
        self.setWindowTitle(f"DKST Dictionary Editor - {opt.label}")

    def _append_row_items(self, key: str, val: str):
        it_key = QStandardItem(key)
        it_val = QStandardItem(val)
        it_key.setEditable(True)
        it_val.setEditable(True)
        self.model.appendRow([it_key, it_val])

    def add_row(self):
        self._append_row_items("", "")
        self.dirty = True

        src_row = self.model.rowCount() - 1
        proxy_row = self.proxy.mapFromSource(self.model.index(src_row, 0)).row()
        if proxy_row >= 0:
            self.table.selectRow(proxy_row)
            self.table.setCurrentIndex(self.proxy.index(proxy_row, 0))
            self.table.edit(self.proxy.index(proxy_row, 0))

    def delete_selected(self):
        idx = self.table.currentIndex()
        if not idx.isValid():
            return

        src_idx = self.proxy.mapToSource(idx)
        if not src_idx.isValid():
            return

        self.model.removeRow(src_idx.row())
        self.dirty = True

    def reload_current(self):
        if not self.current_option:
            return

        if self.dirty:
            res = QMessageBox.question(
                self,
                "Unsaved Changes",
                "Reloading will discard unsaved changes. Continue?",
                QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            )
            if res != QMessageBox.StandardButton.Yes:
                return

        self.load_file(self.current_option)

    def _collect_rows_cleaned(self):
        """
        Returns list of (key, val) after:
          - removing blank rows
          - removing duplicates by key (case-sensitive), keeping first occurrence
        """
        seen = set()
        cleaned = []

        for r in range(self.model.rowCount()):
            key = (self.model.item(r, 0).text() if self.model.item(r, 0) else "").strip()
            val = (self.model.item(r, 1).text() if self.model.item(r, 1) else "").strip()

            if not key and not val:
                continue

            if not key:
                continue

            if key in seen:
                continue

            seen.add(key)
            cleaned.append((key, val))

        return cleaned

    def save_current(self) -> bool:
        if not self.current_path or not self.current_option:
            return False

        cleaned = self._collect_rows_cleaned()

        try:
            with open(self.current_path, "w", encoding="utf-8", newline="\n") as f:
                for key, val in cleaned:
                    f.write(f"{key}, {val}\n")
        except Exception as e:
            QMessageBox.critical(self, "Save Failed", f"Could not save file:\n{self.current_path}\n\n{e}")
            return False

        self._reload_from_cleaned(cleaned)
        self.dirty = False
        QMessageBox.information(self, "Saved", "Saved successfully.")
        return True

    def _reload_from_cleaned(self, cleaned):
        self.clear_model()
        self.model.blockSignals(True)
        try:
            for key, val in cleaned:
                self._append_row_items(key, val)
        finally:
            self.model.blockSignals(False)

    def closeEvent(self, event):
        if not self.dirty:
            event.accept()
            return

        res = QMessageBox.question(
            self,
            "Unsaved Changes",
            "You have unsaved changes. Save before quitting?",
            QMessageBox.StandardButton.Yes
            | QMessageBox.StandardButton.No
            | QMessageBox.StandardButton.Cancel,
        )
        if res == QMessageBox.StandardButton.Cancel:
            event.ignore()
            return
        if res == QMessageBox.StandardButton.Yes:
            if not self.save_current():
                event.ignore()
                return
        event.accept()


def main():
    signal.signal(signal.SIGINT, signal.SIG_DFL)
    app = QApplication(sys.argv)
    win = DictionaryEditor()
    win.show()
    sys.exit(app.exec())


if __name__ == "__main__":
    main()