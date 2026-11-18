package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithLegacyRuntimeResolvesFirmwareALSAHelpers(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	loader := filepath.Join(root, "lib", "ld-linux-armhf.so.3")
	if err := os.WriteFile(loader, []byte("loader"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin", "shairport-sync-ap2")
	vendor := "/system/vendor/quasar/libs"
	got, args := withLegacyRuntime(root, bin, []string{"-v"}, vendor)
	wantPath := filepath.Join(root, "lib") + string(os.PathListSeparator) + vendor
	if got != loader || len(args) != 4 || args[0] != "--library-path" || args[1] != wantPath || args[2] != bin || args[3] != "-v" {
		t.Fatalf("receiver invocation: %q %q", got, args)
	}
	// --library-path overrides LD_LIBRARY_PATH, so the explicit loader list
	// must include the same firmware directory as the receiver environment.
	env := runtimeEnv([]string{"LD_LIBRARY_PATH=/old", "LD_PRELOAD=/old.so"}, wantPath, filepath.Join(vendor, "libasound.so.2"))
	foundPath, foundPreload := false, false
	for _, value := range env {
		switch value {
		case "LD_LIBRARY_PATH=" + wantPath:
			foundPath = true
		case "LD_PRELOAD=" + filepath.Join(vendor, "libasound.so.2"):
			foundPreload = true
		case "LD_LIBRARY_PATH=/old", "LD_PRELOAD=/old.so":
			t.Fatalf("inherited loader setting retained: %s", value)
		}
	}
	if !foundPath || !foundPreload {
		t.Fatalf("receiver environment: %q", env)
	}
	// System tools must never be switched to the package's libc.
	got, args = withLegacyRuntime(root, "/usr/bin/aplay", []string{"--version"}, vendor)
	if got != "/usr/bin/aplay" || len(args) != 1 || args[0] != "--version" {
		t.Fatalf("firmware tool was wrapped: %q %q", got, args)
	}
}

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
