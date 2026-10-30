//go:build linux || darwin

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

type harness struct {
	t        *testing.T
	tm       tmux
	env      []string
	stateDir string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not installed")
	}
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	directory := t.TempDir()
	h := &harness{t: t, tm: tmux{[]string{binary, "-L", fmt.Sprintf("ck-go-test-%d-%d", os.Getpid(), time.Now().UnixNano()), "-f", os.DevNull}}, stateDir: directory}
	t.Cleanup(func() { _, _ = h.tm.run("kill-server") })
	h.must("new-session", "-d", "-s", "base", "sleep 120")
	h.must("set-option", "-g", "default-shell", "/bin/sh")
	h.must("set-option", "-g", "default-command", "sleep 120")
	wrapper := fmt.Sprintf("#!/bin/sh\nexec %q -L %q -f %q \"$@\"\n", binary, h.tm.prefix[2], os.DevNull)
	if err := os.WriteFile(filepath.Join(directory, "tmux"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	h.env = append(os.Environ(), "PATH="+directory+":"+os.Getenv("PATH"), "XDG_STATE_HOME="+directory, "TERM=screen-256color")
	return h
}

func (h *harness) must(args ...string) string {
	h.t.Helper()
	result, err := h.tm.run(args...)
	if err != nil {
		h.t.Fatalf("tmux %v: %v", args, err)
	}
	return result
}
func (h *harness) cli(args ...string) (string, error) {
	cmd := exec.Command(testBinary, args...)
	cmd.Env = h.env
	output, err := cmd.CombinedOutput()
	return string(output), err
}

type terminal struct {
	t      *testing.T
	file   *os.File
	done   chan error
	mu     sync.Mutex
	output bytes.Buffer
}

func (h *harness) start(args ...string) *terminal {
	h.t.Helper()
	cmd := exec.Command(testBinary, args...)
	cmd.Env = h.env
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 100})
	if err != nil {
		h.t.Fatal(err)
	}
	p := &terminal{t: h.t, file: file, done: make(chan error, 1)}
	go func() { p.done <- cmd.Wait() }()
	go func() {
		buffer := make([]byte, 8192)
		for {
			n, err := file.Read(buffer)
			if n > 0 {
				p.mu.Lock()
				p.output.Write(buffer[:n])
				p.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	h.t.Cleanup(func() { _ = cmd.Process.Kill(); _ = file.Close() })
	return p
}
func (p *terminal) send(text string) {
	p.t.Helper()
	p.mu.Lock()
	p.output.Reset()
	p.mu.Unlock()
	if _, err := io.WriteString(p.file, text); err != nil {
		p.t.Fatal(err)
	}
}
func (p *terminal) wait(text string) {
	p.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		found := strings.Contains(p.output.String(), text)
		p.mu.Unlock()
		if found {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	p.mu.Lock()
	output := p.output.String()
	p.mu.Unlock()
	p.t.Fatalf("did not see %q; output=%q", text, output)
}
func (p *terminal) exit() {
	p.t.Helper()
	select {
	case err := <-p.done:
		if err != nil {
			p.t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		p.t.Fatal("ck did not exit")
	}
}
func (h *harness) client(name string) {
	h.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		clients, _ := h.tm.run("list-clients", "-F", "#{client_session}")
		for _, line := range strings.Split(clients, "\n") {
			if line == name {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("no attached client for %q", name)
}
func (h *harness) detach(name string) {
	h.t.Helper()
	s, err := h.tm.findName(name)
	if err != nil {
		h.t.Fatal(err)
	}
	h.must("detach-client", "-s", s.ID)
}

func TestRealCRUD(t *testing.T) {
	h := newHarness(t)
	id, err := h.tm.create("中文项目")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "$") {
		t.Fatal(id)
	}
	if err = h.tm.rename(id, "-renamed"); err != nil {
		t.Fatal(err)
	}
	if s, err := h.tm.findName("-renamed"); err != nil || s.ID != id {
		t.Fatalf("rename: %+v %v", s, err)
	}
	if _, err = h.tm.create("-renamed"); err == nil {
		t.Fatal("duplicate name accepted")
	}
	if _, err = h.tm.findName("-rename"); err == nil {
		t.Fatal("partial name matched")
	}
	if err = h.tm.kill(id); err != nil {
		t.Fatal(err)
	}
}

func TestCLIListAndSafeKill(t *testing.T) {
	h := newHarness(t)
	if out, err := h.cli("ls"); err != nil || !strings.Contains(out, "base") {
		t.Fatalf("%s %v", out, err)
	}
	if out, err := h.cli("kill", "base"); err == nil || !strings.Contains(out, "--yes") {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := h.cli(); err == nil {
		t.Fatal("non-terminal menu accepted")
	}
	if _, err := h.cli("help"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.cli("kill", "base", "--yes"); err != nil {
		t.Fatal(err)
	}
	if out, err := h.cli("ls"); err != nil || !strings.Contains(out, "没有 tmux 会话") {
		t.Fatalf("%s %v", out, err)
	}
}

func (h *harness) waitDirectory(name, directory string) {
	h.t.Helper()
	// A newly spawned pane may briefly report the server's cwd before the
	// child has changed directory. Wait for tmux's live process data.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s, err := h.tm.findName(name)
		if err == nil && s.Path == directory {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("directory for %q did not become %q", name, directory)
}

func TestDirectoryAndHelp(t *testing.T) {
	h := newHarness(t)
	directory := filepath.Join(t.TempDir(), "中文 project")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	h.must("new-session", "-d", "-s", "directory", "-c", directory, "sleep 120")
	h.waitDirectory("directory", directory)
	if out, err := h.cli("ls"); err != nil || !strings.Contains(out, directory) {
		t.Fatalf("ls: %s %v", out, err)
	}
	activeDir := filepath.Join(directory, "active-window")
	if err := os.Mkdir(activeDir, 0700); err != nil {
		t.Fatal(err)
	}
	h.must("new-window", "-t", "directory", "-c", activeDir, "sleep 120")
	h.waitDirectory("directory", activeDir)
	p := h.start()
	p.wait("会话管理")
	p.send("\x1bOB")
	p.wait(activeDir)
	p.send("?")
	p.wait("快捷键帮助")
	p.send("q") // Close help, not the application.
	p.wait("会话管理")
	p.wait(activeDir)
	p.send("q")
	p.exit()
}

func TestMenuRenameSearchDelete(t *testing.T) {
	h := newHarness(t)
	p := h.start()
	p.wait("会话管理")
	p.send("99\r")
	p.wait("不存在")
	p.send("r")
	p.wait("重命名会话")
	p.send("\x15renamed\r")
	p.wait("已重命名")
	p.send("/")
	p.wait("搜索会话名称")
	p.send("missing\r")
	p.wait("没有匹配的会话")
	p.send("\x1b")
	p.wait("renamed")
	p.send("d")
	p.wait("输入 yes 确认")
	p.send("\r")
	p.wait("已取消")
	p.send("d")
	p.wait("输入 yes 确认")
	p.send("yes")
	p.wait("> yes▏")
	p.send("\r")
	p.wait("已结束")
	if sessions, err := h.tm.sessions(); err != nil || len(sessions) != 0 {
		t.Fatalf("%+v %v", sessions, err)
	}
	p.send("q")
	p.exit()
}

func TestMenuNumberAndArrowAttach(t *testing.T) {
	for _, keys := range []string{"1\r", "\x1bOB\r"} {
		t.Run(fmt.Sprintf("%q", keys), func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.tm.create("zzz"); err != nil {
				t.Fatal(err)
			}
			p := h.start()
			p.wait("会话管理")
			p.send(keys)
			name := "base"
			if strings.HasPrefix(keys, "\x1b") {
				name = "zzz"
			}
			h.client(name)
			h.detach(name)
			p.exit()
		})
	}
}

func TestMenuCreateAndLast(t *testing.T) {
	h := newHarness(t)
	p := h.start()
	p.wait("会话管理")
	p.send("n")
	p.wait("输入名称")
	p.send("新项目")
	p.wait("> 新项目▏")
	p.send("\r")
	h.client("新项目")
	h.detach("新项目")
	p.exit()
	p = h.start("last")
	h.client("新项目")
	h.detach("新项目")
	p.exit()
}

func TestMenuInsideSwitchAndCurrentGuard(t *testing.T) {
	h := newHarness(t)
	other, err := h.tm.create("zzz")
	if err != nil {
		t.Fatal(err)
	}
	// Run ck from a real tmux pane with a live client, not merely a fabricated TMUX variable.
	h.must("set-option", "-g", "default-command", "")
	h.must("new-window", "-t", "base", "-n", "ck", "env PATH="+h.stateDir+":"+os.Getenv("PATH")+" XDG_STATE_HOME="+h.stateDir+" "+testBinary)
	p := h.start("attach", "base")
	h.client("base")
	time.Sleep(500 * time.Millisecond)
	h.must("send-keys", "-t", "base:ck", "d")
	time.Sleep(100 * time.Millisecond)
	capture := h.must("capture-pane", "-p", "-t", "base:ck")
	if !strings.Contains(capture, "不能结束") {
		t.Fatalf("current session not protected: %s", capture)
	}
	h.must("send-keys", "-t", "base:ck", "Down", "Enter")
	h.client("zzz")
	h.must("detach-client", "-s", other)
	p.exit()
}
