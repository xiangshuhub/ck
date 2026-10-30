package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/x/term"
)

const help = `CK · 简单的 tmux 会话管理器（Go 版）

用法：
  ck                 打开交互菜单
  ck ls              列出会话
  ck new 名称        新建并进入
  ck attach 名称     进入已有会话（名称精确匹配）
  ck kill 名称       确认后结束会话
  ck kill 名称 --yes  跳过确认，供脚本使用
  ck last            返回上次使用的会话
  ck help            查看帮助
  ck --version       查看版本

菜单：↑↓ 选择 · 数字+回车 进入 · n 新建 · r 改名 · d 结束 · / 搜索 · ? 帮助 · q 退出
离开会话并保持后台运行：Ctrl+b，松开，再按 d（tmux 默认按键）。
结束会话可能终止其中的程序；禁止结束当前所在会话。
`

func usageError() error { return errors.New("参数不正确；使用 ck help 查看帮助") }
func interactive() bool { return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) }

func run(args []string) error {
	command, name, yes := "", "", false
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "help", "-h", "--help":
		if len(args) != 1 {
			return usageError()
		}
		fmt.Print(help)
		return nil
	case "--version":
		if len(args) != 1 {
			return usageError()
		}
		fmt.Println("ck 1.1.0 (Go)")
		return nil
	case "new", "attach":
		if len(args) != 2 {
			return usageError()
		}
		name = args[1]
	case "kill":
		for _, arg := range args[1:] {
			if arg == "--yes" || arg == "-y" {
				yes = true
			} else if name == "" {
				name = arg
			} else {
				return usageError()
			}
		}
		if name == "" {
			return usageError()
		}
	case "ls", "last":
		if len(args) != 1 {
			return usageError()
		}
	case "":
	default:
		return usageError()
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return errors.New("未找到 tmux，请先安装 tmux")
	}
	t := tmux{}
	if command == "ls" {
		sessions, err := t.sessions()
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			fmt.Println("没有 tmux 会话")
		}
		for i, s := range sessions {
			status := "后台"
			if s.Attached > 0 {
				status = "使用中"
			}
			fmt.Printf("%d\t%s\t%s\t%d 个窗口\t%s\n", i+1, safeText(s.Name), status, s.Windows, safeText(s.Path))
		}
		return nil
	}
	if command == "kill" {
		s, err := t.findName(name)
		if err != nil {
			return err
		}
		if s.ID == t.current() {
			return errors.New("不能结束当前所在会话；请从另一个会话或外部终端操作")
		}
		if !yes {
			if !term.IsTerminal(os.Stdin.Fd()) {
				return errors.New("结束会话需要交互确认；脚本中请显式传入 --yes")
			}
			fmt.Printf("结束「%s」会终止其中的程序，输入 yes 确认（默认取消）：", safeText(s.Name))
			answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil || strings.TrimSuffix(strings.TrimSuffix(answer, "\n"), "\r") != "yes" {
				fmt.Println("已取消")
				return nil
			}
		}
		if err := t.kill(s.ID); err != nil {
			return err
		}
		fmt.Println("已结束：" + safeText(s.Name))
		return nil
	}
	if !interactive() {
		return errors.New("此操作需要交互终端；查看列表请使用 ck ls")
	}
	var id string
	var err error
	switch command {
	case "new":
		id, err = t.create(name)
	case "attach":
		var s session
		s, err = t.findName(name)
		id = s.ID
	case "last":
		name = t.lastName()
		if name == "" {
			return errors.New("还没有使用记录，请先运行 ck 选择会话")
		}
		var s session
		s, err = t.findName(name)
		id = s.ID
	default:
		id, err = pick(t)
	}
	if err != nil {
		return err
	}
	if id != "" {
		return t.enter(id)
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ck: "+safeText(err.Error()))
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() > 0 {
			os.Exit(exit.ExitCode())
		}
		os.Exit(1)
	}
}
