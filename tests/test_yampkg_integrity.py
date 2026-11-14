#!/usr/bin/env python3
"""Release regressions: exact payload coverage, modes and Mini-only libc."""
import hashlib
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("package_yampkg", ROOT / "scripts/package_yampkg.py")
pkg = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pkg)
PREFIX = str(pkg.PLUGIN_REL)[len("files/"):] + "/"


class IntegrityTests(unittest.TestCase):
    def check(self, entries, hashes=None):
        buffer = io.BytesIO()
        actual = {}
        with tarfile.open(fileobj=buffer, mode="w") as archive:
            for name, mode, data in entries:
                member = tarfile.TarInfo("files/" + name)
                member.mode, member.size = mode, len(data)
                archive.addfile(member, io.BytesIO(data))
                actual[name] = hashlib.sha256(data).hexdigest()
        buffer.seek(0)
        with tarfile.open(fileobj=buffer) as archive:
            pkg.verify_archive_payload(archive, {"files_sha256": actual if hashes is None else hashes})

    def test_valid_modes_and_complete_hashes(self):
        self.check([(PREFIX + "bin/station-airplay", 0o755, b"elf"),
                    (PREFIX + "launch-airplay.sh", 0o755, b"#!/bin/sh"),
                    (PREFIX + "lib/ld-linux-armhf.so.3", 0o755, b"ld"),
                    (PREFIX + "lib/libc.so.6", 0o644, b"libc")])

    def test_unlisted_library_is_rejected(self):
        with self.assertRaisesRegex(SystemExit, "unlisted"):
            self.check([(PREFIX + "lib/libc.so.6", 0o644, b"libc")], {})

    def test_missing_payload_is_rejected(self):
        with self.assertRaisesRegex(SystemExit, "missing"):
            self.check([], {"missing": "abc"})

    def test_bad_hash_is_rejected(self):
        with self.assertRaisesRegex(SystemExit, "hash mismatch"):
            self.check([("config", 0o644, b"abc")], {"config": "wrong"})

    def test_nonexecutable_launcher_binary_loader_and_hook_are_rejected(self):
        for name in ["bin/airplay-postinstall", PREFIX + "bin/station-airplay",
                     PREFIX + "bin/dbus-daemon", PREFIX + "launch-airplay.sh",
                     PREFIX + "lib/ld-linux-armhf.so.3"]:
            with self.subTest(name=name), self.assertRaisesRegex(SystemExit, "executable mode"):
                self.check([(name, 0o644, b"test")])

    def test_duplicate_payload_is_rejected(self):
        with self.assertRaisesRegex(SystemExit, "duplicate"):
            self.check([("config", 0o644, b"one"), ("config", 0o644, b"two")])

    def test_only_mini_keeps_verified_base_libc(self):
        for model in ("yandexmini", "yandexmini_2", "yandexplum", "yandexstation_2", "mango"):
            with self.subTest(model=model), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                plugin = root / pkg.PLUGIN_REL
                plugin.mkdir(parents=True)
                manifest = {"files_sha256": {}}
                for name in pkg.PRUNE_RUNTIME:
                    path = plugin / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(b"base:" + name.encode())
                    path.chmod(0o755)
                    manifest["files_sha256"][pkg.runtime_key(name)] = pkg.sha256_file(path)
                # Real rebuilt binaries are deliberately not needed by this policy test.
                with patch.object(pkg, "REPLACE_RUNTIME", ()):
                    pkg.reconcile_runtime(plugin, manifest, {"target": model})
                for name in pkg.PRUNE_RUNTIME:
                    keep = model == "yandexmini" and name in pkg.LEGACY_RUNTIME
                    self.assertEqual((plugin / name).exists(), keep, name)
                    self.assertEqual(pkg.runtime_key(name) in manifest["files_sha256"], keep, name)
                if model == "yandexmini":
                    self.assertEqual((plugin / "lib/ld-linux-armhf.so.3").stat().st_mode & 0o777, 0o755)

    def test_mini_without_verified_legacy_files_fails_closed(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(pkg, "REPLACE_RUNTIME", ()):
            with self.assertRaisesRegex(SystemExit, "unverified Mini"):
                pkg.reconcile_runtime(Path(temp), {"files_sha256": {}}, {"target": "yandexmini"})

    def test_extra_binary_does_not_inherit_nonexecutable_checkout_mode(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "source"
            source.write_bytes(b"ELF")
            source.chmod(0o664)
            binary = root / "bin/shairport-sync-ap2-aarch64"
            library = root / "lib/aarch64/libplist-2.0.so.3"
            pkg.copy_payload_file(source, binary, executable=True)
            pkg.copy_payload_file(source, library)
            self.assertEqual(binary.stat().st_mode & 0o777, 0o755)
            self.assertEqual(library.stat().st_mode & 0o777, 0o644)
            self.assertEqual(binary.read_bytes(), source.read_bytes())


if __name__ == "__main__":
    unittest.main()
