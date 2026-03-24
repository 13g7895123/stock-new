---
name: deploy
description: "部署、CI/CD、Docker 環境管理。Use when: 新增 service、修改部署流程、更新 CI/CD workflow、管理多環境 .env、使用 deploy.sh、設定 GitHub Actions secrets、排查部署問題。Covers: scripts/deploy.sh、docker/ env 目錄、.github/workflows/ci.yml、.github/workflows/deploy.yml。"
argument-hint: "操作類型 (deploy | env | cicd | secret)"
---

# Deploy — 部署系統知識庫

統一的部署腳本架構，多環境 env 集中存放於 `docker/envs/`，`docker-compose.yml` 位於 `docker/`，CI/CD 透過 GitHub Actions 自動化。

---

## 目錄結構

```
project-root/
├── docker/
│   ├── docker-compose.yml         ← 所有容器定義（build context 用 ../）
│   ├── .env                       ← 由 deploy.sh 自動產生（gitignored）
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
├── backend/
│   └── .env                      ← 由 deploy.sh 自動同步（gitignored）
│
└── .env.example                   ← 根目錄說明用途（保留）
```

> **重要**：根目錄不再有 `docker-compose.yml` 或 `.env`，一律使用 `docker/` 目錄下的檔案。

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

---

## deploy.sh 運作原理

```
docker/envs/.env.<env>
        │
        │  (若 docker/.env 不存在才複製)
        ▼
    docker/.env  ◄── docker compose 從此讀取（env_file + 變數插值）
        │
        └── 提取後端變數 → backend/.env  ◄── go run main.go 使用
```

### 步驟說明

| 步驟 | 操作 | 說明 |
|------|------|------|
| 1 | 驗證來源 | 確認 `docker/envs/.env.<env>` 存在 |
| 2 | 複製 env | 若 `docker/.env` **不存在**才從來源複製；已存在則略過 |
| 3 | 同步 backend/.env | 從 `docker/.env` 提取後端變數 |
| 4 | docker compose | `cd docker && docker compose pull && up -d --build` |

> **「存在則不複製」的用意**：伺服器上的 `docker/.env` 可能被手動調整（如臨時修改密碼），deploy 時不應覆蓋。如需強制重新套用：`rm docker/.env && ./scripts/deploy.sh production`

### backend/.env 同步規則

從 `docker/.env` 提取以下 key 寫入 `backend/.env`（供本機 `go run main.go` 使用）：

| Key | 說明 |
|-----|------|
| `PORT` | 若無則自動從 `BACKEND_INTERNAL_PORT` 填入 |
| `DB_HOST` | 若無則自動補 `localhost`（container 內由 docker-compose override 為 `postgres`） |
| `DB_PORT` | PostgreSQL port |
| `DB_USER` / `DB_PASS` / `DB_NAME` | 資料庫認證 |
| `BACKEND_INTERNAL_PORT` | container 內部 port |

---

## 環境變數清單

定義位置：`docker/envs/.env.<environment>`

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `DB_USER` | PostgreSQL 使用者 | `postgres` |
| `DB_PASS` | PostgreSQL 密碼 | **production 請務必修改** |
| `DB_NAME` | 資料庫名稱 | `stockdb` |
| `FRONTEND_PORT` | Nuxt 對外 port（host） | `3000` |
| `BACKEND_PORT` | Go 對外 port（host） | `8080` |
| `DB_PORT_EXPOSED` | PostgreSQL 對外 port（host） | `5432` |
| `BACKEND_INTERNAL_PORT` | container 內部 port | `8080` |
| `NUXT_BACKEND_URL` | Nuxt SSR → backend URL | `http://backend:8080` |

> **Port 管理原則**：`docker/docker-compose.yml` 只使用 `${VAR:-預設值}` 格式，不得硬編碼 port 數字。

---

## docker/docker-compose.yml 注意事項

- **build context** 使用 `../backend`、`../frontend`（相對於 `docker/` 目錄）
- **env_file: - .env** 指向 `docker/.env`（docker compose 在 `docker/` 目錄執行）
- 執行方式：`cd docker && docker compose ...`，或 `docker compose -f docker/docker-compose.yml`

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
nano docker/envs/.env.production   # 填入正式環境值

# 首次部署（會建立 docker/.env）
./scripts/deploy.sh production
```

---

## 新增環境（如 staging）

1. `cp docker/envs/.env.production.example docker/envs/.env.staging.example`
2. 調整 ports 避免衝突
3. `deploy.yml` 的 `workflow_dispatch.inputs.options` 加入 `staging`
4. 伺服器建立 `docker/envs/.env.staging`
5. `./scripts/deploy.sh staging`

---

## 新增 Docker Service

1. 在 `docker/docker-compose.yml` 新增 service（build context 用 `../service-name`）
2. 新對外 port 加入 `docker/envs/.env.*.example`
3. 更新本文件的「環境變數清單」

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

### docker compose 啟動失敗
```bash
cd docker
docker compose logs backend   # Go server log
docker compose logs postgres  # DB log
docker compose ps             # 確認所有 container 狀態
```


# Deploy — 台股監控系統部署知識庫

台股監控系統使用 `scripts/deploy.sh` 作為統一部署入口，多環境 env 集中存放於 `docker/` 目錄，CI/CD 透過 GitHub Actions 自動化。

---

## 目錄結構

```
34_stock-new/
├── docker/
│   ├── .env.production.example    ← 正式環境範本（已 commit）
│   └── .env.development.example   ← 開發環境範本（已 commit）
│   # 實際使用的 .env.production / .env.development 被 gitignore
│
├── scripts/
│   └── deploy.sh                  ← 部署主腳本（chmod +x）
│
├── .github/
│   └── workflows/
│       ├── ci.yml                 ← CI：lint / build / docker-build
│       └── deploy.yml             ← CD：SSH 部署到伺服器
│
├── docker-compose.yml             ← 讀取根目錄 .env（由 deploy.sh 產生）
├── .env                           ← 由 deploy.sh 自動產生，勿手動修改
├── backend/
│   └── .env                      ← 由 deploy.sh 自動同步，供本機 Go 開發
└── .env.example                   ← 舊版範本（保留說明用途）
```

---

## 快速部署

```bash
# 1. 建立環境設定（\u53ea\u9700一次）
cp docker/.env.production.example docker/.env.production
# 編輯 docker/.env.production，填入 DB_PASS 等正式值

# 2. 部署
./scripts/deploy.sh production

# 開發環境
cp docker/.env.development.example docker/.env.development
./scripts/deploy.sh development
```

---

## deploy.sh 運作原理

```
docker/.env.<env>
       │
       ├─ cp → .env (root)              ← docker-compose 讀取
       │
       └─ grep backend vars → backend/.env  ← go run main.go 讀取
```

deploy.sh 執行三個步驟：
1. 驗證 `docker/.env.<env>` 存在
2. 複製為根目錄 `.env`（docker-compose 使用）
3. 提取後端變數（`PORT`、`DB_*`、`BACKEND_INTERNAL_PORT`）→ `backend/.env`
4. 執行 `docker compose pull && docker compose up -d --build --remove-orphans`

### 後端變數同步規則

deploy.sh 從 `docker/.env.<env>` 提取這些 key 寫入 `backend/.env`：

| Key | 說明 |
|-----|------|
| `PORT` | Go server 監聽 port（= BACKEND_INTERNAL_PORT） |
| `DB_HOST` | 本機開發時為 `localhost`，container 內由 docker-compose 覆蓋為 `postgres` |
| `DB_PORT` | PostgreSQL port（通常 5432） |
| `DB_USER` | DB 使用者名稱 |
| `DB_PASS` | DB 密碼 |
| `DB_NAME` | 資料庫名稱 |
| `BACKEND_INTERNAL_PORT` | container 內部 port |

> **注意**：`DB_HOST` 若未在 env 檔中定義，deploy.sh 會自動補上 `DB_HOST=localhost`。

---

## 環境變數完整清單

所有環境變數定義於 `docker/.env.<env>`：

| 變數 | 說明 | 典型值 |
|------|------|--------|
| `DB_USER` | PostgreSQL 使用者 | `postgres` |
| `DB_PASS` | PostgreSQL 密碼 | **（production 請更改）** |
| `DB_NAME` | 資料庫名稱 | `stockdb` |
| `FRONTEND_PORT` | Nuxt 對外 port | `3000` |
| `BACKEND_PORT` | Go 對外 port | `8080` |
| `DB_PORT_EXPOSED` | PostgreSQL 對外 port | `5432` |
| `BACKEND_INTERNAL_PORT` | container 內部 port | `8080` |
| `NUXT_BACKEND_URL` | Nuxt SSR 呼叫 backend | `http://backend:8080` |

> **Port 管理原則**：所有對外 port 只能在環境檔定義，`docker-compose.yml` 只使用 `${VAR:-預設值}` 格式，不得硬編碼。

---

## GitHub Actions — CI

**檔案**：`.github/workflows/ci.yml`

觸發條件：所有 push / PR

| Job | 說明 |
|-----|------|
| `backend` | `go vet` + `go build` + `go test` |
| `frontend` | `bun install` + `nuxi typecheck` + `bun run build` |
| `docker-build` | 用 development example 建立 docker image（不 push） |

---

## GitHub Actions — CD

**檔案**：`.github/workflows/deploy.yml`

觸發條件：push 到 `master` / `main`，或手動 `workflow_dispatch`

流程：
1. Checkout + 驗證 docker-compose config
2. SSH 到伺服器：`git reset --hard origin/master`
3. 確認 `docker/.env.production` 存在（需手動在伺服器建立）
4. 執行 `bash scripts/deploy.sh production`

### 必要的 GitHub Secrets

在 **Settings → Secrets and variables → Actions** 設定：

| Secret | 說明 |
|--------|------|
| `DEPLOY_HOST` | 伺服器 IP 或 hostname |
| `DEPLOY_USER` | SSH 使用者名稱（如 `deploy`） |
| `DEPLOY_SSH_KEY` | SSH 私鑰（對應伺服器的 `~/.ssh/authorized_keys`） |
| `DEPLOY_PORT` | SSH port（選用，預設 22） |
| `DEPLOY_PATH` | 伺服器上的專案路徑（如 `/srv/34_stock-new`） |

### 伺服器首次部署設定

```bash
# 在伺服器上執行（\u53ea\u9700一次）
cd /srv/34_stock-new
cp docker/.env.production.example docker/.env.production
nano docker/.env.production   # 填入正式環境值
```

---

## 新增環境

若需要 `staging` 環境：

1. 建立範本：`cp docker/.env.production.example docker/.env.staging.example`
2. 調整 `docker/.env.staging.example` 中的 port 以避免衝突
3. 在 `deploy.yml` 的 `workflow_dispatch.inputs.options` 加入 `staging`
4. 伺服器上建立 `docker/.env.staging`
5. 部署：`./scripts/deploy.sh staging`

---

## 新增 Docker Service

1. 在 `docker-compose.yml` 加入新 service，**所有 port 用 `${VAR:-預設}` 格式**
2. 在 `docker/.env.production.example` 和 `docker/.env.development.example` 新增 port 變數
3. 更新本 skill 文件中的「環境變數完整清單」表格

---

## 排查問題

### deploy.sh 找不到環境檔

```
❌  找不到環境設定檔：…/docker/.env.production
```
→ `cp docker/.env.production.example docker/.env.production` 並填值

### backend/.env 未同步

症狀：`go run main.go` 連不到 DB
→ 手動執行 `./scripts/deploy.sh development` 重新同步

### docker compose 啟動失敗

```bash
docker compose logs backend   # 查看 Go server log
docker compose logs postgres  # 查看 DB log
docker compose ps             # 確認所有 container 狀態
```
