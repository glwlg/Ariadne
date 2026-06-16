from __future__ import annotations

import json
import sys
import time
from contextlib import redirect_stdout
from pathlib import Path


def _configure_stdio():
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure:
            try:
                reconfigure(encoding="utf-8", errors="replace")
            except Exception:
                pass


def _emit(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False))
    sys.stdout.write("\n")
    sys.stdout.flush()


def _line_from_row(row):
    text = ""
    confidence = 0.0
    points = []
    if isinstance(row, (list, tuple)):
        if len(row) >= 1:
            points = row[0] or []
        if len(row) >= 2:
            text = str(row[1] or "")
        if len(row) >= 3:
            try:
                confidence = float(row[2] or 0)
            except Exception:
                confidence = 0.0
    elif isinstance(row, dict):
        text = str(row.get("text") or "")
        confidence = float(row.get("score") or row.get("confidence") or 0)
        points = row.get("points") or row.get("box") or []

    xs = []
    ys = []
    for point in points:
        if isinstance(point, (list, tuple)) and len(point) >= 2:
            try:
                xs.append(float(point[0]))
                ys.append(float(point[1]))
            except Exception:
                pass
    rect = {}
    if xs and ys:
        min_x = min(xs)
        min_y = min(ys)
        rect = {
            "x": int(round(min_x)),
            "y": int(round(min_y)),
            "width": int(round(max(xs) - min_x)),
            "height": int(round(max(ys) - min_y)),
        }
    return {
        "text": text.strip(),
        "confidence": confidence,
        "rect": rect,
    }


def main():
    _configure_stdio()
    if len(sys.argv) < 2:
        _emit({"ok": False, "error": "missing image path"})
        return 0

    image_path = Path(sys.argv[1])
    if not image_path.exists():
        _emit({"ok": False, "error": "image file not found"})
        return 0

    started = time.perf_counter()
    try:
        with redirect_stdout(sys.stderr):
            from rapidocr_onnxruntime import RapidOCR

            engine = RapidOCR()
            result, elapsed = engine(str(image_path))
        lines = []
        if isinstance(result, list):
            for row in result:
                line = _line_from_row(row)
                if line["text"]:
                    lines.append(line)
        text = "\n".join(line["text"] for line in lines).strip()
        elapsed_ms = int(round(float(elapsed) * 1000)) if isinstance(elapsed, (int, float)) else 0
        if elapsed_ms <= 0:
            elapsed_ms = int(round((time.perf_counter() - started) * 1000))
        _emit(
            {
                "ok": True,
                "provider": "rapidocr_onnxruntime",
                "text": text,
                "lines": lines,
                "elapsedMs": elapsed_ms,
            }
        )
        return 0
    except Exception as exc:
        _emit(
            {
                "ok": False,
                "provider": "rapidocr_onnxruntime",
                "error": str(exc),
                "elapsedMs": int(round((time.perf_counter() - started) * 1000)),
            }
        )
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
