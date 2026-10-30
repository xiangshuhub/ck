package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type session struct {
	ID, Name, Path    string
	Windows, Attached int
}

type tmux struct{ prefix []string }

func (t tmux) command(args ...string) *exec.Cmd {
	prefix := t.prefix
	if len(prefix) == 0 {
		prefix = []string{"tmux"}
	}
	return exec.Command(prefix[0], append(append([]string{}, prefix[1:]...), args...)...)
}

func (t tmux) run(args ...string) (string, error) {
	cmd := t.command(args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

func (t tmux) sessions() ([]session, error) {
	output, err := t.run("list-sessions", "-F", "#{session_id}\t#{session_name}\t#{session_windows}\t#{session_attached}")
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no server running") || (strings.Contains(msg, "error connecting to") && strings.Contains(msg, "No such file or directory")) {
			return nil, nil
		}
		return nil, err
	}
	var result []session
	if output == "" {
		return result, nil
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			return nil, fmt.Errorf("无法解析 tmux 会话信息")
		}
		windows, e1 := strconv.Atoi(fields[2])
		attached, e2 := strconv.Atoi(fields[3])
		if e1 != nil || e2 != nil {
			return nil, fmt.Errorf("无法解析 tmux 会话状态")
		}
		result = append(result, session{ID: fields[0], Name: fields[1], Windows: windows, Attached: attached})
	}
	// list-sessions does not reliably provide each session's pane context.
	// Resolve the active window and pane explicitly across all sessions.
	panes, err := t.run("list-panes", "-a", "-F", "#{session_id}\t#{window_active}\t#{pane_active}\t#{pane_current_path}")
	if err != nil {
		return nil, err
	}
	paths := make(map[string]string)
	for _, line := range strings.Split(panes, "\n") {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) == 4 && fields[1] == "1" && fields[2] == "1" {
			paths[fields[0]] = fields[3]
		}
	}
	for i := range result {
		result[i].Path = paths[result[i].ID]
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := strings.ToLower(result[i].Name), strings.ToLower(result[j].Name)
		if a == b {
			return result[i].Name < result[j].Name
		}
		return a < b
	})
	return result, nil
}

// Names and IDs are resolved separately: a session named "$1" must not select ID $1.
func (t tmux) findName(name string) (session, error) {
	sessions, err := t.sessions()
	if err != nil {
		return session{}, err
	}
	for _, s := range sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return session{}, fmt.Errorf("找不到会话：%s", name)
}

func (t tmux) findID(id string) (session, error) {
	sessions, err := t.sessions()
	if err != nil {
		return session{}, err
	}
	for _, s := range sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return session{}, fmt.Errorf("会话已不存在，请重新打开 ck 选择")
}

func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("会话名称不能为空")
	}
	for _, r := range name {
		if r == '.' || r == ':' || unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return errors.New("会话名称不能包含 .、: 或控制字符")
		}
	}
	return nil
}

func (t tmux) create(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return t.run("new-session", "-d", "-P", "-F", "#{session_id}", "-s", name, "-c", cwd)
}

func (t tmux) rename(id, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	_, err := t.run("rename-session", "-t", id, "--", name)
	return err
}

func (t tmux) current() string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	args := []string{"display-message", "-p"}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		args = append(args, "-t", pane)
	}
	id, _ := t.run(append(args, "#{session_id}")...)
	return id
}

func (t tmux) kill(id string) error {
	if id == t.current() {
		return errors.New("不能结束当前所在会话；请从另一个会话或外部终端操作")
	}
	_, err := t.run("kill-session", "-t", id)
	return err
}

func (t tmux) enter(id string) error {
	s, err := t.findID(id)
	if err != nil {
		return err
	}
	if os.Getenv("TMUX") != "" {
		if _, err = t.run("switch-client", "-t", s.ID); err == nil {
			t.remember(s.Name)
		}
		return err
	}
	t.remember(s.Name)
	cmd := t.command("attach-session", "-t", s.ID)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
