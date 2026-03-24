---
name: deploy
description: "部署、CI/CD、Docker 環境管理。Use when: 新增 service、修改部署流程、更新 CI/CD workflow、管理多環境 .env、使用 deploy.sh、設定 GitHub Actions secrets、排查部署問題。Covers: scripts/deploy.sh、docker/ env 目錄、.github/workflows/ci.yml、.github/workflows/deploy.yml。"
argument-hint: "操作類型 (deploy | env | cicd | secret | nginx | pgadmin | phpmyadmin)"
---

# Deploy — 部署系統知識庫

統一的部署腳本架構，多環境 env 集中存放於 `docker/envs/`，`docker-compose.yml` 位於 `docker/`，nginx 統一對外單一 port，CI/CD 透過 GitHub Actions 自動化。

---

## 目錄結構

```
project-root/
├── docker/
│   ├── docker-compose.yml         ← 所有容器定義（build context 用 ../）
│   ├── .env                       ← 由 deploy.sh 自動產生（gitignored）
│   ├── nginx/
│   │   └── nginx.conf             ← nginx 反向代理設定
│   └── envs/
│       ├── .env.production        ← 正式環境實際值（gitignored）
│       ├── .env.development       ← 開發環境實際值（gitignored）
│       ├── .env.production.example  ← 正式環境範本（committed）
│       └── .env.development.example ← 開發環境範本（committed）
│
├── scripts/
│   └── deploy.sh                  ← 部署主腳本（chmod +x）
│
├── .github/
│   └── workflows/
│       ├── ci.yml                 ← CI：lint / build / docker-build
│       └── deploy.yml             ← CD：SSH 部署到伺服器
│
└── backend/
    └── .env                      ← 由 deploy.sh 自動同步（gitignored）
```

> **重要**：根目錄不再有 `docker-compose.yml` 或 `.env`，一律使用 `docker/` 目錄下的檔案。

---

## 服務架構（Port 路由）

```
外部請求
    │
    ▼  APP_PORT（預設 80）
 nginx
    ├── /pgadmin/     →  pgadmin:5050      (PostgreSQL 用 pgAdmin 4)
    ├── /phpmyadmin/  →  phpmyadmin:80     (MySQL/MariaDB 用 phpMyAdmin)
    └── /             →  frontend:3000     (Nuxt，/api/ 由 Nuxt server 內部代理到 backend:8080)

直接存取（DBA 工具）
    DB_PORT_EXPOSED  →  postgres:5432 / mysql:3306
```

| 服務 | 容器 Port | 對外方式 |
|------|-----------|----------|
| nginx | 80 | `APP_PORT`（host 唯一入口） |
| frontend (Nuxt) | 3000 | 只透過 nginx |
| backend (Go) | 8080 | 只透過 Nuxt/nginx，不直接對外 |
| pgadmin *(PostgreSQL)* | 5050 | 只透過 nginx `/pgadmin/` |
| phpmyadmin *(MySQL)* | 80 | 只透過 nginx `/phpmyadmin/` |
| postgres | 5432 | `DB_PORT_EXPOSED`（DBA 工具用） |
| mysql / mariadb | 3306 | `DB_PORT_EXPOSED`（DBA 工具用） |

---

## 快速部署

```bash
# 1. 從範本建立環境設定（每個環境只需一次）
cp docker/envs/.env.production.example docker/envs/.env.production
# 編輯 docker/envs/.env.production，填入 DB_PASS 等正式值

# 2. 部署
./scripts/deploy.sh production

# 開發環境
cp docker/envs/.env.development.example docker/envs/.env.development
./scripts/deploy.sh development
```

### 存取地址

| 功能 | URL |
|------|-----|
| 主應用程式 | `http://host:APP_PORT/` |
| pgAdmin *(PostgreSQL)* | `http://host:APP_PORT/pgadmin/` |
| phpMyAdmin *(MySQL)* | `http://host:APP_PORT/phpmyadmin/` |
| 資料庫（直接） | `host:DB_PORT_EXPOSED` |

---

## deploy.sh 運作原理

```
docker/envs/.env.<env>
        │
        │  ① 密碼安全性檢查（若仍為預設值則自動替換）
        │
        │  ② 若 docker/.env 不存在才複製
        ▼
    docker/.env  ◄── docker compose 從此讀取（env_file + 變數插值）
        │
        └── 提取後端變數 → backend/.env  ◄── go run main.go 使用
```

### 步驟說明

| 步驟 | 操作 | 說明 |
|------|------|------|
| 1 | 驗證來源 | 確認 `docker/envs/.env.<env>` 存在 |
| 2 | **密碼安全性** | 若 `DB_PASS` / `PGADMIN_PASSWORD` / `MYSQL_ROOT_PASSWORD` 仍為 example 預設值 → 自動替換為 8 碼隨機密碼 |
| 3 | 複製 env | 若 `docker/.env` **不存在**才從來源複製；已存在則略過 |
| 4 | 同步 backend/.env | 從 `docker/.env` 提取後端變數 |
| 5 | docker compose | `cd docker && docker compose pull && up -d --build` |

> **「存在則不複製」的用意**：伺服器上的 `docker/.env` 可能被手動調整，deploy 時不應覆蓋。如需強制重新套用：`rm docker/.env && ./scripts/deploy.sh production`

---

## 密碼自動輪替

### 觸發條件

`DB_PASS`、`PGADMIN_PASSWORD`、`MYSQL_ROOT_PASSWORD` 的值與 `.env.<env>.example` 的預設值**完全相同**時，自動觸發。

> **PASSWORD_KEYS** 在 `deploy.sh` 中定義，依專案使用的 DB 管理工具加入對應 key：PostgreSQL 加 `PGADMIN_PASSWORD`，MySQL 加 `MYSQL_ROOT_PASSWORD`。

### 行為

- 在 `docker/envs/.env.<env>` 中原地替換為 8 碼隨機密碼（`[A-Za-z0-9]`）
- 若 `docker/.env` 已存在，也對其進行同樣檢查與替換
- 終端輸出提示訊息，但**不顯示**新密碼（安全考量）
- 替換後立即繼續部署，密碼永久儲存於檔案中

### 強制重新產生

```bash
# 回復為 example 預設值後重新部署，即可觸發新的自動輪替
# 或直接手動編輯 docker/envs/.env.production
```

---

## 環境變數清單

定義位置：`docker/envs/.env.<environment>`

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `DB_USER` | PostgreSQL 使用者 | `postgres` |
| `DB_PASS` | PostgreSQL 密碼 | **production 必須修改（或讓 deploy.sh 自動產生）** |
| `DB_NAME` | 資料庫名稱 | `stockdb` |
| `APP_PORT` | nginx 對外 port（Host） | `80` |
| `DB_PORT_EXPOSED` | PostgreSQL 對外 port（Host） | `5432` |
| `BACKEND_INTERNAL_PORT` | backend container 內部 port | `8080` |
| `NUXT_BACKEND_URL` | Nuxt SSR → backend URL（container 內） | `http://backend:8080` |
| `PGADMIN_EMAIL` | pgAdmin 登入 Email | `admin@example.com` |
| `PGADMIN_PASSWORD` | pgAdmin 登入密碼 | **production 必須修改（或讓 deploy.sh 自動產生）** |

> **已移除**：`FRONTEND_PORT`、`BACKEND_PORT`（不再對外暴露，由 nginx 統一處理）

---

## nginx 設定（docker/nginx/nginx.conf）

### 路由規則

| location | proxy_pass | 適用DB | 說明 |
|----------|------------|--------|------|
| `/pgadmin/` | `http://pgadmin/` | PostgreSQL | 加 `X-Script-Name: /pgadmin` header，供 pgadmin 產生正確 URL |
| `/phpmyadmin/` | `http://phpmyadmin/` | MySQL / MariaDB | 加 `PMA_ABSOLUTE_URI` 環境變數確保內部連結正確 |
| `/` | `http://frontend` | — | Nuxt SSR（含 `/api/` 代理） |

### phpMyAdmin nginx location 範例

```nginx
location /phpmyadmin/ {
    proxy_pass         http://phpmyadmin/;
    proxy_set_header   Host            $host;
    proxy_set_header   X-Real-IP       $remote_addr;
    proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header   X-Forwarded-Proto $scheme;
    proxy_redirect     off;
}
```

### SSE / WebSocket 設定

`location /` 區塊已啟用：
- `proxy_buffering off` — SSE 即時串流
- `proxy_http_version 1.1` + `Upgrade` / `Connection` headers — WebSocket 支援
- `proxy_read_timeout 3600s` — 長連線不中斷

### 修改 nginx 設定後重新載入

```bash
cd docker
docker compose exec nginx nginx -s reload
```

---

## 資料庫管理工具

> **選擇規則**：PostgreSQL / TimescaleDB → 使用 **pgAdmin**；MySQL / MariaDB → 使用 **phpMyAdmin**。

### pgAdmin（PostgreSQL / TimescaleDB）

**docker-compose service：**
```yaml
pgadmin:
  image: dpage/pgadmin4:latest
  container_name: stock_pgadmin
  environment:
    PGADMIN_DEFAULT_EMAIL: ${PGADMIN_EMAIL:-admin@example.com}
    PGADMIN_DEFAULT_PASSWORD: ${PGADMIN_PASSWORD:-pgadmin}
    PGADMIN_CONFIG_ENHANCED_COOKIE_PROTECTION: "False"
  volumes:
    - pgadmin_data:/var/lib/pgadmin
  depends_on:
    postgres:
      condition: service_healthy
  networks:
    - stock-net
```

**登入**：
- URL：`http://host:APP_PORT/pgadmin/`
- Email：`PGADMIN_EMAIL`
- Password：`PGADMIN_PASSWORD`（若曾自動輪替請查 `docker/envs/.env.<env>`）

**新增 DB 連線**：

| 欄位 | 值 |
|------|----||
| Host | `postgres` |
| Port | `5432` |
| Database | `DB_NAME` |
| Username | `DB_USER` |
| Password | `DB_PASS` |

**密碼輪替 key**：`PGADMIN_PASSWORD`

---

### phpMyAdmin（MySQL / MariaDB）

**docker-compose service：**
```yaml
phpmyadmin:
  image: phpmyadmin:latest
  container_name: stock_phpmyadmin
  environment:
    PMA_HOST: mysql
    PMA_PORT: 3306
    PMA_ABSOLUTE_URI: http://localhost:${APP_PORT:-80}/phpmyadmin/
  depends_on:
    - mysql
  networks:
    - stock-net
```

> `PMA_ABSOLUTE_URI` 必須帶 `/phpmyadmin/` trailing slash，否則內部頁面連結會跑掉。

**登入**：
- URL：`http://host:APP_PORT/phpmyadmin/`
- Username：`DB_USER` 或 `root`
- Password：`DB_PASS` 或 `MYSQL_ROOT_PASSWORD`

**env 變數（phpMyAdmin 對應 MySQL）**：

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `DB_USER` | MySQL 使用者 | `app` |
| `DB_PASS` | MySQL 使用者密碼 | **必須修改** |
| `MYSQL_ROOT_PASSWORD` | MySQL root 密碼 | **必須修改** |
| `DB_NAME` | 資料庫名稱 | `appdb` |

**密碼輪替 key**：`DB_PASS`、`MYSQL_ROOT_PASSWORD`

---

## docker/docker-compose.yml 注意事項

- **build context** 使用 `../backend`、`../frontend`（相對於 `docker/` 目錄）
- **env_file: - .env** 指向 `docker/.env`（docker compose 在 `docker/` 目錄執行）
- 執行方式：`cd docker && docker compose ...`
- `frontend` 和 `backend` **不暴露** host port，只透過 nginx 存取
- `pgadmin` **不暴露** host port，只透過 nginx `/pgadmin/` 存取
- `phpmyadmin` **不暴露** host port，只透過 nginx `/phpmyadmin/` 存取

---

## GitHub Actions — CI

**檔案**：`.github/workflows/ci.yml`  
**觸發**：所有 push / PR

| Job | 說明 |
|-----|------|
| `backend` | `go vet` + `go build` + `go test` |
| `frontend` | `bun install` + `nuxi typecheck` + `bun run build` |
| `docker-build` | 用 development example 建立 `docker/.env`，進 `docker/` 執行 `docker compose build` |

---

## GitHub Actions — CD

**檔案**：`.github/workflows/deploy.yml`  
**觸發**：push 到 `master` / `main`，或手動 `workflow_dispatch`

流程：
1. Checkout → 用 production example 建立 `docker/.env` → 驗證 compose config
2. SSH 到伺服器：`git reset --hard origin/master`
3. 確認 `docker/envs/.env.production` 存在（需手動在伺服器建立）
4. 執行 `bash scripts/deploy.sh production`

### 必要的 GitHub Secrets

| Secret | 說明 |
|--------|------|
| `DEPLOY_HOST` | 伺服器 IP 或 hostname |
| `DEPLOY_USER` | SSH 使用者名稱 |
| `DEPLOY_SSH_KEY` | SSH 私鑰（PEM 格式） |
| `DEPLOY_PORT` | SSH port（選用，預設 22） |
| `DEPLOY_PATH` | 伺服器專案路徑（如 `/srv/my-project`） |

### 伺服器首次設定

```bash
# 在伺服器上執行（只需一次）
cd /srv/my-project
cp docker/envs/.env.production.example docker/envs/.env.production
nano docker/envs/.env.production   # 填入正式環境值（或留預設讓 deploy.sh 自動產生密碼）

# 首次部署（會自動輪替預設密碼、建立 docker/.env、啟動所有服務）
./scripts/deploy.sh production
```

---

## 新增環境（如 staging）

1. `cp docker/envs/.env.production.example docker/envs/.env.staging.example`
2. 調整 `APP_PORT` / `DB_PORT_EXPOSED` 避免衝突
3. `deploy.yml` 的 `workflow_dispatch.inputs.options` 加入 `staging`
4. 伺服器建立 `docker/envs/.env.staging`
5. `./scripts/deploy.sh staging`

---

## 新增 Docker Service

1. 在 `docker/docker-compose.yml` 新增 service（build context 用 `../service-name`）
2. 若需新對外 port，加入 `APP_PORT` 類型變數（不要直接在 compose 硬編碼）
3. 更新 `docker/envs/.env.*.example` 加入新變數
4. 若需 nginx 路由，在 `docker/nginx/nginx.conf` 新增 upstream + location 區塊
5. 更新本文件的「環境變數清單」與「nginx 路由規則」

---

## 排查問題

### deploy.sh 找不到環境檔
```
❌  找不到環境設定檔：…/docker/envs/.env.production
```
→ `cp docker/envs/.env.production.example docker/envs/.env.production` 並填值

### 想強制重新套用 env
```bash
rm docker/.env
./scripts/deploy.sh production
```

### backend/.env 未同步
症狀：`go run main.go` 連不到 DB  
→ 再次執行 `./scripts/deploy.sh development` 重新同步

### pgAdmin 無法開啟 / URL 不正確
症狀：pgAdmin 頁面跳轉到錯誤的 URL  
→ 確認 nginx `X-Script-Name: /pgadmin` header 已正確設定  
→ `docker compose exec nginx nginx -t` 驗證 nginx config

### phpMyAdmin 無法開啟 / 內部連結錯誤
症狀：phpMyAdmin 登入後頁面連結不含 `/phpmyadmin/` 前綴  
→ 確認 `PMA_ABSOLUTE_URI` 環境變數已設定並包含完整路徑（帶 trailing slash）  
→ 確認 nginx `location /phpmyadmin/` 區塊的 `proxy_pass http://phpmyadmin/;` 有 trailing slash

### docker compose 啟動失敗
```bash
cd docker
docker compose logs nginx     # nginx 代理 log
docker compose logs backend   # Go server log
docker compose logs postgres  # DB log
docker compose logs pgadmin      # pgAdmin log
docker compose logs phpmyadmin   # phpMyAdmin log
docker compose ps             # 確認所有 container 狀態
```
