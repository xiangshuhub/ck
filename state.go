package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func statePath() string {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "ck", "last.json")
}

func (t tmux) serverKey() string {
	if socket, err := t.run("display-message", "-p", "#{socket_path}"); err == nil && socket != "" {
		return socket
	}
	if socket := strings.Split(os.Getenv("TMUX"), ",")[0]; socket != "" {
		return socket
	}
	return "default"
}

func loadState() map[string]string {
	data := map[string]string{}
	content, err := os.ReadFile(statePath())
	if err == nil {
		_ = json.Unmarshal(content, &data)
	}
	if data == nil {
		data = map[string]string{}
	}
	return data
}

func (t tmux) lastName() string { return loadState()[t.serverKey()] }

func (t tmux) remember(name string) {
	path := statePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	data := loadState()
	data[t.serverKey()] = name
	content, err := json.Marshal(data)
	if err != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".last-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		_ = os.Rename(file.Name(), path)
	}
}
