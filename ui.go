package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

var menuHelp = []string{
	"↑ / ↓       选择会话（首尾循环）",
	"Enter       进入选中会话；空列表时新建",
	"数字+Enter  按当前列表编号进入，如 12",
	"n           新建会话并进入",
	"r           重命名选中会话",
	"d           结束会话，输入 yes 确认",
	"/           搜索名称，留空显示全部",
	"Ctrl+r      刷新列表",
	"Esc         清除编号/搜索；无筛选时退出",
	"q           退出菜单，不结束会话",
	"?           打开或关闭帮助",
	"输入框：Enter 确认 · Esc 取消 · Ctrl+u 清空",
	"Ctrl+c：退出整个菜单，包括输入框",
	"目录：选中会话的活动窗口、活动窗格当前目录",
	"离开会话：Ctrl+b，松开，再按 d（默认前缀）",
	"结束会话可能终止程序；禁止结束当前会话",
}

type refreshMsg time.Time

func refreshTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return refreshMsg(t) })
}

type model struct {
	tmux                                      tmux
	sessions                                  []session
	selected, current, query, digits, message string
	width, height                             int
	mode, input, target, targetName           string
	result                                    string
	err                                       error
	showHelp                                  bool
	helpOffset                                int
}

func newModel(t tmux) (model, error) {
	sessions, err := t.sessions()
	if err != nil {
		return model{}, err
	}
	m := model{tmux: t, sessions: sessions, current: t.current()}
	last := t.lastName()
	for _, s := range sessions {
		if s.Name == last {
			m.selected = s.ID
		}
	}
	m.normalize()
	return m, nil
}

func (m model) Init() tea.Cmd { return refreshTick() }
func (m model) visible() []session {
	var result []session
	for _, s := range m.sessions {
		if strings.Contains(strings.ToLower(s.Name), strings.ToLower(m.query)) {
			result = append(result, s)
		}
	}
	return result
}
func (m *model) normalize() int {
	visible := m.visible()
	for i, s := range visible {
		if s.ID == m.selected {
			return i
		}
	}
	m.selected = ""
	if len(visible) > 0 {
		m.selected = visible[0].ID
	}
	return 0
}
func (m *model) reload() {
	sessions, err := m.tmux.sessions()
	if err != nil {
		m.message = err.Error()
		return
	}
	m.sessions = sessions
	m.normalize()
}
func (m *model) startPrompt(mode, value string) {
	m.mode, m.input, m.digits = mode, value, ""
	m.target = m.selected
	for _, s := range m.sessions {
		if s.ID == m.target {
			m.targetName = s.Name
		}
	}
}
func (m *model) acceptPrompt() tea.Cmd {
	mode, value := m.mode, m.input
	m.mode, m.input = "", ""
	var err error
	switch mode {
	case "search":
		m.query = value
		m.normalize()
	case "new":
		m.result, err = m.tmux.create(value)
		if err == nil {
			return tea.Quit
		}
	case "rename":
		err = m.tmux.rename(m.target, value)
		if err == nil {
			m.message = "已重命名"
			if m.tmux.lastName() == m.targetName {
				m.tmux.remember(value)
			}
			m.reload()
		}
	case "kill":
		if value == "yes" {
			err = m.tmux.kill(m.target)
			if err == nil {
				m.message = "已结束：" + m.targetName
				m.reload()
			}
		} else {
			m.message = "已取消"
		}
	}
	if err != nil {
		m.message = err.Error()
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case refreshMsg:
		// Never change the meaning of a number or a confirmation while typing.
		if m.digits == "" && m.mode == "" && !m.showHelp {
			m.reload()
		}
		return m, refreshTick()
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.mode != "" {
			switch key {
			case "esc":
				m.mode, m.input = "", ""
			case "enter":
				cmd := m.acceptPrompt()
				return m, cmd
			case "backspace", "ctrl+h":
				runes := []rune(m.input)
				if len(runes) > 0 {
					m.input = string(runes[:len(runes)-1])
				}
			case "ctrl+u":
				m.input = ""
			default:
				if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
					for _, r := range msg.Runes {
						if unicode.IsPrint(r) {
							m.input += string(r)
						}
					}
				}
			}
			return m, nil
		}
		if m.showHelp {
			page := max(1, m.height-6)
			switch key {
			case "?", "esc", "q":
				m.showHelp = false
			case "up":
				m.helpOffset--
			case "down":
				m.helpOffset++
			case "pgup":
				m.helpOffset -= page
			case "pgdown":
				m.helpOffset += page
			}
			m.helpOffset = max(0, min(m.helpOffset, max(0, len(menuHelp)-page)))
			return m, nil
		}
		if key == "q" {
			return m, tea.Quit
		}
		if m.width < 42 || m.height < 10 {
			if key == "esc" {
				return m, tea.Quit
			}
			return m, nil
		}
		index := m.normalize()
		visible := m.visible()
		switch key {
		case "esc":
			if m.query != "" || m.digits != "" {
				m.query, m.digits = "", ""
				m.normalize()
			} else {
				return m, tea.Quit
			}
		case "up", "down":
			m.digits = ""
			if len(visible) > 0 {
				delta := 1
				if key == "up" {
					delta = -1
				}
				index = (index + delta + len(visible)) % len(visible)
				m.selected = visible[index].ID
			}
		case "backspace", "ctrl+h":
			if len(m.digits) > 0 {
				m.digits = m.digits[:len(m.digits)-1]
			}
		case "enter":
			if m.digits != "" {
				number, _ := strconv.Atoi(m.digits)
				if number < 1 || number > len(visible) {
					m.message, m.digits = "编号不存在，请重新输入", ""
					return m, nil
				}
				m.selected = visible[number-1].ID
			}
			if len(visible) > 0 {
				m.result = m.selected
				return m, tea.Quit
			}
			if len(m.sessions) == 0 && m.query == "" {
				m.startPrompt("new", "")
			}
		case "?":
			m.showHelp, m.helpOffset = true, 0
		case "n":
			m.startPrompt("new", "")
		case "/":
			m.startPrompt("search", m.query)
		case "r":
			if len(visible) > 0 {
				m.startPrompt("rename", visible[index].Name)
			}
		case "d":
			if len(visible) > 0 {
				if m.selected == m.current {
					m.message = "不能结束当前会话；请从另一个会话或外部终端操作"
				} else {
					m.startPrompt("kill", "")
				}
			}
		case "ctrl+r":
			m.digits = ""
			m.reload()
		default:
			// Fast typing and pasted numbers can arrive as one KeyRunes event.
			if msg.Type == tea.KeyRunes && key != "" && strings.Trim(key, "0123456789") == "" {
				m.digits += key
				if len(m.digits) > 8 {
					m.digits = m.digits[:8]
				}
				number, _ := strconv.Atoi(m.digits)
				if number > 0 && number <= len(visible) {
					m.selected = visible[number-1].ID
				}
				m.message = ""
			}
		}
	}
	return m, nil
}

// tmux names and error output are untrusted terminal text: strip escape/control codes.
func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, s)
}

func (m model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	lines := make([]string, m.height)
	put := func(row int, text string, reverse bool) {
		if row < 0 || row >= len(lines) {
			return
		}
		text = runewidth.Truncate(safeText(text), max(0, m.width-1), "")
		if reverse {
			text = "\x1b[7m" + text + "\x1b[0m"
		}
		lines[row] = text
	}
	if m.width < 42 || m.height < 10 {
		put(0, "终端过小，请放大窗口；q 退出", false)
		return strings.Join(lines, "\n")
	}
	if m.showHelp {
		put(1, "  CK · 快捷键帮助", false)
		page := max(1, m.height-6)
		start := min(m.helpOffset, max(0, len(menuHelp)-page))
		for i := start; i < len(menuHelp) && i < start+page; i++ {
			put(3+i-start, "  "+menuHelp[i], false)
		}
		put(m.height-2, "  ↑↓ 滚动 · ? / Esc / q 返回菜单", false)
		return strings.Join(lines, "\n")
	}
	if m.mode != "" {
		title := map[string]string{"new": "新建会话：输入名称", "rename": "重命名会话", "search": "搜索会话名称（留空显示全部）", "kill": "结束会话：" + m.targetName}[m.mode]
		put(1, "  "+title, false)
		if m.mode == "kill" {
			put(2, "  将终止其中的程序。输入 yes 确认（默认取消）", false)
		}
		value := safeText(m.input)
		if overflow := runewidth.StringWidth(value) - (m.width - 7); overflow > 0 {
			// TruncateLeft removes this many cells, not all but this many.
			value = runewidth.TruncateLeft(value, overflow, "")
		}
		put(4, "  > "+value+"▏", false)
		put(6, "  Enter 确认 · Esc 取消 · Ctrl+u 清空", false)
		return strings.Join(lines, "\n")
	}
	put(1, "  CK · tmux 会话管理", false)
	query := m.query
	if query == "" {
		query = "全部"
	}
	put(2, "  ↑↓ 选择 · Enter 进入 · 数字+回车 进入   搜索: "+query, false)
	put(4, "  编号   会话名称                         状态 / 窗口数", false)
	visible := m.visible()
	index := m.normalize()
	rows := m.height - 9
	start := max(0, index-rows+1)
	nameWidth := min(32, max(8, m.width-30))
	for i := start; i < len(visible) && i < start+rows; i++ {
		s := visible[i]
		name := runewidth.Truncate(safeText(s.Name), nameWidth, "")
		name += strings.Repeat(" ", max(0, nameWidth-runewidth.StringWidth(name)))
		status := "后台"
		if s.ID == m.current {
			status = "当前"
		} else if s.Attached > 0 {
			status = "使用中"
		}
		marker := " "
		if i == index {
			marker = ">"
		}
		put(5+i-start, fmt.Sprintf(" %s %3d   %s  %s / %d", marker, i+1, name, status, s.Windows), i == index)
	}
	if len(visible) == 0 {
		text := "  没有会话，按 Enter 或 n 新建"
		if m.query != "" {
			text = "  没有匹配的会话，按 / 修改搜索"
		}
		put(5, text, false)
	}
	if len(visible) > 0 {
		path := visible[index].Path
		if path == "" {
			path = "（无法获取）"
		}
		put(m.height-4, "  当前目录: "+path, false)
	}
	message := m.message
	if m.digits != "" {
		message = "编号: " + m.digits
	}
	put(m.height-3, "  "+message, false)
	put(m.height-2, "  n 新建 · r 改名 · d 结束 · / 搜索 · ? 帮助 · q 退出", false)
	return strings.Join(lines, "\n")
}

func pick(t tmux) (string, error) {
	m, err := newModel(t)
	if err != nil {
		return "", err
	}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return "", fmt.Errorf("无法打开交互菜单：%w；可用 ck ls 查看会话", err)
	}
	result := final.(model)
	return result.result, result.err
}
