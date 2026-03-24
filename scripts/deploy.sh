#!/usr/bin/env bash
# =============================================================
#  deploy.sh  —  部署腳本
#
#  用法：
#    ./scripts/deploy.sh [environment]
#
#  environment 可選值：
#    production   (預設)
#    development
#
#  範例：
#    ./scripts/deploy.sh production
#    ./scripts/deploy.sh development
#
#  前置作業：
#    確認 docker/envs/.env.<environment> 存在
#    可從範本建立：
#      cp docker/envs/.env.production.example docker/envs/.env.production
# =============================================================
set -euo pipefail

# ── 路徑設定 ──────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# ── 參數解析 ──────────────────────────────────────────────────
ENV="${1:-production}"
SRC_ENV="$ROOT_DIR/docker/envs/.env.$ENV"
DOCKER_ENV="$ROOT_DIR/docker/.env"
BACKEND_ENV="$ROOT_DIR/backend/.env"

# ── 驗證來源環境檔存在 ────────────────────────────────────────
if [[ ! -f "$SRC_ENV" ]]; then
  echo ""
  echo "❌  找不到環境設定檔：$SRC_ENV"
  echo ""
  echo "   請先從範本建立並填入正確值："
  echo "   cp $ROOT_DIR/docker/envs/.env.$ENV.example $SRC_ENV"
  echo ""
  exit 1
fi

echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║              部署啟動                        ║"
echo "╚══════════════════════════════════════════════╝"
echo "  環境：$ENV"
echo "  時間：$(date '+%Y-%m-%d %H:%M:%S')"
echo ""

# ── 步驟 1：複製 env 到 docker/.env（若已存在則略過）─────────
if [[ -f "$DOCKER_ENV" ]]; then
  echo "  ℹ️   docker/.env 已存在，略過複製（保留現有設定）"
  echo "      如需重新套用請先刪除：rm $DOCKER_ENV"
else
  cp "$SRC_ENV" "$DOCKER_ENV"
  echo "  ✅  環境設定已複製"
  echo "      $SRC_ENV"
  echo "      → $DOCKER_ENV"
fi

# ── 步驟 2：同步 backend/.env（供本機 go run 使用）───────────
grep -E '^(PORT|DB_HOST|DB_PORT|DB_USER|DB_PASS|DB_NAME|BACKEND_INTERNAL_PORT)=' \
  "$DOCKER_ENV" > "$BACKEND_ENV" || true
# container 外本機連 DB 用 localhost
if ! grep -q '^DB_HOST=' "$BACKEND_ENV"; then
  echo "DB_HOST=localhost" >> "$BACKEND_ENV"
fi
# PORT 對齊 BACKEND_INTERNAL_PORT
if ! grep -q '^PORT=' "$BACKEND_ENV"; then
  INTERNAL_PORT="$(grep -E '^BACKEND_INTERNAL_PORT=' "$DOCKER_ENV" | cut -d= -f2 || echo '8080')"
  echo "PORT=${INTERNAL_PORT}" >> "$BACKEND_ENV"
fi
echo "  ✅  後端變數已同步 → $BACKEND_ENV"

# ── 步驟 3：啟動 Docker Compose ──────────────────────────────
echo ""
echo "  🐳  啟動 Docker Compose（$ENV）…"
echo ""
cd "$ROOT_DIR/docker"
docker compose pull --quiet
docker compose up -d --build --remove-orphans

echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║              部署完成 ✅                      ║"
echo "╚══════════════════════════════════════════════╝"
echo ""
echo "  環境：$ENV"
echo "  完成：$(date '+%Y-%m-%d %H:%M:%S')"
echo ""

# ── 顯示服務狀態 ──────────────────────────────────────────────
docker compose ps
echo ""

