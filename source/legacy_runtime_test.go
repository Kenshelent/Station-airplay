package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithLegacyRuntimeUsesBundledLoader(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	loader := filepath.Join(root, "lib", "ld-linux-armhf.so.3")
	if err := os.WriteFile(loader, []byte("loader"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin", "dbus-daemon")
	got, args := withLegacyRuntime(root, bin, []string{"--version"})
	if got != loader {
		t.Fatalf("binary %q, want %q", got, loader)
	}
	want := []string{"--library-path", filepath.Join(root, "lib"), bin, "--version"}
	if len(args) != len(want) {
		t.Fatalf("args %q, want %q", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args %q, want %q", args, want)
		}
	}
	if got, args := withLegacyRuntime(root, "/usr/bin/dbus-daemon", nil); got != "/usr/bin/dbus-daemon" || args != nil {
		t.Fatalf("firmware tool was wrapped: %q %q", got, args)
	}
	if err := os.Remove(loader); err != nil {
		t.Fatal(err)
	}
	if got, args := withLegacyRuntime(root, bin, []string{"--version"}); got != bin || len(args) != 1 || args[0] != "--version" {
		t.Fatalf("package without loader was wrapped: %q %q", got, args)
	}
}
