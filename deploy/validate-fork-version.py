#!/usr/bin/env python3
"""Enforce the user-owned fork version across local build entry points."""
import datetime
import json
from pathlib import Path
import re
import sys

repo = Path(__file__).resolve().parents[1]
version = sys.argv[1] if len(sys.argv) > 1 else (repo / "backend/cmd/server/VERSION").read_text().strip()
match = re.fullmatch(r"v(\d+\.\d+\.\d+)-mr-(\d{6})w(\d{2,})", version)
if not match:
    raise SystemExit("Version must match v<upstream>-mr-<YYMMDD>w<sequence>")
upstream, date, sequence = match.groups()
datetime.datetime.strptime(date, "%y%m%d")
if upstream != (repo / "backend/cmd/server/UPSTREAM_VERSION").read_text().strip():
    raise SystemExit("Fork version does not match UPSTREAM_VERSION")
if version != (repo / "backend/cmd/server/VERSION").read_text().strip():
    raise SystemExit("Build version must match backend/cmd/server/VERSION")
releases = json.loads((repo / "deploy/releases.json").read_text())
current = [release for release in releases if release["version"] == version]
if len(current) != 1 or current[0]["sequence"] != int(sequence) or int(sequence) < 1:
    raise SystemExit("Version must have exactly one matching release sequence")
if current[0]["upstream"] != upstream or current[0]["date"].replace("-", "")[2:] != date:
    raise SystemExit("Release metadata does not match version")
sequences = [release["sequence"] for release in releases if release["upstream"] == upstream]
if len(set(sequences)) != len(sequences) or sorted(sequences) != list(range(1, max(sequences) + 1)):
    raise SystemExit("Release sequences must increase without duplicates or gaps")
print(version)
