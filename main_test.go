package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var testBinary string

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "ck-go-tests-")
	if err != nil {
		panic(err)
	}
	testBinary = filepath.Join(directory, "ck")
	build := exec.Command("go", "build", "-o", testBinary, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(directory)
		panic(string(output) + err.Error())
	}
	code := m.Run()
	os.RemoveAll(directory)
	os.Exit(code)
}

func TestValidateName(t *testing.T) {
	for _, name := range []string{"", " ", "a.b", "a:b", "a\nb", "a\tb", "x\x1by", "a\u202eb"} {
		if validateName(name) == nil {
			t.Errorf("accepted invalid name %q", name)
		}
	}
	for _, name := range []string{"api", "中文项目", "my project", "-test", "$1"} {
		if err := validateName(name); err != nil {
			t.Errorf("rejected %q: %v", name, err)
		}
	}
}

func key(m model, name string) model {
	var msg tea.KeyMsg
	switch name {
	case "enter":
		msg.Type = tea.KeyEnter
	case "up":
		msg.Type = tea.KeyUp
	case "down":
		msg.Type = tea.KeyDown
	case "esc":
		msg.Type = tea.KeyEsc
	default:
		msg.Type = tea.KeyRunes
		msg.Runes = []rune(name)
	}
	next, _ := m.Update(msg)
	return next.(model)
}

func sampleModel() model {
	return model{width: 100, height: 30, sessions: []session{
		{ID: "$1", Name: "api", Windows: 1}, {ID: "$2", Name: "中文项目", Windows: 2},
	}, selected: "$1"}
}

func TestMenuNavigationAndNumbers(t *testing.T) {
	m := sampleModel()
	m = key(m, "up")
	if m.selected != "$2" {
		t.Fatal("up did not wrap")
	}
	m = key(m, "down")
	if m.selected != "$1" {
		t.Fatal("down did not wrap")
	}
	m = key(m, "2")
	m = key(m, "enter")
	if m.result != "$2" {
		t.Fatal("number did not select session")
	}
	m = sampleModel()
	m = key(m, "99") // A fast pair of digits may arrive in one input event.
	m = key(m, "enter")
	if m.result != "" || !strings.Contains(m.message, "编号不存在") {
		t.Fatal("invalid number accepted")
	}
}

func TestMenuMultiDigitAndSearch(t *testing.T) {
	m := sampleModel()
	for i := 3; i <= 12; i++ {
		m.sessions = append(m.sessions, session{ID: string(rune('a' + i)), Name: "extra"})
	}
	m = key(key(m, "1"), "2")
	m = key(m, "enter")
	if m.result != m.sessions[11].ID {
		t.Fatal("12 did not choose twelfth session")
	}
	m = sampleModel()
	m = key(m, "/")
	m = key(m, "中文")
	m = key(m, "enter")
	if m.mode != "" || m.query != "中文" || m.selected != "$2" {
		t.Fatalf("search failed: %+v", m)
	}
	m = key(m, "esc")
	if m.query != "" {
		t.Fatal("escape did not clear search")
	}
}

func TestPromptCancellationAndSafety(t *testing.T) {
	m := sampleModel()
	m.current = "$1"
	m = key(m, "d")
	if m.mode != "" || !strings.Contains(m.message, "不能结束") {
		t.Fatal("current session could be killed")
	}
	m.current = ""
	m = key(m, "d")
	if m.mode != "kill" {
		t.Fatal("missing kill confirmation")
	}
	m = key(m, "enter")
	if m.mode != "" || m.message != "已取消" {
		t.Fatal("empty confirmation did not cancel")
	}
	m = key(key(m, "n"), "esc")
	if m.mode != "" {
		t.Fatal("escape did not cancel prompt")
	}
}

func TestSafeText(t *testing.T) {
	if strings.ContainsAny(safeText("a\x1b[31m\nb\u202e"), "\x1b\n\u202e") {
		t.Fatal("terminal control text not sanitized")
	}
	m := sampleModel()
	if !strings.Contains(m.View(), "中文项目") {
		t.Fatal("Chinese name not rendered")
	}
	m.width, m.height = 20, 5
	if !strings.Contains(m.View(), "终端过小") {
		t.Fatal("missing small-terminal hint")
	}
}

func TestHelpPreservesMenuState(t *testing.T) {
	m := sampleModel()
	m.selected, m.query, m.digits = "$2", "中文", "1"
	m = key(m, "?")
	if !m.showHelp || !strings.Contains(m.View(), "快捷键帮助") {
		t.Fatal("help did not open")
	}
	for _, closeKey := range []string{"?", "esc", "q"} {
		closed := key(m, closeKey)
		if closed.showHelp || closed.selected != "$2" || closed.query != "中文" || closed.digits != "1" {
			t.Fatalf("help did not preserve state with %q: %+v", closeKey, closed)
		}
	}
	m.height = 10
	for range len(menuHelp) {
		m = key(m, "down")
	}
	if !strings.Contains(m.View(), "禁止结束当前会话") {
		t.Fatal("help did not scroll to the end")
	}
	m = sampleModel()
	m.startPrompt("new", "")
	m = key(m, "?")
	if m.showHelp || m.input != "?" {
		t.Fatal("question mark was not input in a prompt")
	}
}

func TestSelectedDirectory(t *testing.T) {
	m := sampleModel()
	m.sessions[0].Path = "/tmp/api"
	m.sessions[1].Path = "/tmp/中文目录"
	if !strings.Contains(m.View(), "当前目录: /tmp/api") {
		t.Fatal("missing current directory")
	}
	m = key(m, "down")
	if !strings.Contains(m.View(), "当前目录: /tmp/中文目录") {
		t.Fatal("directory did not follow selection")
	}
	m.sessions[1].Path = ""
	if !strings.Contains(m.View(), "无法获取") {
		t.Fatal("missing unavailable-directory fallback")
	}
}

func TestPromptInputEcho(t *testing.T) {
	for _, mode := range []string{"new", "rename", "search", "kill"} {
		for _, input := range []string{"", "yes", "项目名称", "my project"} {
			t.Run(mode+"/"+input, func(t *testing.T) {
				m := sampleModel()
				m.startPrompt(mode, "")
				if input != "" {
					m = key(m, input)
				}
				if !strings.Contains(m.View(), "> "+input+"▏") {
					t.Fatalf("input not echoed: %q", m.View())
				}
			})
		}
	}
}

func TestLongPromptInputKeepsVisibleTail(t *testing.T) {
	m := sampleModel()
	m.width = 42
	m.startPrompt("new", strings.Repeat("中", 50)+"tail")
	view := m.View()
	if !strings.Contains(view, "tail▏") {
		t.Fatalf("long input lost its visible tail: %q", view)
	}
	if strings.Contains(view, m.input) {
		t.Fatal("long input was not clipped")
	}
}

func TestCorruptState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(statePath()), 0700); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"null", "invalid json", "[]"} {
		if err := os.WriteFile(statePath(), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if loadState() == nil {
			t.Fatal("nil state map")
		}
	}
}
