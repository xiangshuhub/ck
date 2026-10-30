# ck：轻量的 tmux 会话选择器（Go 版）

中文交互菜单，只管理 tmux 会话，不做分屏、窗口管理或项目配置。

**运行时只需要 tmux 和交互终端，不需要 Python、curses、Go、Node.js 或 fzf。**
纯 Go 编译为单个静态可执行文件；界面库已编译进文件，无须另行安装。

## 使用

```bash
ck                 # 打开交互菜单
ck new api         # 新建 api 并进入
ck attach api      # 进入 api（名称精确匹配，不自动新建）
ck ls              # 列出会话，不需要交互终端
ck last            # 返回上次通过 ck 进入的会话
ck kill api        # 确认后结束会话
ck kill api --yes  # 跳过确认，供脚本使用，谨慎操作
ck help            # 帮助
ck --version       # 查看版本
```

### 菜单快捷键

| 按键 | 功能 |
| --- | --- |
| ↑ / ↓ | 选择会话，支持首尾循环 |
| Enter | 进入选中会话；没有会话时新建 |
| 数字 + Enter | 进入对应编号，如 `12` + Enter |
| n | 输入名称，新建并进入 |
| r | 重命名选中会话 |
| d | 结束选中会话，必须输入 `yes` 确认 |
| / | 按名称搜索，不区分大小写；清空搜索显示全部 |
| Ctrl+r | 刷新列表 |
| ? | 打开快捷键帮助；帮助页按 ↑↓ 滚动，按 ? / Esc / q 返回菜单 |
| q | 退出菜单，不结束会话 |
| Esc | 清除编号/搜索；没有筛选时退出 |
| Ctrl+c | 退出菜单，包括输入框 |

输入框支持退格、Ctrl+u 清空、Enter 确认、Esc 取消，输入时允许使用空格和中文。

- 列表按名称稳定排序，每两秒自动刷新；输入数字或打开输入框时暂停刷新，避免误操作。
- 编号只对应当前显示列表，搜索后会重新编号，不是持久标识。
- 默认选中上次使用的会话，不修改 tmux 配置。
- 菜单底部显示选中会话的当前工作目录，随选择及刷新更新。它是该会话活动窗口中活动窗格的目录，不是固定的创建目录；长路径会截断，`ck ls` 的最后一列提供完整路径。
- 在普通终端运行会附加会话；在 tmux 内运行会切换会话，不会嵌套 tmux。
- “使用中”表示已有客户端连接；进入不会踢掉已有客户端。
- `q` 只退出菜单。进入会话后，按 tmux 默认快捷键 **Ctrl+b，松开，再按 d** 离开，后台程序继续运行。如果修改过 tmux 前缀，请使用自己的前缀键。
- **结束会话会关闭其中的终端，并可能终止运行中的程序。** 为避免误杀自身，菜单及 `ck kill` 均禁止结束当前所在会话；请在另一个会话或外部终端操作。
- 会话名称不能包含 `.`、`:` 或控制字符。
- 最近会话记录在 `${XDG_STATE_HOME:-~/.local/state}/ck/last.json`，按 tmux 服务器分别保存。通过菜单改名时会同步更新最近记录；会话被外部工具重命名或删除后，旧记录可能失效，请重新选择。
- 正常退出、Ctrl+c 或收到正常终止信号时，界面库会恢复终端状态。不要用 `kill -9` 强制结束交互程序；如果终端状态异常，可以运行 `reset`。

## 在线安装（无需 Go）

支持 Linux/macOS 的 amd64（x86_64）和 arm64。运行时需要先安装 tmux；下载时需要 curl 和系统的 SHA-256 工具（Linux 通常为 sha256sum，macOS 为 shasum）。

下载脚本后执行；建议先查看脚本内容，不要以 root 身份运行：

```bash
ck_setup=$(mktemp -d)
curl --proto '=https' --tlsv1.2 -fSL https://raw.githubusercontent.com/xiangshuhub/ck/main/install.sh -o "$ck_setup/install.sh"
less "$ck_setup/install.sh"
bash "$ck_setup/install.sh"
```

脚本自动识别系统和架构，下载 [GitHub Release](https://github.com/xiangshuhub/ck/releases/latest) 中的二进制，并核对同一 Release 的 `SHA256SUMS`；下载或校验失败不会覆盖已有文件。校验用于检测损坏，不是独立的发布者签名。

更新已有版本：

```bash
ck_setup=$(mktemp -d)
curl --proto '=https' --tlsv1.2 -fSL https://raw.githubusercontent.com/xiangshuhub/ck/main/install.sh -o "$ck_setup/install.sh"
bash "$ck_setup/install.sh" --force
```

固定版本或指定安装目录：

```bash
CK_VERSION=v1.1.0 bash "$ck_setup/install.sh" --force
bash "$ck_setup/install.sh" --force "$HOME/.local/bin"
```

默认安装到 `~/.local/bin/ck`。若提示找不到 `ck`，当前终端执行：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

需要永久生效时，把这行加入所用 shell 的配置文件（如 `~/.bashrc` 或 `~/.zshrc`）。

### 在线卸载

```bash
ck_remove=$(mktemp -d)
curl --proto '=https' --tlsv1.2 -fSL https://raw.githubusercontent.com/xiangshuhub/ck/main/uninstall.sh -o "$ck_remove/uninstall.sh"
less "$ck_remove/uninstall.sh"
bash "$ck_remove/uninstall.sh"
```

默认输入 `yes` 确认。自动化卸载或自定义目录：

```bash
bash "$ck_remove/uninstall.sh" --yes
bash "$ck_remove/uninstall.sh" --yes "$HOME/.local/bin"
```

只删除 `ck`，不影响 tmux 会话或最近使用记录。安装/卸载完成后可删除下载的两个脚本。

## 本地安装已编译版本

构建文件在 `bin/ck`。安装脚本将它复制为独立可执行文件，不创建符号链接，之后移动或删除项目目录也不影响使用。

```bash
./install.sh                  # 安装到 ~/.local/bin/ck
./install.sh --force          # 确认已有 ck 是自己的旧版本后，用此命令更新
./install.sh /其他/安装目录    # 可指定目录，无须 sudo（需要目录写权限）
```

默认拒绝覆盖已有文件；`--force` 使用临时文件和重命名更新，也能替换旧符号链接而不覆盖其指向的文件。

确保安装目录在 `PATH` 中。脚本不修改 shell 配置。也可以直接执行 `./bin/ck`。
安装、卸载脚本不调用 Go 或 Python；本地有 `bin/ck` 时优先使用它，否则走在线下载。

二进制需要与系统/架构匹配。Release 提供 Linux/macOS 的 amd64 和 arm64 版本。

### 卸载

```bash
./uninstall.sh                       # 默认目录，输入 yes 确认
./uninstall.sh --yes                  # 跳过确认
./uninstall.sh --yes /其他/安装目录   # 对应自定义安装目录
```

只删除指定目录内的 `ck` 文件，不结束 tmux 会话，不删除最近使用记录。不再保留项目目录时，也可以直接删除 `~/.local/bin/ck`。

## 从源码构建

```bash
git clone https://github.com/xiangshuhub/ck.git
cd ck
```

仅开发/编译时需要 Go 1.24+，首次构建需要下载 `go.mod` / `go.sum` 锁定的依赖。
界面使用 Bubble Tea，中文字符宽度使用 go-runewidth；它们都编译进可执行文件，不是运行时外部依赖。

```bash
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/ck .
./bin/ck --version
```

交叉编译示例：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o bin/ck-linux-arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o bin/ck-macos-arm64 .
```

更新时先构建到临时文件，再重命名并重新安装，避免直接覆盖正在运行的二进制：

```bash
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/ck.next .
mv bin/ck.next bin/ck
./install.sh --force
```

## 测试

```bash
go test -v ./...
go vet ./...
go test -race ./... # 竞态检测需要支持的平台、CGO 和 C 编译器，仅开发时需要
```

集成测试依赖 tmux，使用随机命名、忽略用户配置的独立 tmux 服务器，并通过伪终端验证真实交互，不会操作现有会话。测试结束会清理自己的服务器。无需 Python。

## 文件

- `main.go`：命令行入口
- `ui.go`：交互菜单
- `tmux.go`：tmux 命令封装
- `state.go`：最近会话记录
- `*_test.go`：单元、真实终端交互及安装/卸载测试
- `install.sh` / `uninstall.sh`：独立安装与卸载
