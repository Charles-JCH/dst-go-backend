#!/usr/bin/env bash
# 在平台服务器上执行：解压发布包，包里有什么就更新什么，由 deploy.sh 和 GitHub Actions 调用
#
# 用法：update.sh <发布包路径>
#
# 发布包内容（都可以缺省）：
#   game-panel                 平台程序，更新后重启 game-panel 服务
#   dst.sh                     安装 Agent 时上传到用户服务器的脚本
#   agent/                     Agent 程序 dst-agent-linux-amd64、dst-agent-linux-arm64
#   web/                       前端页面
#
# config/config.yaml 和 storage/data/ 只在服务器上维护，本脚本不会改动。
set -euo pipefail

# root 直接执行，其他用户通过免密 sudo 重新执行本脚本
if [[ "$(id -u)" != 0 ]]; then
  exec sudo -n bash "$0" "$@"
fi

APP=/opt/game-panel
PKG="${1:?用法: update.sh <发布包路径>}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP" "$PKG"' EXIT
tar -xzf "$PKG" -C "$TMP"

if [[ -f "$TMP/game-panel" && ! -f "$APP/config/config.yaml" ]]; then
  echo "缺少 $APP/config/config.yaml，请先在服务器上创建配置文件" >&2
  exit 1
fi

if [[ -f "$TMP/dst.sh" ]]; then
  mkdir -p "$APP/scripts"
  install -m 0755 "$TMP/dst.sh" "$APP/scripts/dst.sh"
fi

if [[ -d "$TMP/agent" ]]; then
  mkdir -p "$APP/storage/agent"
  install -m 0644 "$TMP"/agent/* "$APP/storage/agent/"
fi

# 前端整体替换，避免残留旧版本的文件
if [[ -d "$TMP/web" ]]; then
  rm -rf "$APP/web.new" "$APP/web.old"
  cp -r "$TMP/web" "$APP/web.new"
  chmod -R a+rX "$APP/web.new"
  if [[ -d "$APP/web" ]]; then
    mv "$APP/web" "$APP/web.old"
  fi
  mv "$APP/web.new" "$APP/web"
  rm -rf "$APP/web.old"
  echo "前端已更新"
fi

if [[ -f "$TMP/game-panel" ]]; then
  # 先写成 .new 再改名：直接覆盖正在运行的程序会报 Text file busy
  install -m 0755 "$TMP/game-panel" "$APP/game-panel.new"
  mv -f "$APP/game-panel.new" "$APP/game-panel"

  systemctl restart game-panel
  sleep 2
  if ! systemctl is-active --quiet game-panel; then
    echo "game-panel 启动失败，最近日志：" >&2
    journalctl -u game-panel -n 30 --no-pager >&2
    exit 1
  fi
  echo "game-panel 已更新并启动"
fi
