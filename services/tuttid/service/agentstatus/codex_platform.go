package agentstatus

import "path/filepath"

// codexNpmPlatformDir returns the @openai npm optional-subpackage directory
// name that holds the platform-specific codex binary, e.g.
// "codex-darwin-arm64". ok is false for platforms codex does not publish.
//
// The @openai/codex npm package is only a JS launcher; the real binary lives
// in a per-platform optionalDependency subpackage. A missing/incomplete
// subpackage is the root cause of the spawn ENOENT seen in the field.
func codexNpmPlatformDir(goos, goarch string) (string, bool) {
	var nodeOS string
	switch goos {
	case "darwin":
		nodeOS = "darwin"
	case "linux":
		nodeOS = "linux"
	case "windows":
		nodeOS = "win32"
	default:
		return "", false
	}
	var nodeArch string
	switch goarch {
	case "arm64":
		nodeArch = "arm64"
	case "amd64":
		nodeArch = "x64"
	case "386":
		nodeArch = "ia32"
	default:
		return "", false
	}
	return "codex-" + nodeOS + "-" + nodeArch, true
}

// codexVendorTargetTriple returns the Rust target triple codex uses to namespace
// the native binary inside the platform subpackage's vendor/ directory, e.g.
// "aarch64-apple-darwin". This mirrors the resolution in the @openai/codex
// launcher (bin/codex.js), which joins vendor/<triple>/bin/codex. ok is false
// for platforms codex does not publish a vendored binary for.
func codexVendorTargetTriple(goos, goarch string) (string, bool) {
	switch goos {
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-darwin", true
		case "arm64":
			return "aarch64-apple-darwin", true
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-musl", true
		case "arm64":
			return "aarch64-unknown-linux-musl", true
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "x86_64-pc-windows-msvc", true
		case "arm64":
			return "aarch64-pc-windows-msvc", true
		}
	}
	return "", false
}

func codexPlatformBinaryName(goos string) string {
	if goos == "windows" {
		return "codex.exe"
	}
	return "codex"
}

// codexPlatformBinaryPath returns the legacy absolute path to the
// platform-specific codex binary, at the subpackage root. Older codex releases
// shipped the binary here directly.
func codexPlatformBinaryPath(codexPkgDir, goos, goarch string) (string, bool) {
	dir, ok := codexNpmPlatformDir(goos, goarch)
	if !ok {
		return "", false
	}
	return filepath.Join(codexPkgDir, "node_modules", "@openai", dir, codexPlatformBinaryName(goos)), true
}

// codexPlatformVendorBinaryPath returns the absolute path to the
// platform-specific codex binary under the vendor/ layout used by codex >= 0.140
// (e.g. .../@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex).
func codexPlatformVendorBinaryPath(codexPkgDir, goos, goarch string) (string, bool) {
	dir, dirOK := codexNpmPlatformDir(goos, goarch)
	triple, tripleOK := codexVendorTargetTriple(goos, goarch)
	if !dirOK || !tripleOK {
		return "", false
	}
	return filepath.Join(codexPkgDir, "node_modules", "@openai", dir, "vendor", triple, "bin", codexPlatformBinaryName(goos)), true
}

// codexPlatformBinaryCandidates lists the absolute paths where the
// platform-specific codex binary may live inside an installed @openai/codex
// package directory, newest layout first. codex changed its subpackage layout
// (the native binary moved from the package root into vendor/<triple>/bin/), so
// the resolver must accept both to stay compatible across versions.
func codexPlatformBinaryCandidates(codexPkgDir, goos, goarch string) []string {
	var candidates []string
	if vendor, ok := codexPlatformVendorBinaryPath(codexPkgDir, goos, goarch); ok {
		candidates = append(candidates, vendor)
	}
	if legacy, ok := codexPlatformBinaryPath(codexPkgDir, goos, goarch); ok {
		candidates = append(candidates, legacy)
	}
	return candidates
}

// codexPlatformBinaryComplete reports whether the platform-specific codex
// binary is present and executable inside the given @openai/codex package
// directory. It returns the resolved binary path alongside the verdict; when no
// candidate is executable it returns the preferred (newest-layout) path so the
// detail message points at where the binary is expected.
func (s Service) codexPlatformBinaryComplete(codexPkgDir, goos, goarch string) (string, bool) {
	candidates := codexPlatformBinaryCandidates(codexPkgDir, goos, goarch)
	if len(candidates) == 0 {
		return "", false
	}
	for _, path := range candidates {
		if s.executableFile(path) {
			return path, true
		}
	}
	return candidates[0], false
}

func codexPackageDirForBinary(binaryPath string) string {
	packageJSONPath := findAdapterPackageJSON(binaryPath, "@openai/codex")
	if packageJSONPath == "" {
		return ""
	}
	return filepath.Dir(packageJSONPath)
}
