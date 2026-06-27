package agentstatus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexNpmPlatformDir(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		want   string
		ok     bool
	}{
		{"darwin", "arm64", "codex-darwin-arm64", true},
		{"darwin", "amd64", "codex-darwin-x64", true},
		{"linux", "amd64", "codex-linux-x64", true},
		{"linux", "arm64", "codex-linux-arm64", true},
		{"windows", "amd64", "codex-win32-x64", true},
		{"freebsd", "riscv64", "", false},
	}
	for _, tc := range cases {
		got, ok := codexNpmPlatformDir(tc.goos, tc.goarch)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("codexNpmPlatformDir(%q,%q)=(%q,%v), want (%q,%v)", tc.goos, tc.goarch, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCodexPlatformBinaryPath(t *testing.T) {
	pkg := "/home/u/.npm/lib/node_modules/@openai/codex"
	got, ok := codexPlatformBinaryPath(pkg, "darwin", "arm64")
	want := filepath.Join(pkg, "node_modules", "@openai", "codex-darwin-arm64", "codex")
	if !ok || got != want {
		t.Fatalf("codexPlatformBinaryPath darwin/arm64 = (%q,%v), want (%q,true)", got, ok, want)
	}
	winGot, ok := codexPlatformBinaryPath(pkg, "windows", "amd64")
	winWant := filepath.Join(pkg, "node_modules", "@openai", "codex-win32-x64", "codex.exe")
	if !ok || winGot != winWant {
		t.Fatalf("codexPlatformBinaryPath windows = (%q,%v), want (%q,true)", winGot, ok, winWant)
	}
	if _, ok := codexPlatformBinaryPath(pkg, "plan9", "mips"); ok {
		t.Fatalf("codexPlatformBinaryPath unsupported platform should be ok=false")
	}
}

func TestCodexVendorTargetTriple(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		want   string
		ok     bool
	}{
		{"darwin", "arm64", "aarch64-apple-darwin", true},
		{"darwin", "amd64", "x86_64-apple-darwin", true},
		{"linux", "amd64", "x86_64-unknown-linux-musl", true},
		{"linux", "arm64", "aarch64-unknown-linux-musl", true},
		{"windows", "amd64", "x86_64-pc-windows-msvc", true},
		{"windows", "arm64", "aarch64-pc-windows-msvc", true},
		{"linux", "386", "", false},
		{"freebsd", "riscv64", "", false},
	}
	for _, tc := range cases {
		got, ok := codexVendorTargetTriple(tc.goos, tc.goarch)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("codexVendorTargetTriple(%q,%q)=(%q,%v), want (%q,%v)", tc.goos, tc.goarch, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCodexPlatformVendorBinaryPath(t *testing.T) {
	pkg := "/home/u/.npm/lib/node_modules/@openai/codex"
	got, ok := codexPlatformVendorBinaryPath(pkg, "darwin", "arm64")
	want := filepath.Join(pkg, "node_modules", "@openai", "codex-darwin-arm64", "vendor", "aarch64-apple-darwin", "bin", "codex")
	if !ok || got != want {
		t.Fatalf("codexPlatformVendorBinaryPath darwin/arm64 = (%q,%v), want (%q,true)", got, ok, want)
	}
	winGot, ok := codexPlatformVendorBinaryPath(pkg, "windows", "amd64")
	winWant := filepath.Join(pkg, "node_modules", "@openai", "codex-win32-x64", "vendor", "x86_64-pc-windows-msvc", "bin", "codex.exe")
	if !ok || winGot != winWant {
		t.Fatalf("codexPlatformVendorBinaryPath windows = (%q,%v), want (%q,true)", winGot, ok, winWant)
	}
}

// TestServiceCodexPlatformBinaryCompleteVendorLayout reproduces the field bug:
// codex >= 0.140 ships the native binary under vendor/<triple>/bin/, NOT at the
// subpackage root. The completeness probe must accept the vendor layout so the
// provider is not falsely reported as not_installed / platform_pkg_incomplete.
func TestServiceCodexPlatformBinaryCompleteVendorLayout(t *testing.T) {
	pkg := t.TempDir()
	vendorBin := filepath.Join(pkg, "node_modules", "@openai", "codex-darwin-arm64", "vendor", "aarch64-apple-darwin", "bin", "codex")

	svc := Service{IsExecutableFile: func(p string) bool {
		info, err := os.Stat(p)
		return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
	}}

	// Only the vendor-layout binary exists (no legacy subpackage-root binary).
	if err := os.MkdirAll(filepath.Dir(vendorBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendorBin, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	path, complete := svc.codexPlatformBinaryComplete(pkg, "darwin", "arm64")
	if !complete {
		t.Fatalf("expected complete with vendor-layout binary, got incomplete")
	}
	if path != vendorBin {
		t.Fatalf("expected resolved path %q, got %q", vendorBin, path)
	}
}

func TestServiceCodexPlatformBinaryComplete(t *testing.T) {
	pkg := t.TempDir()
	binPath := filepath.Join(pkg, "node_modules", "@openai", "codex-darwin-arm64", "codex")

	svc := Service{IsExecutableFile: func(p string) bool {
		info, err := os.Stat(p)
		return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
	}}

	// Missing subpackage binary -> incomplete (this is the report's ENOENT root cause).
	if path, complete := svc.codexPlatformBinaryComplete(pkg, "darwin", "arm64"); complete {
		t.Fatalf("expected incomplete when binary missing, got complete (path=%q)", path)
	}

	// Present but not executable -> still incomplete.
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, complete := svc.codexPlatformBinaryComplete(pkg, "darwin", "arm64"); complete {
		t.Fatalf("expected incomplete when binary not executable")
	}

	// Present and executable -> complete.
	if err := os.Chmod(binPath, 0o755); err != nil {
		t.Fatal(err)
	}
	path, complete := svc.codexPlatformBinaryComplete(pkg, "darwin", "arm64")
	if !complete || path != binPath {
		t.Fatalf("expected complete with path=%q, got (%q,%v)", binPath, path, complete)
	}
}
