<script setup lang="ts">
useHead({
  title: 'DB Viewer | Stock',
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

interface TableInfo {
  name: string
  row_count: number
}

interface ColumnInfo {
  name: string
  type: string
  nullable: string
  default: string
}

interface TableDataResp {
  data: Record<string, unknown>[]
  total: number
  page: number
  limit: number
  pages: number
}

// ── 狀態 ──────────────────────────────────────────────────────
const tables = ref<TableInfo[]>([])
const selectedTable = ref<string | null>(null)
const columns = ref<ColumnInfo[]>([])
const tableData = ref<TableDataResp | null>(null)
const loadingTables = ref(false)
const loadingData = ref(false)
const currentPage = ref(1)
const activeTab = ref<'data' | 'schema'>('data')

// ── API 呼叫 ──────────────────────────────────────────────────
async function fetchTables() {
  loadingTables.value = true
  try {
    const res = await fetch('/api/admin/db/tables')
    tables.value = await res.json()
  } finally {
    loadingTables.value = false
  }
}

async function selectTable(name: string) {
  selectedTable.value = name
  currentPage.value = 1
  activeTab.value = 'data'
  await Promise.all([fetchColumns(name), fetchData(name, 1)])
}

async function fetchColumns(name: string) {
  const res = await fetch(`/api/admin/db/tables/${name}/columns`)
  columns.value = await res.json()
}

async function fetchData(name: string, page: number) {
  loadingData.value = true
  try {
    const res = await fetch(`/api/admin/db/tables/${name}/data?page=${page}&limit=50`)
    tableData.value = await res.json()
  } finally {
    loadingData.value = false
  }
}

async function goPage(p: number) {
  if (!selectedTable.value) return
  currentPage.value = p
  await fetchData(selectedTable.value, p)
}

// 把所有值顯示為字串，處理 null / object
function displayVal(v: unknown): string {
  if (v === null || v === undefined) return 'NULL'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

function isNull(v: unknown): boolean {
  return v === null || v === undefined
}

// ── 初始化 ─────────────────────────────────────────────────────
onMounted(fetchTables)

// ── 提取資料表的欄位順序（取第一列的 keys）──────────────────────
const dataColumns = computed<string[]>(() => {
  if (!tableData.value?.data?.length) return []
  return Object.keys(tableData.value.data[0])
})

const selectedTableInfo = computed(() =>
  tables.value.find(t => t.name === selectedTable.value),
)
</script>

<template>
  <div class="db-viewer">
    <!-- header -->
    <header class="header">
      <div class="header-left">
        <span class="header-icon">🗄️</span>
        <h1 class="header-title">DB Viewer</h1>
        <span class="header-sub">PostgreSQL · public schema</span>
      </div>
      <button class="btn-refresh" :disabled="loadingTables" @click="fetchTables">
        <span :class="{ spin: loadingTables }">↻</span> 重新整理
      </button>
    </header>

    <div class="layout">
      <!-- 左側資料表清單 -->
      <aside class="sidebar">
        <div class="sidebar-title">資料表</div>
        <div v-if="loadingTables" class="loading-text">載入中…</div>
        <ul v-else class="table-list">
          <li
            v-for="t in tables"
            :key="t.name"
            class="table-item"
            :class="{ active: t.name === selectedTable }"
            @click="selectTable(t.name)"
          >
            <span class="table-icon">▦</span>
            <span class="table-name">{{ t.name }}</span>
            <span class="table-count">{{ t.row_count.toLocaleString() }}</span>
          </li>
        </ul>
        <div v-if="!loadingTables && tables.length === 0" class="empty-text">
          無資料表
        </div>
      </aside>

      <!-- 右側內容 -->
      <main class="content">
        <!-- 未選擇時的空狀態 -->
        <div v-if="!selectedTable" class="empty-state">
          <div class="empty-icon">🗄️</div>
          <p>從左側選擇一個資料表</p>
        </div>

        <template v-else>
          <!-- 資料表標題列 -->
          <div class="content-header">
            <div class="content-title">
              <span class="content-table-name">{{ selectedTable }}</span>
              <span v-if="selectedTableInfo" class="content-count">
                共 {{ selectedTableInfo.row_count.toLocaleString() }} 列
              </span>
            </div>
            <div class="tabs">
              <button
                class="tab-btn"
                :class="{ active: activeTab === 'data' }"
                @click="activeTab = 'data'"
              >資料</button>
              <button
                class="tab-btn"
                :class="{ active: activeTab === 'schema' }"
                @click="activeTab = 'schema'"
              >Schema</button>
            </div>
          </div>

          <!-- Schema Tab -->
          <div v-if="activeTab === 'schema'" class="schema-view">
            <table class="data-table">
              <thead>
                <tr>
                  <th>欄位名稱</th>
                  <th>資料型別</th>
                  <th>可Null</th>
                  <th>預設值</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="col in columns" :key="col.name">
                  <td class="col-name">{{ col.name }}</td>
                  <td class="col-type">{{ col.type }}</td>
                  <td>
                    <span :class="col.nullable === 'YES' ? 'badge-yes' : 'badge-no'">
                      {{ col.nullable === 'YES' ? 'YES' : 'NO' }}
                    </span>
                  </td>
                  <td class="col-default">{{ col.default || '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Data Tab -->
          <div v-else class="data-view">
            <div v-if="loadingData" class="loading-text center">載入中…</div>
            <template v-else-if="tableData">
              <div v-if="tableData.data.length === 0" class="empty-text center">
                此資料表沒有資料
              </div>
              <template v-else>
                <div class="table-scroll">
                  <table class="data-table">
                    <thead>
                      <tr>
                        <th v-for="col in dataColumns" :key="col">{{ col }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(row, i) in tableData.data" :key="i">
                        <td
                          v-for="col in dataColumns"
                          :key="col"
                          :class="{ 'null-cell': isNull(row[col]) }"
                        >{{ displayVal(row[col]) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>

                <!-- 分頁 -->
                <div class="pagination">
                  <span class="page-info">
                    第 {{ tableData.page }} / {{ tableData.pages }} 頁 ·
                    共 {{ tableData.total.toLocaleString() }} 列
                  </span>
                  <div class="page-btns">
                    <button
                      class="page-btn"
                      :disabled="tableData.page <= 1"
                      @click="goPage(1)"
                    >«</button>
                    <button
                      class="page-btn"
                      :disabled="tableData.page <= 1"
                      @click="goPage(tableData.page - 1)"
                    >‹</button>
                    <span class="page-current">{{ tableData.page }}</span>
                    <button
                      class="page-btn"
                      :disabled="tableData.page >= tableData.pages"
                      @click="goPage(tableData.page + 1)"
                    >›</button>
                    <button
                      class="page-btn"
                      :disabled="tableData.page >= tableData.pages"
                      @click="goPage(tableData.pages)"
                    >»</button>
                  </div>
                </div>
              </template>
            </template>
          </div>
        </template>
      </main>
    </div>
  </div>
</template>

<style scoped>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

.db-viewer {
  font-family: 'DM Sans', sans-serif;
  background: #0d1117;
  color: #c9d1d9;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* ── Header ────────────────────────────────────────────────── */
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 24px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
  position: sticky;
  top: 0;
  z-index: 10;
}
.header-left { display: flex; align-items: center; gap: 10px; }
.header-icon { font-size: 1.3rem; }
.header-title { font-size: 1.1rem; font-weight: 600; color: #e6edf3; }
.header-sub { font-size: 0.78rem; color: #8b949e; font-family: 'Fira Code', monospace; }
.btn-refresh {
  display: flex; align-items: center; gap: 6px;
  padding: 6px 14px; border-radius: 6px;
  background: #21262d; border: 1px solid #30363d;
  color: #c9d1d9; font-size: 0.85rem; cursor: pointer;
  transition: background 0.15s;
}
.btn-refresh:hover { background: #30363d; }
.btn-refresh:disabled { opacity: 0.5; cursor: default; }

/* ── Layout ─────────────────────────────────────────────────── */
.layout {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  height: calc(100vh - 57px);
}

/* ── Sidebar ────────────────────────────────────────────────── */
.sidebar {
  width: 240px;
  min-width: 200px;
  background: #161b22;
  border-right: 1px solid #30363d;
  overflow-y: auto;
  padding: 12px 0;
  flex-shrink: 0;
}
.sidebar-title {
  font-size: 0.7rem; font-weight: 600; letter-spacing: 0.08em;
  text-transform: uppercase; color: #8b949e;
  padding: 0 16px 8px;
}
.table-list { list-style: none; }
.table-item {
  display: flex; align-items: center; gap: 8px;
  padding: 7px 16px; cursor: pointer;
  transition: background 0.1s;
  border-left: 3px solid transparent;
}
.table-item:hover { background: #1c2128; }
.table-item.active {
  background: #1c2128;
  border-left-color: #388bfd;
  color: #e6edf3;
}
.table-icon { font-size: 0.75rem; color: #8b949e; }
.table-name { flex: 1; font-size: 0.88rem; font-family: 'Fira Code', monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.table-count {
  font-size: 0.7rem; font-family: 'Fira Code', monospace;
  color: #8b949e; background: #21262d;
  padding: 1px 6px; border-radius: 10px;
}

/* ── Content ──────────────────────────────────────────────────── */
.content {
  flex: 1;
  overflow: auto;
  display: flex;
  flex-direction: column;
}
.empty-state {
  flex: 1; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 12px;
  color: #8b949e;
}
.empty-icon { font-size: 3rem; }

/* ── Content Header ──────────────────────────────────────────── */
.content-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 20px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
  position: sticky;
  top: 0;
  z-index: 5;
}
.content-title { display: flex; align-items: center; gap: 12px; }
.content-table-name {
  font-family: 'Fira Code', monospace;
  font-size: 1rem; font-weight: 600; color: #e6edf3;
}
.content-count { font-size: 0.8rem; color: #8b949e; }
.tabs { display: flex; gap: 4px; }
.tab-btn {
  padding: 5px 14px; border-radius: 5px;
  background: transparent; border: 1px solid #30363d;
  color: #8b949e; font-size: 0.85rem; cursor: pointer;
  transition: all 0.15s;
}
.tab-btn.active {
  background: #21262d; border-color: #58a6ff;
  color: #58a6ff;
}

/* ── Schema View ─────────────────────────────────────────────── */
.schema-view { padding: 16px 20px; overflow-x: auto; }
.col-name { font-family: 'Fira Code', monospace; color: #79c0ff; }
.col-type { font-family: 'Fira Code', monospace; color: #ff9a3e; }
.col-default { font-family: 'Fira Code', monospace; color: #8b949e; font-size: 0.8rem; }
.badge-yes { background: #1a2a1a; color: #56d364; padding: 2px 7px; border-radius: 4px; font-size: 0.75rem; font-family: 'Fira Code', monospace; }
.badge-no  { background: #2a1a1a; color: #f85149; padding: 2px 7px; border-radius: 4px; font-size: 0.75rem; font-family: 'Fira Code', monospace; }

/* ── Data View ──────────────────────────────────────────────── */
.data-view { display: flex; flex-direction: column; flex: 1; overflow: hidden; }
.table-scroll { flex: 1; overflow: auto; padding: 0 20px 0; }

/* ── Data Table ──────────────────────────────────────────────── */
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.83rem;
  font-family: 'Fira Code', monospace;
}
.data-table thead {
  position: sticky; top: 0; z-index: 2;
  background: #161b22;
}
.data-table th {
  padding: 9px 12px;
  text-align: left;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #8b949e;
  border-bottom: 2px solid #30363d;
  white-space: nowrap;
}
.data-table td {
  padding: 7px 12px;
  border-bottom: 1px solid #21262d;
  color: #c9d1d9;
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.data-table tr:hover td { background: #1c2128; }
.null-cell { color: #6e7681; font-style: italic; }

/* ── Pagination ──────────────────────────────────────────────── */
.pagination {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 20px;
  border-top: 1px solid #30363d;
  flex-shrink: 0;
}
.page-info { font-size: 0.8rem; color: #8b949e; }
.page-btns { display: flex; align-items: center; gap: 4px; }
.page-btn {
  width: 32px; height: 32px;
  background: #21262d; border: 1px solid #30363d;
  color: #c9d1d9; border-radius: 5px;
  font-size: 1rem; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.15s;
}
.page-btn:hover:not(:disabled) { background: #30363d; }
.page-btn:disabled { opacity: 0.4; cursor: default; }
.page-current {
  padding: 0 12px; height: 32px; line-height: 32px;
  background: #1a2a4a; border: 1px solid #388bfd;
  color: #58a6ff; border-radius: 5px; font-size: 0.85rem;
}

/* ── Misc ──────────────────────────────────────────────────── */
.loading-text { padding: 16px; color: #8b949e; font-size: 0.88rem; }
.center { text-align: center; padding: 40px; }
.empty-text { padding: 16px; color: #8b949e; font-size: 0.85rem; }

.spin { display: inline-block; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
