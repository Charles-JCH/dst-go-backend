#!/usr/bin/env bash
# 打包前后端并更新到平台服务器（Linux / macOS / Windows Git Bash 均可运行）
#
# 用法：deploy/deploy.sh root@43.155.211.217
#
# 服务器上的更新步骤见 deploy/update.sh。目录结构（WorkingDirectory=/opt/game-panel）：
#   game-panel                 平台程序
#   config/config.yaml         配置文件，含密钥，只在服务器上维护，本脚本不会上传或覆盖
#   scripts/dst.sh             安装 Agent 时上传到用户服务器的脚本
#   storage/agent/             Agent 程序 dst-agent-linux-amd64、dst-agent-linux-arm64
#   storage/data/              SQLite 数据库，本脚本不会改动
#   web/                       前端页面，由 Nginx 提供
#
# 建议先配置 SSH 密钥登录（ssh-copy-id），否则上传和更新各要输入一次密码。
set -euo pipefail

HOST="${1:-}"
if [[ -z "$HOST" ]]; then
  echo "用法: $0 <用户@服务器地址>" >&2
  exit 1
fi

BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="${FRONTEND_DIR:-$BACKEND_DIR/../../frontend/dst-go-frontend}"
STAGE="$(mktemp -d)"
OUT="$(mktemp -d)"
PACKAGE="$OUT/game-panel-deploy.tar.gz"
trap 'rm -rf "$STAGE" "$OUT"' EXIT

echo "==> 构建前端"
(cd "$FRONTEND_DIR" && npm run build)
cp -r "$FRONTEND_DIR/dist" "$STAGE/web"

echo "==> 构建平台和 Agent"
mkdir -p "$STAGE/agent"
(
  cd "$BACKEND_DIR"
  export CGO_ENABLED=0 GOOS=linux
  GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$STAGE/game-panel" ./cmd/server
  for arch in amd64 arm64; do
    GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$STAGE/agent/dst-agent-linux-$arch" ./cmd/agent
  done
)
cp "$BACKEND_DIR/scripts/dst.sh" "$STAGE/dst.sh"

echo "==> 上传到 $HOST"
tar -czf "$PACKAGE" -C "$STAGE" .
scp "$PACKAGE" "$BACKEND_DIR/deploy/update.sh" "$HOST:/tmp/"

echo "==> 更新并重启"
ssh "$HOST" bash /tmp/update.sh /tmp/game-panel-deploy.tar.gz

echo "==> 部署完成"
