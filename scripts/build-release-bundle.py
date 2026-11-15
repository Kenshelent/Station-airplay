#!/usr/bin/env python3
"""Pack receiver, command helper and web panel into ONE release .yampkg per model.

Release tool: it does not publish, contact the network, install anything or
change permissions. Publishing stays a separate, explicit `yam-publish release`
step (see docs/RELEASE-2.1.8.md).

The receiver runtime comes from the already verified 2.1.8 base packages; this
tool only rewrites descriptor versions, the Luna dependency and the manifest,
and embeds the helper and panel payloads. Every output archive is read back and
its hashes, modes and plugin descriptors are asserted.
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

PLUGIN = "share/luna/plugins/"
RECEIVER = PLUGIN + "airplay.airplay-luna/"
HELPER = PLUGIN + "airplay.panel-command/"
PANEL = PLUGIN + "airplay.panel/"
SPLIT = ["airplay/panel-command", "airplay/panel"]


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
    path.write_bytes(subprocess.check_output(["zstd", "-19", "-q", "-c"], input=raw.getvalue()))


def one(directory, pattern, target):
    matches = sorted(directory.glob(pattern.format(target=target)))
    if len(matches) != 1:
        raise SystemExit(f"{target}: expected exactly one {pattern}, found {len(matches)}")
    return matches[0]


def set_version(text, version, name):
    updated, count = re.subn(r'^version = "[^"]+"$', f'version = "{version}"', text, count=1, flags=re.M)
    if count != 1:
        raise SystemExit(f"{name}: version line is not unique")
    return updated


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-packages", type=Path, required=True,
                        help="directory with the verified airplay-luna and panel-command packages")
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--luna-min", default="0.2.27",
                        help="minimum Luna version that hosts packages with several plugins")
    parser.add_argument("--targets", help="comma-separated subset")
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9A-Za-z][0-9A-Za-z.+-]*", args.luna_min):
        parser.error("invalid Luna version")

    root = Path(__file__).resolve().parents[1]
    version = re.search(r'^version = "([^"]+)"', (root / "plugin.toml").read_text(), re.M)[1]
    if re.search(r"-(test|panel|dev|rc)\.", version):
        parser.error("plugin.toml still carries a pre-release suffix: " + version)
    panel_toml = (root / "panel/web.toml").read_text()
    helper_toml = (root / "panel/command.toml").read_text()
    panel_version = re.search(r'^version = "([^"]+)"', panel_toml, re.M)[1]
    helper_version = re.search(r'^version = "([^"]+)"', helper_toml, re.M)[1]
    for name, value in [("panel/web.toml", panel_version), ("panel/command.toml", helper_version)]:
        if re.search(r"-(test|panel|dev|rc)\.", value):
            parser.error(f"{name} still carries a pre-release suffix: {value}")
    if 'id = "airplay.panel"' not in panel_toml:
        parser.error("panel/web.toml must declare id airplay.panel to match its payload directory")

    matrix = json.loads((root / "release/yampkg/targets.json").read_text())
    targets = [entry["target"] for entry in matrix["targets"]]
    if args.targets:
        requested = [item.strip() for item in args.targets.split(",") if item.strip()]
        unknown = sorted(set(requested) - set(targets))
        if unknown:
            parser.error("unknown targets: " + ", ".join(unknown))
        targets = requested

    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    source_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    records = []
    for target in targets:
        base = one(args.base_packages, "station-airplay-*-{target}.yampkg", target)
        helper = one(args.base_packages, "airplay-panel-command-*-{target}.yampkg", target)
        manifest, payload = read_package(base)
        if manifest["name"] != "airplay/airplay-luna" or manifest["target"] != target:
            raise SystemExit(f"{base.name}: unexpected name/target")
        helper_manifest, helper_payload = read_package(helper)
        if helper_manifest["name"] != "airplay/panel-command" or helper_manifest["target"] != target:
            raise SystemExit(f"{helper.name}: unexpected name/target")
        if not all(name.startswith(HELPER) for name in helper_payload):
            raise SystemExit(f"{helper.name}: unexpected helper payload layout")
        if set(payload) & set(helper_payload):
            raise SystemExit(f"{target}: receiver and helper payloads overlap")
        payload.update(helper_payload)

        text, mode = payload[RECEIVER + "plugin.toml"]
        text = set_version(text.decode(), version, "receiver plugin.toml")
        text = re.sub(r"Требует Luna [0-9][0-9A-Za-z.+-]*\+",
                      f"Требует Luna {args.luna_min}+", text)
        payload[RECEIVER + "plugin.toml"] = (text.encode(), mode)

        text, mode = payload[HELPER + "plugin.toml"]
        payload[HELPER + "plugin.toml"] = (set_version(text.decode(), helper_version, "helper plugin.toml").encode(), mode)

        panel_dir = root / "panel"
        payload[PANEL + "plugin.toml"] = (set_version(panel_toml, panel_version, "panel/web.toml").encode(), 0o644)
        for name in ["main.lua", "index.html", "LUNA-LICENSE"]:
            payload[PANEL + name] = ((panel_dir / name).read_bytes(), 0o644)

        postinstall = root / "release/yampkg/airplay-postinstall.sh"
        if not postinstall.is_file():
            raise SystemExit("missing release/yampkg/airplay-postinstall.sh")
        payload["bin/airplay-postinstall"] = (postinstall.read_bytes(), 0o755)

        manifest = copy.deepcopy(manifest)
        manifest["version"] = version
        manifest["depends"] = ["yam/luna >=" + args.luna_min]
        manifest["conflicts"] = sorted(set(manifest.get("conflicts", []) + SPLIT))
        manifest["replaces"] = sorted(set(manifest.get("replaces", []) + SPLIT))
        manifest["description"] = (
            "AirPlay 2 для Яндекс Станций: приёмник, панель управления и helper команд "
            f"в одном пакете. Требует Luna {args.luna_min}+."
        )
        artifact = out / f"station-airplay-{version}-{target}.yampkg"
        pack(artifact, manifest, payload)

        verified, checked = read_package(artifact)
        if verified["depends"] != ["yam/luna >=" + args.luna_min]:
            raise SystemExit(f"{artifact.name}: dependency mismatch")
        descriptors = sorted(name for name in checked if name.startswith(PLUGIN) and name.endswith("/plugin.toml"))
        expected = sorted([RECEIVER + "plugin.toml", HELPER + "plugin.toml", PANEL + "plugin.toml"])
        if descriptors != expected:
            raise SystemExit(f"{artifact.name}: unexpected plugin descriptors {descriptors}")
        for name in [HELPER + "panel-command", RECEIVER + "bin/station-airplay"]:
            if not checked[name][1] & 0o111:
                raise SystemExit(f"{artifact.name}: {name} lost its executable bit")
        if not checked["bin/airplay-postinstall"][1] & 0o111:
            raise SystemExit(f"{artifact.name}: postinstall lost its executable bit")
        if verified.get("postinstall") != ["bin/airplay-postinstall"]:
            raise SystemExit(f"{artifact.name}: unexpected postinstall manifest")
        if verified["version"] != version or verified["target"] != target:
            raise SystemExit(f"{artifact.name}: manifest version/target mismatch")
        records.append({
            "target": target,
            "file": artifact.name,
            "sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
            "size": artifact.stat().st_size,
            "base": base.name,
            "base_sha256": hashlib.sha256(base.read_bytes()).hexdigest(),
            "helper": helper.name,
            "helper_sha256": hashlib.sha256(helper.read_bytes()).hexdigest(),
            "plugins": descriptors,
        })
    (out / "SHA256SUMS").write_text("".join(
        record["sha256"] + "  " + record["file"] + "\n" for record in records))
    (out / "release-provenance.json").write_text(json.dumps({
        "package": "airplay/airplay-luna",
        "version": version,
        "depends": ["yam/luna >=" + args.luna_min],
        "source_commit": source_commit,
        "helper_version": helper_version,
        "panel_version": panel_version,
        "conflicts": SPLIT,
        "replaces": SPLIT,
        "published": False,
        "artifacts": records,
    }, indent=2) + "\n")
    print(f"Built and verified {len(records)} single-package release artifacts, version {version}")


if __name__ == "__main__":
    main()
