---
name: port-management
description: 'Port 管理規則：所有對外暴露的 Port 必須統一定義在根目錄 .env 中。Use when: 新增 service、修改 docker-compose.yml ports、新增 Port 變數、設定容器對外 Port。DO NOT hardcode port numbers in docker-compose.yml or source code.'
argument-hint: '要新增或修改的 Port 變數名稱'
---

# Port 管理規則

## 核心原則

所有對外暴露的 Port **必須** 統一定義在根目錄的 `.env`，不得在 `docker-compose.yml` 或程式碼裡硬編碼 Port 數字。

## 何時使用此規則

- 在 `docker-compose.yml` 新增或修改 `ports:` 設定
- 新增新的服務（service）
- 修改任何容器對外的 Port

## 操作步驟

### 1. 在 `.env` 與 `.env.example` 新增 Port 變數

對外 Port（Host 端）：
```env
FRONTEND_PORT=3000
BACKEND_PORT=8080
DB_PORT_EXPOSED=5432
```

容器內部 Port（Container 端）：
```env
BACKEND_INTERNAL_PORT=8080
```

### 2. 在 `docker-compose.yml` 只用變數，格式一律 `${VAR:-預設值}`

```yaml
ports:
  - "${BACKEND_PORT:-8080}:${BACKEND_INTERNAL_PORT:-8080}"
```

**禁止寫法：**
```yaml
ports:
  - "8080:8080"   # ❌ 硬編碼
```

### 3. 更新 Port 變數對照表（見 [references/port-table.md](./references/port-table.md)）

## 當前 Port 變數

詳見 [references/port-table.md](./references/port-table.md)
