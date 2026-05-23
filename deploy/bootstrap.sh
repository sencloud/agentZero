#!/usr/bin/env bash
# 一次性初始化脚本：在阿里云 ECS（47.96.115.180）上准备 Docker 环境
# 用法：scp deploy/bootstrap.sh root@47.96.115.180:/root/ && ssh root@47.96.115.180 'bash /root/bootstrap.sh'

set -euo pipefail

echo "[1/4] 更新系统并安装基础工具..."
if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y ca-certificates curl gnupg lsb-release
elif command -v yum >/dev/null 2>&1; then
    yum install -y ca-certificates curl yum-utils
fi

echo "[2/4] 安装 Docker..."
if ! command -v docker >/dev/null 2>&1; then
    # 阿里云镜像 + 显式安装清单，避开 docker-model-plugin 这种 Ubuntu 20.04 上还没有的可选包
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    CODENAME=$(. /etc/os-release && echo "$VERSION_CODENAME")
    echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://mirrors.aliyun.com/docker-ce/linux/ubuntu ${CODENAME} stable" \
        > /etc/apt/sources.list.d/docker.list
    apt-get update -y
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
systemctl enable --now docker

echo "[3/4] 校验 docker compose 插件..."
docker compose version

echo "[4/4] 准备目录结构..."
mkdir -p /opt/agentzero
chown root:root /opt/agentzero
chmod 750 /opt/agentzero

echo "完成。后续 CI 会把 docker-compose.yml / Caddyfile / .env 推到 /opt/agentzero/。"
