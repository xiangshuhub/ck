#!/bin/sh
# Only remove the explicitly selected executable; never touch sessions or state.
set -eu
yes=false
dest=${HOME:?}/.local/bin
if [ "${1-}" = "--yes" ]; then yes=true; shift; fi
if [ "$#" -gt 1 ] || [ "${1-}" = "--help" ]; then
    printf '用法: %s [--yes] [安装目录]\n默认卸载 ~/.local/bin/ck；--yes 跳过确认。\n' "$0"
    [ "${1-}" = "--help" ] && exit 0
    exit 1
fi
if [ "$#" -eq 1 ]; then
    case "$1" in ''|-*) printf '无效安装目录: %s\n' "$1" >&2; exit 1 ;; esac
    dest=$1
fi
case "$dest" in /*) ;; *) dest=$PWD/$dest ;; esac
target=$dest/ck
if [ -d "$target" ]; then
    printf '拒绝删除目录: %s\n' "$target" >&2
    exit 1
fi
if [ ! -e "$target" ] && [ ! -L "$target" ]; then
    printf '未安装: %s\n' "$target"
    exit 0
fi
if [ "$yes" != true ]; then
    printf '仅删除 %s，不影响 tmux 会话或使用记录。输入 yes 确认: ' "$target"
    answer=''
    if ! IFS= read -r answer || [ "$answer" != yes ]; then
        printf '已取消\n'
        exit 0
    fi
fi
rm -f -- "$target"
printf '已卸载: %s\n' "$target"
