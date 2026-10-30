#!/bin/sh
# Install a standalone local build, or download and verify a GitHub Release.
set -eu
force=false
dest=${HOME:?}/.local/bin
if [ "${1-}" = "--force" ]; then force=true; shift; fi
if [ "$#" -gt 1 ] || [ "${1-}" = "--help" ]; then
    printf '用法: %s [--force] [安装目录]\n默认安装到 ~/.local/bin；覆盖已有 ck 需要 --force。\n没有本地 bin/ck 时下载 GitHub Release；CK_VERSION 可指定标签。\n' "$0"
    [ "${1-}" = "--help" ] && exit 0
    exit 1
fi
if [ "$#" -eq 1 ]; then
    case "$1" in ''|-*) printf '无效安装目录: %s\n' "$1" >&2; exit 1 ;; esac
    dest=$1
fi
case "$dest" in /*) ;; *) dest=$PWD/$dest ;; esac
root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
source=$root/bin/ck
target=$dest/ck
if [ -d "$target" ]; then
    printf '拒绝覆盖目录: %s\n' "$target" >&2
    exit 1
fi
if { [ -e "$target" ] || [ -L "$target" ]; } && [ "$force" != true ]; then
    printf '目标已存在: %s\n确认它是旧版 ck 后，使用 --force 更新。\n' "$target" >&2
    exit 1
fi
temp=''
download=''
cleanup() {
    if [ -n "$temp" ]; then rm -f -- "$temp"; fi
    if [ -n "$download" ]; then rm -rf -- "$download"; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if [ ! -f "$source" ] || [ ! -x "$source" ]; then
    command -v curl >/dev/null 2>&1 || { printf '在线安装需要 curl。\n' >&2; exit 1; }
    case "$(uname -s)" in
        Linux) platform=linux ;;
        Darwin) platform=darwin ;;
        *) printf '暂不支持此系统，仅支持 Linux/macOS。\n' >&2; exit 1 ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) printf '暂不支持此架构，仅支持 amd64/arm64。\n' >&2; exit 1 ;;
    esac
    if command -v sha256sum >/dev/null 2>&1; then
        hash=sha256sum
    elif command -v shasum >/dev/null 2>&1; then
        hash=shasum
    else
        printf '缺少 SHA-256 校验工具，需要 sha256sum 或 shasum。\n' >&2
        exit 1
    fi
    version=${CK_VERSION:-latest}
    case "$version" in
        latest) base=https://github.com/xiangshuhub/ck/releases/latest/download ;;
        v[0-9]*)
            case "$version" in *[!a-zA-Z0-9._-]*) printf '无效 CK_VERSION\n' >&2; exit 1 ;; esac
            base=https://github.com/xiangshuhub/ck/releases/download/$version ;;
        *) printf 'CK_VERSION 应为 latest 或 v 开头的版本标签。\n' >&2; exit 1 ;;
    esac
    asset=ck_${platform}_${arch}
    download=$(mktemp -d "${TMPDIR:-/tmp}/ck-download.XXXXXX")
    printf '下载 %s (%s)...\n' "$asset" "$version"
    curl --proto '=https' --tlsv1.2 -fSL --retry 3 "$base/$asset" -o "$download/$asset"
    curl --proto '=https' --tlsv1.2 -fSL --retry 3 "$base/SHA256SUMS" -o "$download/SHA256SUMS"
    expected=$(awk -v name="$asset" '$2 == name { print $1 }' "$download/SHA256SUMS")
    case "$expected" in ''|*[!a-fA-F0-9]*) printf '校验文件无效或缺少对应架构。\n' >&2; exit 1 ;; esac
    if [ "${#expected}" -ne 64 ]; then printf 'SHA-256 长度无效。\n' >&2; exit 1; fi
    if [ "$hash" = sha256sum ]; then
        actual=$(sha256sum "$download/$asset" | awk '{print $1}')
    else
        actual=$(shasum -a 256 "$download/$asset" | awk '{print $1}')
    fi
    if [ "$actual" != "$expected" ]; then
        printf 'SHA-256 校验失败，未安装，已有文件保持不变。\n' >&2
        exit 1
    fi
    source=$download/$asset
fi
mkdir -p -- "$dest"
temp=$(mktemp "$dest/.ck-install.XXXXXX")
cp -- "$source" "$temp"
chmod 755 "$temp"
# Renaming replaces the symlink itself, never the file it points to.
mv -f -- "$temp" "$target"
printf '已独立安装: %s\n无需保留项目目录。请确保安装目录在 PATH 中。\n' "$target"
if ! command -v tmux >/dev/null 2>&1; then
    printf '提示：运行 ck 前请先安装 tmux。\n'
fi
