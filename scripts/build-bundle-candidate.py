#!/usr/bin/env python3
"""Pack receiver + command helper + Lua UI into ONE model-specific YAM package.

Private build only: no registry writes, network calls, installs or permission changes.
"""
import argparse
import copy
import hashlib
import io
import json
import re
import subprocess
import tarfile
from pathlib import Path

import yaml

VERSION = "2.1.8-panel.3"
PLUGIN = "share/luna/plugins/"
RECEIVER = PLUGIN + "airplay.airplay-luna/"
HELPER = PLUGIN + "airplay.panel-command/"
PANEL = PLUGIN + "airplay.panel/"


def read_package(path):
    raw = subprocess.check_output(["zstd", "-dc", str(path)])
    with tarfile.open(fileobj=io.BytesIO(raw)) as archive:
        manifest = yaml.safe_load(archive.extractfile("MANIFEST.yaml").read())
        payload = {}
        for member in archive:
            if member.isdir():
                continue
            if not member.isfile() or member.name.startswith("/") or ".." in Path(member.name).parts:
                raise ValueError("unsafe package entry")
            if member.name == "MANIFEST.yaml":
                continue
            if not member.name.startswith("files/"):
                raise ValueError("unexpected package entry")
            relative = member.name[6:]
            if relative in payload:
                raise ValueError("duplicate payload path")
            data = archive.extractfile(member).read()
            assert hashlib.sha256(data).hexdigest() == manifest["files_sha256"][relative]
            payload[relative] = (data, member.mode)
        assert set(payload) == set(manifest["files_sha256"])
        return manifest, payload


def add(payload, name, data, mode=0o644):
    assert name not in payload, name
    payload[name] = (data, mode)


def pack(path, manifest, payload):
    manifest["files_sha256"] = {
        name: hashlib.sha256(data).hexdigest()
        for name, (data, _) in sorted(payload.items())
    }
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode="w") as archive:
        content = {"MANIFEST.yaml": (yaml.safe_dump(manifest, allow_unicode=True, sort_keys=False).encode(), 0o644)}
        content.update({"files/" + name: item for name, item in sorted(payload.items())})
        for name, (data, mode) in content.items():
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.mtime = len(data), mode, 0
            archive.addfile(info, io.BytesIO(data))
    packed = subprocess.check_output(["zstd", "-19", "-q", "-c"], input=raw.getvalue())
    path.write_bytes(packed)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--base-packages", type=Path, required=True)
    p.add_argument("--out", type=Path, required=True)
    p.add_argument("--luna-version", required=True)
    args = p.parse_args()
    if not re.fullmatch(r"[0-9A-Za-z][0-9A-Za-z.+-]*", args.luna_version):
        p.error("invalid Luna version")
    root = Path(__file__).resolve().parents[1]
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    bases = sorted(args.base_packages.glob("station-airplay-2.1.8-panel.2-*.yampkg"))
    assert len(bases) == 12, "expected all 12 model-specific receiver packages"
    records = []
    for base in bases:
        manifest, payload = read_package(base)
        target = manifest["target"]
        assert manifest["name"] == "airplay/airplay-luna"
        helper_manifest, helper = read_package(
            args.base_packages / f"airplay-panel-command-0.1.0-test.1-{target}.yampkg")
        assert helper_manifest["target"] == target
        assert helper_manifest["name"] == "airplay/panel-command"
        assert all(name.startswith(HELPER) for name in helper), "unexpected helper payload"
        assert not (set(payload) & set(helper))
        payload.update(helper)
        data, mode = payload[RECEIVER + "plugin.toml"]
        text, count = re.subn(r'^version = "2.1.8-panel.2"$',
                             f'version = "{VERSION}"', data.decode(), count=1, flags=re.M)
        assert count == 1
        payload[RECEIVER + "plugin.toml"] = (text.encode(), mode)
        ui = """id = "airplay.panel"
name = "AirPlay Panel"
version = "0.1.0-test.3"
entry = "main.lua"
permissions = ["web"]
[launch]
kind = "lua"
mode = "service"
"""
        add(payload, PANEL + "plugin.toml", ui.encode())
        for name in ["main.lua", "index.html", "LUNA-LICENSE"]:
            add(payload, PANEL + name, (root / "panel" / name).read_bytes())
        manifest = copy.deepcopy(manifest)
        manifest["version"] = VERSION
        manifest["depends"] = ["yam/luna =" + args.luna_version]
        # Adopt the former split packages through YAM's normal ownership
        # transfer, never by unlinking their files or editing the database.
        old = ["airplay/panel-command", "airplay/panel"]
        manifest["conflicts"] = sorted(set(manifest.get("conflicts", []) + old))
        manifest["replaces"] = sorted(set(manifest.get("replaces", []) + old))
        manifest["description"] = "AirPlay receiver with bundled colour panel and command helper (private candidate)."
        artifact = out / f"station-airplay-{VERSION}-{target}.yampkg"
        pack(artifact, manifest, payload)
        verified, checked = read_package(artifact)
        assert verified["depends"] == ["yam/luna =" + args.luna_version]
        descriptors = sorted(name for name in checked if name.startswith(PLUGIN) and name.endswith("/plugin.toml"))
        assert descriptors == sorted([
            RECEIVER + "plugin.toml", HELPER + "plugin.toml", PANEL + "plugin.toml",
        ])
        assert checked[HELPER + "panel-command"][1] & 0o111
        assert checked[RECEIVER + "bin/station-airplay"][1] & 0o111
        records.append({
            "target": target, "file": artifact.name, "plugins": descriptors,
            "sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
            "size": artifact.stat().st_size,
        })
    (out / "SHA256SUMS").write_text("".join(
        record["sha256"] + "  " + record["file"] + "\n" for record in records))
    (out / "provenance.json").write_text(json.dumps({
        "private_candidate": True, "published": False, "package": "airplay/airplay-luna",
        "version": VERSION, "depends": ["yam/luna =" + args.luna_version],
        "artifacts": records,
    }, indent=2) + "\n")
    print(f"Built and verified {len(records)} bundles: ONE package per model, all three components inside")


if __name__ == "__main__":
    main()
