# Port 變數對照表

| 變數名稱 | 預設值 | 類型 | 說明 |
|---|---|---|---|
| `FRONTEND_PORT` | `3000` | 對外 Port | Nuxt 前端對外 Port |
| `BACKEND_PORT` | `8080` | 對外 Port | Go 後端 API 對外 Port |
| `BACKEND_INTERNAL_PORT` | `8080` | 容器內部 Port | Go 後端在容器內監聽的 Port |
| `DB_PORT_EXPOSED` | `5432` | 對外 Port | PostgreSQL 對外 Port（開發用） |

## 新增 Port 時的 Checklist

- [ ] `.env` 加入新變數
- [ ] `.env.example` 同步加入新變數
- [ ] `docker-compose.yml` 使用 `${VAR:-預設值}` 格式
- [ ] 更新此表格
