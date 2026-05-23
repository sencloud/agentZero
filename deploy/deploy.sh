#!/usr/bin/env bash
# 本地一键部署：rsync 代码到服务器 → 服务器上 docker compose build && up
#
# 用法：
#   ./deploy/deploy.sh              # 部署到默认服务器
#   HOST=47.96.115.180 ./deploy/deploy.sh
#   SKIP_BUILD=1 ./deploy/deploy.sh # 跳过 docker build（只刷 Caddyfile/compose）
#
# 前置依赖（本机）：
#   - .local-secrets/server.env                 # 含 JWT_SECRET / DEEPSEEK_API_KEY 等
#   - .local-secrets/agentzero_deploy_ed25519   # 部署用 SSH 私钥
#   - rsync, ssh, scp

set -euo pipefail

# ──────────── 配置 ────────────
HOST="${HOST:-47.96.115.180}"
SSH_USER="${SSH_USER:-root}"
SSH_KEY="${SSH_KEY:-.local-secrets/agentzero_deploy_ed25519}"
REMOTE_DIR="${REMOTE_DIR:-/opt/agentzero}"
ENV_FILE="${ENV_FILE:-.local-secrets/server.env}"
DOMAIN="${AGENTZERO_DOMAIN:-47.96.115.180.nip.io}"
SKIP_BUILD="${SKIP_BUILD:-0}"

# 解析项目根（脚本可在任意 cwd 调用）
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

bold() { printf "\033[1m%s\033[0m\n" "$*"; }
red()  { printf "\033[31m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
gray() { printf "\033[2m%s\033[0m\n" "$*"; }

# ──────────── 校验本地依赖 ────────────
bold "[1/6] 校验本地依赖"

if [[ ! -f "$ENV_FILE" ]]; then
  red "缺少 $ENV_FILE"
  echo "    用 deploy/.env.example 当模板，复制后填好敏感值再跑。"
  echo "    例: cp deploy/.env.example $ENV_FILE && \$EDITOR $ENV_FILE"
  exit 1
fi
if [[ ! -f "$SSH_KEY" ]]; then
  red "缺少 SSH 私钥 $SSH_KEY"
  exit 1
fi
chmod 600 "$SSH_KEY"

for cmd in rsync ssh scp; do
  command -v "$cmd" >/dev/null || { red "缺少命令: $cmd"; exit 1; }
done

# 必填项校验：JWT_SECRET、DEEPSEEK_API_KEY 不能空
# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a
[[ -z "${JWT_SECRET:-}" || "$JWT_SECRET" == "please-change-me-in-production" ]] \
  && { red "$ENV_FILE 里 JWT_SECRET 未设置或仍是占位符"; exit 1; }
[[ -z "${DEEPSEEK_API_KEY:-}" ]] && { red "$ENV_FILE 里 DEEPSEEK_API_KEY 必填"; exit 1; }

gray "  HOST=$HOST  REMOTE_DIR=$REMOTE_DIR  DOMAIN=$DOMAIN  SKIP_BUILD=$SKIP_BUILD"

# ──────────── SSH 选项 ────────────
SSH_OPTS=(-i "$SSH_KEY" -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=~/.ssh/known_hosts)
SSH="ssh ${SSH_OPTS[*]} ${SSH_USER}@${HOST}"

# ──────────── 准备远端目录 ────────────
bold "[2/6] 准备远端目录 $REMOTE_DIR"
$SSH "mkdir -p $REMOTE_DIR"

# ──────────── rsync 源码 + 部署清单 ────────────
bold "[3/6] 同步代码"
RSYNC_OPTS=(-az --delete \
  --exclude='.git/' \
  --exclude='.local-secrets/' \
  --exclude='node_modules/' \
  --exclude='app/' \
  --exclude='.dart_tool/' \
  --exclude='**/data/' \
  --exclude='**/build/' \
  --exclude='**/*.db' \
  --exclude='**/*.db-journal' \
  --exclude='**/.env')

rsync "${RSYNC_OPTS[@]}" \
  -e "ssh ${SSH_OPTS[*]}" \
  server/ \
  "${SSH_USER}@${HOST}:${REMOTE_DIR}/server/"

rsync -az -e "ssh ${SSH_OPTS[*]}" \
  deploy/docker-compose.yml \
  deploy/Caddyfile \
  "${SSH_USER}@${HOST}:${REMOTE_DIR}/"

# ──────────── 推送 .env ────────────
bold "[4/6] 推送 .env"
# 服务器上 .env 严格 600，且不让 rsync 误删
TMP_ENV="$(mktemp)"
trap 'rm -f "$TMP_ENV"' EXIT
{
  cat "$ENV_FILE"
  # 部署脚本统一注入域名（避免 .env 文件忘了写）
  if ! grep -q '^AGENTZERO_DOMAIN=' "$ENV_FILE"; then
    echo "AGENTZERO_DOMAIN=$DOMAIN"
  fi
} > "$TMP_ENV"
scp "${SSH_OPTS[@]}" -q "$TMP_ENV" "${SSH_USER}@${HOST}:${REMOTE_DIR}/.env"
$SSH "chmod 600 ${REMOTE_DIR}/.env"

# ──────────── 构建 & 启动 ────────────
bold "[5/6] 远端 build + up"
if [[ "$SKIP_BUILD" == "1" ]]; then
  $SSH "cd $REMOTE_DIR && docker compose up -d --remove-orphans"
else
  $SSH "cd $REMOTE_DIR && docker compose build api && docker compose up -d --remove-orphans"
fi

# ──────────── 健康检查 ────────────
bold "[6/6] 健康检查"
healthy=0
for i in 1 2 3 4 5 6 7 8 9 10; do
  if $SSH "docker exec agentzero-api wget -qO- http://127.0.0.1:8080/healthz" 2>/dev/null | grep -q '"status":"ok"'; then
    green "✓ 容器内 /healthz 通过"
    healthy=1
    break
  fi
  gray "  容器 healthz 第 $i 次未就绪，2s 后重试"
  sleep 2
done
[[ $healthy -ne 1 ]] && { red "✗ 容器健康检查失败，查 docker logs agentzero-api"; exit 1; }

# 公网 HTTPS healthz（依赖 Caddy 拿到 Let's Encrypt 证书；首次部署可能需要 1-2 分钟）
public_ok=0
for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
  if curl -fsS --max-time 5 "https://${DOMAIN}/healthz" >/dev/null 2>&1; then
    green "✓ 公网 https://${DOMAIN}/healthz 通过"
    public_ok=1
    break
  fi
  gray "  公网 healthz 第 $i 次未就绪，5s 后重试"
  sleep 5
done
[[ $public_ok -ne 1 ]] && {
  red "✗ 公网 https 暂时不可达"
  gray "  Caddy 拿证书可能还在路上，可稍后手动 curl 验证；或 ssh 进服务器看 docker logs agentzero-caddy"
}

bold "部署完成"
echo "  API:    https://${DOMAIN}"
echo "  容器:    ${SSH} 'docker compose -f ${REMOTE_DIR}/docker-compose.yml ps'"
echo "  日志:    ${SSH} 'docker logs -f --tail=100 agentzero-api'"
