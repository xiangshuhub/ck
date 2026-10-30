//go:build linux || darwin

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRemoteInstallVerifiesChecksum(t *testing.T) {
	directory := t.TempDir()
	bundle, release, fakeBin := filepath.Join(directory, "bundle"), filepath.Join(directory, "release"), filepath.Join(directory, "tools")
	for _, path := range []string{bundle, release, fakeBin} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	installer, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "install.sh"), installer, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(testBinary)
	if err != nil {
		t.Fatal(err)
	}
	asset := "ck_" + runtime.GOOS + "_" + runtime.GOARCH
	if err := os.WriteFile(filepath.Join(release, asset), data, 0600); err != nil {
		t.Fatal(err)
	}
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(data), asset)
	if err := os.WriteFile(filepath.Join(release, "SHA256SUMS"), []byte(checksum), 0600); err != nil {
		t.Fatal(err)
	}
	fakeCurl := `#!/bin/sh
set -eu
url=''
out=''
while [ "$#" -gt 0 ]; do
 case "$1" in
  -o) shift; out=$1 ;;
  https://*) url=$1 ;;
 esac
 shift
done
cp "$FAKE_RELEASE/${url##*/}" "$out"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte(fakeCurl), 0700); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(directory, "installed")
	run := func() (string, error) {
		cmd := exec.Command("sh", filepath.Join(bundle, "install.sh"), "--force", targetDir)
		cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "FAKE_RELEASE="+release, "CK_VERSION=v1.1.0")
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	if output, err := run(); err != nil {
		t.Fatalf("remote install: %s %v", output, err)
	}
	target := filepath.Join(targetDir, "ck")
	if output, err := exec.Command(target, "--version").CombinedOutput(); err != nil || !strings.Contains(string(output), "(Go)") {
		t.Fatalf("binary: %s %v", output, err)
	}
	// A corrupted download must not replace the previously installed executable.
	if err := os.WriteFile(filepath.Join(release, asset), []byte("corrupted download"), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := run(); err == nil || !strings.Contains(output, "SHA-256 校验失败") {
		t.Fatalf("checksum mismatch accepted: %s %v", output, err)
	}
	installed, err := os.ReadFile(target)
	if err != nil || sha256.Sum256(installed) != sha256.Sum256(data) {
		t.Fatal("failed download replaced installed executable")
	}
	if err := os.WriteFile(filepath.Join(release, "SHA256SUMS"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := run(); err == nil {
		t.Fatal("empty checksum file accepted")
	}
}
