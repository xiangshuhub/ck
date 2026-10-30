//go:build linux || darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndUninstall(t *testing.T) {
	bundle := t.TempDir()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(bundle, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	for source, target := range map[string]string{
		testBinary:     filepath.Join(bundle, "bin", "ck"),
		"install.sh":   filepath.Join(bundle, "install.sh"),
		"uninstall.sh": filepath.Join(bundle, "uninstall.sh"),
	} {
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0755); err != nil {
			t.Fatal(err)
		}
	}
	runScript := func(name string, args ...string) (string, error) {
		cmd := exec.Command("sh", append([]string{filepath.Join(bundle, name)}, args...)...)
		cmd.Env = append(os.Environ(), "HOME="+home)
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	if out, err := runScript("install.sh"); err != nil {
		t.Fatalf("install: %s %v", out, err)
	}
	target := filepath.Join(home, ".local", "bin", "ck")
	if info, err := os.Lstat(target); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("not standalone: %v %v", info, err)
	}
	if _, err := runScript("install.sh"); err == nil {
		t.Fatal("overwrote existing target without --force")
	}
	if out, err := runScript("install.sh", "--force"); err != nil {
		t.Fatalf("update: %s %v", out, err)
	}
	if out, err := runScript("uninstall.sh"); err != nil || !strings.Contains(out, "已取消") {
		t.Fatalf("confirmation: %s %v", out, err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("uninstall removed without confirmation")
	}
	// Removing the bundled executable cannot break an independently installed copy.
	if err := os.Remove(filepath.Join(bundle, "bin", "ck")); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(target, "--version").CombinedOutput(); err != nil || !strings.Contains(string(out), "(Go)") {
		t.Fatalf("standalone: %s %v", out, err)
	}
	if out, err := runScript("uninstall.sh", "--yes"); err != nil {
		t.Fatalf("uninstall: %s %v", out, err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists: %v", err)
	}
	if _, err := runScript("uninstall.sh", "--yes"); err != nil {
		t.Fatal("uninstall is not idempotent")
	}
}

func TestInstallReplacesSymlinkNotItsTarget(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "bin with spaces")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(t.TempDir(), "original")
	if err := os.WriteFile(original, []byte("do not overwrite"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "ck")
	if err := os.Symlink(original, target); err != nil {
		t.Fatal(err)
	}
	// Supply a temporary bundle rather than relying on the developer's bin/ck.
	bundle := t.TempDir()
	if err := os.Mkdir(filepath.Join(bundle, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	for source, dest := range map[string]string{"install.sh": filepath.Join(bundle, "install.sh"), testBinary: filepath.Join(bundle, "bin", "ck")} {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", filepath.Join(bundle, "install.sh"), "--force", directory)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	if info, err := os.Lstat(target); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("not regular: %v %v", info, err)
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != "do not overwrite" {
		t.Fatal("symlink target was overwritten")
	}
}
