"""Xuất OpenAPI spec ra file YAML (hoặc JSON).

Cách dùng (chạy ở thư mục gốc project):
    python scripts/export_openapi.py                  # -> openapi.yaml
    python scripts/export_openapi.py -o docs/api.yaml
    python scripts/export_openapi.py --json           # -> openapi.json
"""
import argparse
import json
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

# Chỉ để import app, không kết nối DB thật
os.environ.setdefault("DATABASE_URL", "sqlite:///:memory:")
os.environ.setdefault("SECRET_KEY", "export-openapi-only")

from app.main import app


def main():
    parser = argparse.ArgumentParser(description="Export OpenAPI spec")
    parser.add_argument("-o", "--output", help="Đường dẫn file đầu ra")
    parser.add_argument("--json", action="store_true", help="Xuất JSON thay vì YAML")
    args = parser.parse_args()

    spec = app.openapi()
    out = Path(args.output or ("openapi.json" if args.json else "openapi.yaml"))
    out.parent.mkdir(parents=True, exist_ok=True)

    if args.json:
        out.write_text(json.dumps(spec, indent=2, ensure_ascii=False), encoding="utf-8")
    else:
        try:
            import yaml
        except ImportError:
            sys.exit("Thiếu PyYAML. Chạy: pip install -r requirements-dev.txt")
        out.write_text(
            yaml.safe_dump(spec, sort_keys=False, allow_unicode=True), encoding="utf-8"
        )

    print(f"Đã xuất: {out}")


if __name__ == "__main__":
    main()
