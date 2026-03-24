# CI/CD 說明文件

本專案使用 GitHub Actions 進行持續整合與持續部署。

---

## 流程概覽

```
push / PR
    │
    ▼
[CI] ci.yml ──── Backend  : go vet → go build → go test
                 Frontend : bun install → typecheck → bun build
                 Docker   : docker compose build（不推送）

push to master/main
    │
    ▼
[CD] deploy.yml ─ validate compose config
                  SSH → git reset --hard
                  確認 docker/envs/.env.production 存在
                  bash scripts/deploy.sh production
```

---

## CI（ci.yml）

**觸發條件**
- 所有 `push`（排除 `dependabot/**`）
- PR 至 `master`、`main`、`feat/**`、`fix/**`

**Jobs**

| Job | 說明 | 工作目錄 |
|-----|------|---------|
| `Backend (Go)` | `go vet` + `go build` + `go test` | `backend/` |
| `Frontend (Nuxt / Bun)` | `bun install` + `nuxi typecheck` + `bun build` | `frontend/` |
| `Docker Compose Build` | 用 `docker/envs/.env.development.example` 建立 `docker/.env`，執行 `docker compose build` | `docker/` |

**不需要任何 Secret。**

---

## CD（deploy.yml）

**觸發條件**
- CI (`CI — Lint & Build`) 在 `master` 或 `main` 上**成功完成**後自動觸發
- 手動觸發（`workflow_dispatch`），可選擇 `production` 或 `development`
- CI 失敗時**不會**執行部署

**執行環境**：GitHub Environment `production`（可在 Settings → Environments 設定 Protection rules）

**步驟**

1. Checkout 程式碼
2. 用 `docker/envs/.env.production.example` 建立 `docker/.env`，驗證 compose config 語法
3. SSH 到伺服器：
   - `git fetch --all && git reset --hard origin/<branch>`
   - 確認 `docker/envs/.env.production` 存在（否則報錯退出）
   - `bash scripts/deploy.sh production`

---

## 必要的 GitHub Secrets

在 **Settings → Secrets and variables → Actions → Repository secrets** 新增：

| Secret 名稱 | 說明 | 範例值 |
|------------|------|--------|
| `DEPLOY_HOST` | 伺服器 IP 或 hostname | `123.456.78.90` |
| `DEPLOY_USER` | SSH 登入使用者名稱 | `deploy` 或 `ubuntu` |
| `DEPLOY_SSH_KEY` | SSH **私鑰**（PEM 格式完整內容） | `-----BEGIN OPENSSH PRIVATE KEY-----...` |
| `DEPLOY_PORT` | SSH port（選填，預設 22） | `22` |
| `DEPLOY_PATH` | 伺服器上的專案根目錄絕對路徑 | `/srv/34_stock-new` |

> **注意**：`DEPLOY_PORT` 若不設定，workflow 會 fallback 到 `22`（`${{ secrets.DEPLOY_PORT || 22 }}`）。

### 取得 SSH 私鑰

```bash
# 在本機產生部署用 key pair（建議獨立一組，不要用個人 key）
ssh-keygen -t ed25519 -C "github-deploy" -f ~/.ssh/deploy_key

# 將公鑰加到伺服器
ssh-copy-id -i ~/.ssh/deploy_key.pub <USER>@<HOST>

# 複製私鑰內容 → 貼到 GitHub Secret DEPLOY_SSH_KEY
cat ~/.ssh/deploy_key
```

---

## 伺服器首次設定

在伺服器上執行一次（之後 CD 會自動維護）：

```bash
# 1. Clone 專案
git clone <repo-url> /srv/34_stock-new
cd /srv/34_stock-new

# 2. 建立 production 環境設定
cp docker/envs/.env.production.example docker/envs/.env.production
nano docker/envs/.env.production
# 建議至少修改：DB_PASS、PGADMIN_PASSWORD
# 或留預設值，deploy.sh 會自動產生隨機密碼

# 3. 首次手動部署
bash scripts/deploy.sh production
```

---

## 手動觸發部署

1. 前往 GitHub Repo → **Actions** → **CD — Deploy to Production**
2. 點選 **Run workflow**
3. 選擇 `production` 或 `development`
4. 點 **Run workflow** 確認

---

## 常見問題

### Secret 未設定導致 SSH 連線失敗
```
Error: Input required and not supplied: host
```
→ 確認 `DEPLOY_HOST`、`DEPLOY_USER`、`DEPLOY_SSH_KEY` 都已設定

### 伺服器找不到 env 檔
```
❌ 找不到 docker/envs/.env.production
```
→ SSH 進伺服器，執行：
```bash
cp docker/envs/.env.production.example docker/envs/.env.production
```

### Docker Compose Build 失敗
→ CI 的 docker-build job 用 `development.example` 建置，若新增 service 或修改 compose 後記得確認 example 檔有同步更新

### 想重新套用 env（強制覆蓋現有 docker/.env）
```bash
rm docker/.env
bash scripts/deploy.sh production
```
