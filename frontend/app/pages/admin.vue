<script setup lang="ts">
useHead({
  title: '後台管理 | Stock',
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

// ── Section 導航 ──────────────────────────────────────────────
type Section = 'settings' | 'logs' | 'db' | 'schedule'
const route = useRoute()
const activeSection = ref<Section>('settings')

onMounted(() => {
  const s = route.query.section as string
  if (s === 'settings' || s === 'logs' || s === 'db' || s === 'schedule') activeSection.value = s
})

watch(activeSection, (s) => {
  navigateTo({ query: { section: s } }, { replace: true })
})

const { isDark } = useAppPrefs()
const { entries: logEntries, clearLogs } = useApiLogger()
const { logCall } = useApiLogger()

// ══════════════════════════════════════════════════════════════
// 系統設定 Section
// ══════════════════════════════════════════════════════════════
interface SchemeInfo {
  id: string
  label: string
  description: string
  need_service?: string
}
interface FeatureConfig {
  primary: string
  fallback_enabled: boolean
  fallback: string
  fallback_trigger: 'error' | 'empty_data'
}
interface Feature {
  id: string
  label: string
  description: string
  category: string
  schemes: SchemeInfo[]
  config: FeatureConfig
}

const features = ref<Feature[]>([])
const selectedId = ref<string | null>(null)
const settingsLoading = ref(false)
const saving = ref(false)
const savedId = ref<string | null>(null)
const settingsError = ref('')
const drafts = ref<Record<string, FeatureConfig>>({})

async function fetchSettings() {
  settingsLoading.value = true
  settingsError.value = ''
  try {
    const data = await logCall({
      action: '載入系統設定',
      trigger: '後台頁面 / 重新整理',
      method: 'GET',
      endpoint: '/api/settings/features',
      analysis: [
        '取得所有可設定功能的資料來源配置。',
        '',
        '回應格式：Feature[] 陣列，每個元素包含：',
        '  • id          功能唯一識別碼（例：chips_scraper）',
        '  • label       中文顯示名稱',
        '  • description 功能說明',
        '  • category    類別（scraper = 爬蟲，sync = 同步）',
        '  • schemes[]   可選方案列表，每個方案含：',
        '      - id           方案識別碼（例：python_playwright）',
        '      - label        方案名稱',
        '      - description  方案說明',
        '      - need_service 依賴的外部服務（選填）',
        '  • config      目前的設定：',
        '      - primary          當前主方案 ID',
        '      - fallback_enabled 備案機制是否啟用',
        '      - fallback         備案方案 ID',
        '      - fallback_trigger error = 連線失敗時 / empty_data = 資料為空時',
      ].join('\n'),
      call: () => $fetch<Feature[]>('/api/settings/features'),
    })
    features.value = data
    for (const f of features.value) {
      drafts.value[f.id] = { ...f.config }
    }
    if (!selectedId.value && features.value.length) {
      selectedId.value = features.value[0]?.id ?? null
    }
  } catch (e: unknown) {
    const err = e as { message?: string }
    settingsError.value = err.message ?? '載入失敗'
  } finally {
    settingsLoading.value = false
  }
}

async function saveFeature(id: string) {
  saving.value = true
  savedId.value = null
  settingsError.value = ''
  try {
    const updated = await logCall({
      action: '儲存功能設定',
      trigger: `儲存設定 [功能：${id}]`,
      method: 'PUT',
      endpoint: `/api/settings/features/${id}`,
      requestBody: drafts.value[id],
      analysis: [
        `更新指定功能（${id}）的資料來源設定。`,
        '',
        '請求 body（JSON）：',
        '  • primary          新的主方案 ID（字串）',
        '  • fallback_enabled 是否啟用備案機制（布林）',
        '  • fallback         備案方案 ID（字串，fallback_enabled=false 時可空）',
        '  • fallback_trigger 觸發條件（"error" | "empty_data"）',
        '',
        '回應格式：更新後的完整 Feature 物件，結構與 GET /api/settings/features 中的元素相同。',
        '前端收到後更新本地 features[] 與草稿，並顯示「✓ 已儲存」提示 2 秒。',
      ].join('\n'),
      call: () => $fetch<Feature>(`/api/settings/features/${id}`, {
        method: 'PUT',
        body: JSON.stringify(drafts.value[id]),
        headers: { 'Content-Type': 'application/json' },
      }),
    })
    const idx = features.value.findIndex(f => f.id === id)
    if (idx !== -1) features.value[idx] = updated
    drafts.value[id] = { ...updated.config }
    savedId.value = id
    setTimeout(() => { savedId.value = null }, 2000)
  } catch (e: unknown) {
    const err = e as { response?: { _data?: { error?: string } }; data?: { error?: string }; message?: string }
    settingsError.value = err?.response?._data?.error ?? err?.data?.error ?? err.message ?? '儲存失敗'
  } finally {
    saving.value = false
  }
}

const selectedFeature = computed(() =>
  features.value.find(f => f.id === selectedId.value),
)
const EMPTY_CONFIG: FeatureConfig = { primary: '', fallback_enabled: false, fallback: '', fallback_trigger: 'error' }
const draft = computed<FeatureConfig>(() =>
  selectedId.value ? (drafts.value[selectedId.value] ?? EMPTY_CONFIG) : EMPTY_CONFIG,
)
const isDirty = computed(() => {
  if (!selectedId.value || !selectedFeature.value) return false
  return JSON.stringify(drafts.value[selectedId.value]) !== JSON.stringify(selectedFeature.value.config)
})
const fallbackOptions = computed<SchemeInfo[]>(() => {
  if (!selectedFeature.value) return []
  return selectedFeature.value.schemes.filter(s => s.id !== draft.value.primary)
})
watch(() => draft.value.primary, (newPrimary) => {
  if (!selectedId.value) return
  const d = drafts.value[selectedId.value]
  if (d && d.fallback === newPrimary) {
    d.fallback = fallbackOptions.value[0]?.id ?? ''
  }
})

const schemeColors: Record<string, string> = {
  go_http:            '#4ecfa8',
  python_http:        '#5b9cf6',
  python_playwright:  '#e07b5a',
  twse_tpex_api:      '#a78ce8',
  broker_api:         '#f0a842',
}
function schemeColor(id: string) { return schemeColors[id] ?? '#8b949e' }

const categoryLabels: Record<string, string> = { scraper: '爬蟲', sync: '同步' }

// ══════════════════════════════════════════════════════════════
// 資料庫 Section
// ══════════════════════════════════════════════════════════════
interface TableInfo { name: string; row_count: number }
interface ColumnInfo { name: string; type: string; nullable: string; default: string }
interface TableDataResp {
  data: Record<string, unknown>[]
  total: number
  page: number
  limit: number
  pages: number
}

const tables = ref<TableInfo[]>([])
const selectedTable = ref<string | null>(null)
const columns = ref<ColumnInfo[]>([])
const tableData = ref<TableDataResp | null>(null)
const dbLoading = ref(false)
const dbDataLoading = ref(false)
const dbPage = ref(1)
const dbActiveTab = ref<'data' | 'schema'>('data')

async function fetchDbTables() {
  dbLoading.value = true
  try {
    const res = await fetch('/api/admin/db/tables')
    tables.value = await res.json()
  } finally {
    dbLoading.value = false
  }
}

async function selectDbTable(name: string) {
  selectedTable.value = name
  dbPage.value = 1
  dbActiveTab.value = 'data'
  await Promise.all([
    fetch(`/api/admin/db/tables/${name}/columns`).then(r => r.json()).then(d => { columns.value = d }),
    fetchDbData(name, 1),
  ])
}

async function fetchDbData(name: string, page: number) {
  dbDataLoading.value = true
  try {
    const res = await fetch(`/api/admin/db/tables/${name}/data?page=${page}&limit=50`)
    tableData.value = await res.json()
  } finally {
    dbDataLoading.value = false
  }
}

async function goDbPage(p: number) {
  if (!selectedTable.value) return
  dbPage.value = p
  await fetchDbData(selectedTable.value, p)
}

function isNull(v: unknown) { return v === null || v === undefined }

const dataColumns = computed<string[]>(() => {
  if (!tableData.value?.data?.length) return []
  return Object.keys(tableData.value.data[0])
})
const selectedTableInfo = computed(() =>
  tables.value.find(t => t.name === selectedTable.value),
)

const dbColSearch = ref('')
const filteredColumns = computed(() => {
  if (!dbColSearch.value.trim()) return columns.value
  const q = dbColSearch.value.toLowerCase()
  return columns.value.filter(c =>
    c.name.toLowerCase().includes(q) || c.type.toLowerCase().includes(q),
  )
})

function typeClass(t: string): string {
  const lower = t.toLowerCase()
  if (/int|serial/.test(lower)) return 'type-int'
  if (/varchar|text|char|uuid|enum/.test(lower)) return 'type-str'
  if (/bool/.test(lower)) return 'type-bool'
  if (/timestamp|date|time/.test(lower)) return 'type-time'
  if (/numeric|decimal|float|double|real/.test(lower)) return 'type-num'
  if (/json|array/.test(lower)) return 'type-json'
  return 'type-other'
}

function dbDisplayVal(v: unknown): string {
  if (v === null || v === undefined) return ''
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

// ══════════════════════════════════════════════════════════════
// 操作紀錄 Section
// ══════════════════════════════════════════════════════════════
type LogFilter = 'all' | 'success' | 'error'
const logFilter = ref<LogFilter>('all')
const logSearch = ref('')
const expandedLogId = ref<number | null>(null)

const filteredLogs = computed(() => {
  let list = logEntries.value
  if (logFilter.value === 'success') list = list.filter(e => e.success)
  if (logFilter.value === 'error') list = list.filter(e => !e.success)
  if (logSearch.value.trim()) {
    const q = logSearch.value.trim().toLowerCase()
    list = list.filter(e =>
      e.action.toLowerCase().includes(q) ||
      e.endpoint.toLowerCase().includes(q) ||
      e.trigger.toLowerCase().includes(q),
    )
  }
  return list
})

function toggleLogEntry(id: number) {
  expandedLogId.value = expandedLogId.value === id ? null : id
}

function formatJson(val: unknown): string {
  try {
    return JSON.stringify(val, null, 2)
  } catch {
    return String(val)
  }
}

// ══════════════════════════════════════════════════════════════
// 排程管理 Section
// ══════════════════════════════════════════════════════════════
interface Schedule {
  task_id: string
  enabled: boolean
  type: string
  hour: number
  minute: number
  weekday: number
  exclude_weekends: boolean
  params: string
  last_run_at: string | null
  next_run_at: string | null
  updated_at: string
}
interface ScheduleEntry {
  id: string
  label: string
  description: string
  has_params: boolean
  schedule: Schedule
}
interface ScheduleDraft {
  enabled: boolean
  type: 'manual' | 'daily' | 'weekly'
  hour: number
  minute: number
  weekday: number
  exclude_weekends: boolean
  days: number
}

const schedules = ref<ScheduleEntry[]>([])
const scheduleDrafts = ref<Record<string, ScheduleDraft>>({})
const scheduleLoading = ref(false)
const scheduleSaving = ref<Record<string, boolean>>({})
const scheduleRunning = ref<Record<string, boolean>>({})
const scheduleSaved = ref<Record<string, boolean>>({})
const scheduleError = ref('')
const weekdayNames = ['週日', '週一', '週二', '週三', '週四', '週五', '週六']

function parseScheduleDraft(s: Schedule, hasParams: boolean): ScheduleDraft {
  let days = 1
  if (hasParams) {
    try { days = (JSON.parse(s.params) as { days?: number }).days ?? 1 } catch {}
  }
  return {
    enabled: s.enabled,
    type: (s.type as 'manual' | 'daily' | 'weekly') || 'manual',
    hour: s.hour,
    minute: s.minute,
    weekday: s.weekday,
    exclude_weekends: s.exclude_weekends ?? false,
    days,
  }
}

async function fetchSchedules() {
  scheduleLoading.value = true
  scheduleError.value = ''
  try {
    const data = await logCall({
      action: '載入排程設定',
      trigger: '排程管理頁 / 載入',
      method: 'GET',
      endpoint: '/api/schedules',
      analysis: '取得所有任務的排程設定（enabled, type, hour, minute, weekday, params, last_run_at, next_run_at）',
      call: () => $fetch<ScheduleEntry[]>('/api/schedules'),
    })
    schedules.value = data
    for (const e of data) {
      scheduleDrafts.value[e.id] = parseScheduleDraft(e.schedule, e.has_params)
    }
  } catch (e: unknown) {
    const err = e as { message?: string }
    scheduleError.value = err.message ?? '載入失敗'
  } finally {
    scheduleLoading.value = false
  }
}

async function saveSchedule(entry: ScheduleEntry) {
  const id = entry.id
  const d = scheduleDrafts.value[id]
  if (!d) return
  scheduleSaving.value[id] = true
  scheduleError.value = ''
  try {
    const body = {
      enabled: d.enabled,
      type: d.type,
      hour: d.hour,
      minute: d.minute,
      weekday: d.weekday,
      exclude_weekends: d.exclude_weekends,
      params: entry.has_params ? JSON.stringify({ days: d.days }) : '{}',
    }
    const updated = await logCall({
      action: '儲存排程設定',
      trigger: `儲存排程 [任務：${entry.label}]`,
      method: 'PUT',
      endpoint: `/api/schedules/${id}`,
      requestBody: body,
      analysis: '更新任務排程設定並計算 next_run_at。\ntype=manual 時不會自動觸發。\n回應為更新後的 TaskSchedule 物件。',
      call: () => $fetch<Schedule>(`/api/schedules/${id}`, {
        method: 'PUT',
        body: JSON.stringify(body),
        headers: { 'Content-Type': 'application/json' },
      }),
    })
    const idx = schedules.value.findIndex(e => e.id === id)
    if (idx !== -1) schedules.value[idx].schedule = updated
    scheduleDrafts.value[id] = parseScheduleDraft(updated, entry.has_params)
    scheduleSaved.value[id] = true
    setTimeout(() => { scheduleSaved.value[id] = false }, 2000)
  } catch (e: unknown) {
    const err = e as { message?: string }
    scheduleError.value = err.message ?? '儲存失敗'
  } finally {
    scheduleSaving.value[id] = false
  }
}

async function runNow(entry: ScheduleEntry) {
  const id = entry.id
  scheduleRunning.value[id] = true
  scheduleError.value = ''
  try {
    await logCall({
      action: '立即執行任務',
      trigger: `手動觸發 [任務：${entry.label}]`,
      method: 'POST',
      endpoint: `/api/schedules/${id}/run`,
      analysis: '立即執行一次任務（無視排程時間），使用目前 params 設定。回應為 { ok: true, task_id }。',
      call: () => $fetch(`/api/schedules/${id}/run`, { method: 'POST' }),
    })
  } catch (e: unknown) {
    const err = e as { message?: string }
    scheduleError.value = err.message ?? '執行失敗'
  } finally {
    scheduleRunning.value[id] = false
  }
}

function scheduleDraftDirty(entry: ScheduleEntry): boolean {
  const d = scheduleDrafts.value[entry.id]
  const s = entry.schedule
  if (!d) return false
  let origDays = 1
  if (entry.has_params) {
    try { origDays = (JSON.parse(s.params) as { days?: number }).days ?? 1 } catch {}
  }
  return d.enabled !== s.enabled
    || d.type !== s.type
    || d.hour !== s.hour
    || d.minute !== s.minute
    || d.weekday !== s.weekday
    || d.exclude_weekends !== (s.exclude_weekends ?? false)
    || (entry.has_params && d.days !== origDays)
}

function fmtRunTime(t: string | null): string {
  if (!t) return '—'
  try {
    return new Date(t).toLocaleString('zh-TW', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
    })
  } catch { return t }
}

// ── 假日管理 ─────────────────────────────────────────────────
const holidays = ref<string[]>([])
const holidaysDraft = ref('')
const holidaysLoading = ref(false)
const holidaysSaving = ref(false)
const holidaysSaved = ref(false)
const holidaysError = ref('')

async function fetchHolidays() {
  holidaysLoading.value = true
  holidaysError.value = ''
  try {
    const data = await $fetch<{ dates: string[] }>('/api/schedules/holidays')
    holidays.value = data.dates ?? []
    holidaysDraft.value = holidays.value.join('\n')
  } catch (e: unknown) {
    const err = e as { message?: string }
    holidaysError.value = err.message ?? '載入失敗'
  } finally {
    holidaysLoading.value = false
  }
}

async function saveHolidays() {
  const raw = holidaysDraft.value
  const dates = raw.split('\n')
    .map(l => l.trim())
    .filter(l => /^\d{4}-\d{2}-\d{2}$/.test(l))
  holidaysSaving.value = true
  holidaysSaved.value = false
  holidaysError.value = ''
  try {
    const result = await $fetch<{ dates: string[] }>('/api/schedules/holidays', {
      method: 'PUT',
      body: JSON.stringify({ dates }),
      headers: { 'Content-Type': 'application/json' },
    })
    holidays.value = result.dates ?? dates
    holidaysDraft.value = holidays.value.join('\n')
    holidaysSaved.value = true
    setTimeout(() => { holidaysSaved.value = false }, 2000)
  } catch (e: unknown) {
    const err = e as { message?: string }
    holidaysError.value = err.message ?? '儲存失敗'
  } finally {
    holidaysSaving.value = false
  }
}

// ── 初始化 ────────────────────────────────────────────────────
onMounted(() => {
  fetchSettings()
  fetchDbTables()
  fetchSchedules()
  fetchHolidays()
})
</script>

<template>
  <div class="page" :class="{ light: !isDark }">

    <!-- ══ Header ══ -->
    <header class="header">
      <div class="header__inner">
        <div class="brand">
          <NuxtLink to="/" class="back-btn" aria-label="返回首頁">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M10 3L5 8l5 5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </NuxtLink>
          <div class="brand-divider" />
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true" class="brand-icon">
            <rect x="1"  y="1"  width="7" height="7" rx="2" fill="var(--blue)" />
            <rect x="10" y="1"  width="7" height="7" rx="2" fill="var(--gold)" opacity="0.6" />
            <rect x="1"  y="10" width="7" height="7" rx="2" fill="var(--blue)"  opacity="0.45" />
            <rect x="10" y="10" width="7" height="7" rx="2" fill="var(--blue)"  opacity="0.8" />
          </svg>
          <div class="brand-info">
            <span class="brand-main">後台管理</span>
            <span class="brand-sub">Admin Panel</span>
          </div>
        </div>
        <nav class="header-nav">
          <span v-if="logEntries.length" class="log-count-badge">
            {{ logEntries.length }} 筆紀錄
          </span>
        </nav>
      </div>
    </header>

    <!-- ══ Layout ══ -->
    <div class="admin-layout">

      <!-- ── 左側導航 ── -->
      <aside class="admin-nav">
        <div class="nav-group-label">管理項目</div>
        <nav class="nav-items">
          <button
            class="nav-item"
            :class="{ active: activeSection === 'settings' }"
            @click="activeSection = 'settings'"
          >
            <svg width="15" height="15" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <circle cx="8" cy="8" r="2.3" stroke="currentColor" stroke-width="1.4"/>
              <path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M3.05 12.95l1.06-1.06M11.89 4.11l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
            系統設定
          </button>
          <button
            class="nav-item"
            :class="{ active: activeSection === 'logs' }"
            @click="activeSection = 'logs'"
          >
            <svg width="15" height="15" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <rect x="2" y="3" width="12" height="1.5" rx="0.75" fill="currentColor" opacity="0.7"/>
              <rect x="2" y="7" width="12" height="1.5" rx="0.75" fill="currentColor" opacity="0.5"/>
              <rect x="2" y="11" width="8" height="1.5" rx="0.75" fill="currentColor" opacity="0.35"/>
            </svg>
            操作紀錄
            <span v-if="logEntries.length" class="nav-badge">{{ logEntries.length }}</span>
          </button>
          <button
            class="nav-item"
            :class="{ active: activeSection === 'db' }"
            @click="activeSection = 'db'"
          >
            <svg width="15" height="15" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <ellipse cx="8" cy="4" rx="6" ry="2" stroke="currentColor" stroke-width="1.4"/>
              <path d="M2 4v4c0 1.1 2.69 2 6 2s6-.9 6-2V4" stroke="currentColor" stroke-width="1.4"/>
              <path d="M2 8v4c0 1.1 2.69 2 6 2s6-.9 6-2V8" stroke="currentColor" stroke-width="1.4"/>
            </svg>
            資料庫檢視
          </button>
          <button
            class="nav-item"
            :class="{ active: activeSection === 'schedule' }"
            @click="activeSection = 'schedule'"
          >
            <svg width="15" height="15" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <rect x="2" y="3" width="12" height="11" rx="2" stroke="currentColor" stroke-width="1.4"/>
              <path d="M5 1v3M11 1v3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
              <path d="M2 7h12" stroke="currentColor" stroke-width="1.4"/>
              <path d="M5 10.5h2M5 12.5h4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
              <circle cx="11.5" cy="11.5" r="2" fill="currentColor" opacity="0.85"/>
            </svg>
            排程管理
          </button>
        </nav>

        <div class="nav-separator" />
        <div class="nav-group-label">快速連結</div>
        <nav class="nav-items">
          <NuxtLink to="/" class="nav-link-item">
            <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><path d="M2 8L8 2l6 6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/><path d="M4 6v7h8V6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            主控台
          </NuxtLink>
          <NuxtLink to="/debug" class="nav-link-item">
            <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="5.5" stroke="currentColor" stroke-width="1.4"/><path d="M8 5.5v3l2 1.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
            除錯工具
          </NuxtLink>
        </nav>
      </aside>

      <!-- ── 主內容 ── -->
      <main class="admin-main">

        <!-- ════ 系統設定 ════ -->
        <section v-if="activeSection === 'settings'" class="section-wrap">
          <div class="section-header">
            <div>
              <h1 class="section-title">系統設定</h1>
              <p class="section-desc">管理各功能的資料來源方案與備案策略</p>
            </div>
            <button class="btn-secondary" :disabled="settingsLoading" @click="fetchSettings">
              <svg width="13" height="13" viewBox="0 0 16 16" fill="none" :class="{ spin: settingsLoading }">
                <path d="M13.65 2.35A8 8 0 1 0 14.9 8.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
                <path d="M14.5 2v4h-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              重新整理
            </button>
          </div>

          <div v-if="settingsError" class="error-banner">
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none"><path d="M8 1L1 14h14L8 1Z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/><path d="M8 7v3M8 12v.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
            {{ settingsError }}
          </div>

          <div class="settings-layout">
            <!-- 功能列表 -->
            <div class="feat-sidebar">
              <div class="feat-sidebar-title">功能列表</div>
              <div v-if="settingsLoading" class="feat-loading">
                <span class="spin-icon">◌</span> 載入中…
              </div>
              <ul v-else class="feat-list">
                <li
                  v-for="f in features"
                  :key="f.id"
                  class="feat-item"
                  :class="{ active: f.id === selectedId }"
                  @click="selectedId = f.id"
                >
                  <div class="feat-item-top">
                    <span class="feat-item-label">{{ f.label }}</span>
                    <span class="badge-category">{{ categoryLabels[f.category] ?? f.category }}</span>
                  </div>
                  <div class="feat-item-scheme">
                    <span class="scheme-dot" :style="{ background: schemeColor(drafts[f.id]?.primary ?? '') }" />
                    <span class="scheme-id-text">{{ drafts[f.id]?.primary }}</span>
                    <span v-if="drafts[f.id]?.fallback_enabled" class="fallback-badge">備案</span>
                  </div>
                </li>
              </ul>
            </div>

            <!-- 功能設定內容 -->
            <div class="feat-content">
              <div v-if="!selectedFeature" class="empty-state">
                <svg width="36" height="36" viewBox="0 0 36 36" fill="none" class="empty-icon"><circle cx="18" cy="18" r="6" stroke="currentColor" stroke-width="1.6"/><path d="M18 2v3M18 31v3M2 18h3M31 18h3M5.9 5.9l2.1 2.1M27.9 27.9l2.1 2.1M5.9 30.1l2.1-2.1M27.9 8.1l2.1-2.1" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
                <p>從左側選擇一個功能</p>
              </div>

              <template v-else>
                <div class="feat-content-header">
                  <div>
                    <h2 class="feat-title">{{ selectedFeature.label }}</h2>
                    <p class="feat-desc">{{ selectedFeature.description }}</p>
                  </div>
                  <div class="feat-header-actions">
                    <span v-if="savedId === selectedFeature.id" class="saved-badge">✓ 已儲存</span>
                    <button
                      class="btn-primary"
                      :class="{ dirty: isDirty }"
                      :disabled="saving || !isDirty"
                      @click="saveFeature(selectedFeature.id)"
                    >
                      {{ saving ? '儲存中…' : '儲存設定' }}
                    </button>
                  </div>
                </div>

                <!-- 主要方案 -->
                <section class="setting-section">
                  <div class="section-badge-title">
                    <span class="step-chip">1</span>
                    主要方案
                    <span class="section-hint">正常情況下使用的資料取得方式</span>
                  </div>
                  <div class="scheme-grid">
                    <label
                      v-for="s in selectedFeature.schemes"
                      :key="s.id"
                      class="scheme-card"
                      :class="{ selected: draft.primary === s.id }"
                      :style="{ '--accent': schemeColor(s.id) }"
                    >
                      <input v-model="draft.primary" type="radio" :value="s.id" class="scheme-radio" />
                      <div class="scheme-card-header">
                        <span class="scheme-color-bar" />
                        <span class="scheme-label">{{ s.label }}</span>
                        <span v-if="draft.primary === s.id" class="scheme-active-badge">主要</span>
                      </div>
                      <p class="scheme-desc">{{ s.description }}</p>
                      <div v-if="s.need_service" class="scheme-requires">
                        需要：<code>{{ s.need_service }}</code>
                      </div>
                    </label>
                  </div>
                </section>

                <!-- 備案設定 -->
                <section class="setting-section">
                  <div class="section-badge-title">
                    <span class="step-chip">2</span>
                    備案設定
                    <span class="section-hint">主方案失敗時的行為</span>
                  </div>
                  <div class="fallback-row">
                    <label class="toggle-label">
                      <div class="toggle" :class="{ on: draft.fallback_enabled }" @click="draft.fallback_enabled = !draft.fallback_enabled">
                        <div class="toggle-thumb" />
                      </div>
                      <span>{{ draft.fallback_enabled ? '啟用備案' : '停用備案（失敗時直接報錯）' }}</span>
                    </label>
                  </div>

                  <template v-if="draft.fallback_enabled">
                    <div class="fallback-config">
                      <div class="fallback-group">
                        <label class="config-label">備案方案</label>
                        <div v-if="fallbackOptions.length === 0" class="no-fallback-hint">沒有其他可用方案</div>
                        <div v-else class="scheme-select-row">
                          <label
                            v-for="s in fallbackOptions"
                            :key="s.id"
                            class="scheme-select-item"
                            :class="{ selected: draft.fallback === s.id }"
                            :style="{ '--accent': schemeColor(s.id) }"
                          >
                            <input v-model="draft.fallback" type="radio" :value="s.id" class="scheme-radio" />
                            <span class="scheme-color-dot" />
                            <div>
                              <div class="scheme-select-label">{{ s.label }}</div>
                              <div class="scheme-select-desc">{{ s.description }}</div>
                            </div>
                          </label>
                        </div>
                      </div>
                      <div class="fallback-group">
                        <label class="config-label">觸發條件</label>
                        <div class="radio-row">
                          <label class="radio-option" :class="{ selected: draft.fallback_trigger === 'error' }">
                            <input v-model="draft.fallback_trigger" type="radio" value="error" class="scheme-radio" />
                            <div>
                              <span class="radio-label">連線失敗時</span>
                              <span class="radio-desc">主方案無法連線或回傳錯誤</span>
                            </div>
                          </label>
                          <label class="radio-option" :class="{ selected: draft.fallback_trigger === 'empty_data' }">
                            <input v-model="draft.fallback_trigger" type="radio" value="empty_data" class="scheme-radio" />
                            <div>
                              <span class="radio-label">資料為空時</span>
                              <span class="radio-desc">主方案成功但回傳空資料</span>
                            </div>
                          </label>
                        </div>
                      </div>
                    </div>
                    <!-- 流程預覽 -->
                    <div class="flow-preview">
                      <div class="flow-node primary-node" :style="{ '--c': schemeColor(draft.primary) }">
                        {{ selectedFeature.schemes.find(s => s.id === draft.primary)?.label ?? draft.primary }}
                      </div>
                      <div class="flow-arrow">
                        <span class="flow-arrow-label">{{ draft.fallback_trigger === 'error' ? '連線失敗' : '資料為空' }}</span>
                        →
                      </div>
                      <div v-if="draft.fallback" class="flow-node fallback-node" :style="{ '--c': schemeColor(draft.fallback) }">
                        {{ selectedFeature.schemes.find(s => s.id === draft.fallback)?.label ?? draft.fallback }}
                      </div>
                      <div v-else class="flow-node empty-node">（未選擇備案）</div>
                    </div>
                  </template>
                </section>

                <!-- 所有方案說明 -->
                <section class="setting-section">
                  <div class="section-badge-title">
                    <span class="step-chip">3</span>
                    所有方案說明
                  </div>
                  <table class="scheme-table">
                    <thead>
                      <tr><th>方案</th><th>說明</th><th>依賴</th><th>狀態</th></tr>
                    </thead>
                    <tbody>
                      <tr v-for="s in selectedFeature.schemes" :key="s.id">
                        <td>
                          <div class="scheme-table-id">
                            <span class="scheme-dot" :style="{ background: schemeColor(s.id) }" />
                            <span class="scheme-table-label">{{ s.label }}</span>
                          </div>
                          <code class="scheme-id-code">{{ s.id }}</code>
                        </td>
                        <td class="scheme-table-desc">{{ s.description }}</td>
                        <td>
                          <span v-if="s.need_service" class="need-service-badge">{{ s.need_service }}</span>
                          <span v-else class="no-dep">內建</span>
                        </td>
                        <td>
                          <span v-if="s.id === draft.primary" class="status-primary">主要</span>
                          <span v-else-if="s.id === draft.fallback && draft.fallback_enabled" class="status-fallback">備案</span>
                          <span v-else class="status-idle">備用</span>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </section>
              </template>
            </div>
          </div>
        </section>

        <!-- ════ 操作紀錄 ════ -->
        <section v-else-if="activeSection === 'logs'" class="section-wrap">
          <div class="section-header">
            <div>
              <h1 class="section-title">操作紀錄</h1>
              <p class="section-desc">所有按鈕觸發的 API 呼叫、原始回應與欄位分析</p>
            </div>
            <div class="log-actions">
              <div class="log-filter-group">
                <button
                  class="filter-btn"
                  :class="{ active: logFilter === 'all' }"
                  @click="logFilter = 'all'"
                >全部 {{ logEntries.length }}</button>
                <button
                  class="filter-btn"
                  :class="{ active: logFilter === 'success' }"
                  @click="logFilter = 'success'"
                >成功 {{ logEntries.filter(e => e.success).length }}</button>
                <button
                  class="filter-btn filter-btn--err"
                  :class="{ active: logFilter === 'error' }"
                  @click="logFilter = 'error'"
                >失敗 {{ logEntries.filter(e => !e.success).length }}</button>
              </div>
              <button class="btn-secondary btn-sm" :disabled="!logEntries.length" @click="clearLogs">
                清除全部
              </button>
            </div>
          </div>

          <!-- 搜尋 -->
          <div class="log-search-bar">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" class="search-icon"><circle cx="6.5" cy="6.5" r="4.5" stroke="currentColor" stroke-width="1.4"/><path d="M10 10l3.5 3.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
            <input v-model="logSearch" type="text" class="log-search" placeholder="搜尋操作名稱、端點…" />
          </div>

          <!-- 空狀態 -->
          <div v-if="!logEntries.length" class="log-empty">
            <svg width="40" height="40" viewBox="0 0 40 40" fill="none"><rect x="6" y="8" width="28" height="24" rx="4" stroke="currentColor" stroke-width="1.6"/><path d="M12 16h16M12 22h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
            <p>尚無操作紀錄</p>
            <span>回到主控台執行操作後，所有 API 呼叫會自動記錄在這裡</span>
          </div>

          <div v-else-if="!filteredLogs.length" class="log-empty">
            <p>沒有符合篩選條件的紀錄</p>
          </div>

          <!-- 紀錄列表 -->
          <div v-else class="log-list">
            <div
              v-for="entry in filteredLogs"
              :key="entry.id"
              class="log-entry"
              :class="{ expanded: expandedLogId === entry.id, 'log-entry--err': !entry.success }"
            >
              <!-- 主列（點擊展開） -->
              <div class="log-entry-row" @click="toggleLogEntry(entry.id)">
                <div class="log-entry-left">
                  <div class="log-status-dot" :class="entry.success ? 'dot-ok' : 'dot-err'" />
                  <div class="log-meta">
                    <div class="log-action">{{ entry.action }}</div>
                    <div class="log-sub">
                      <span class="log-trigger">{{ entry.trigger }}</span>
                      <span class="log-sep">·</span>
                      <span class="log-ts">{{ entry.timestamp }}</span>
                    </div>
                  </div>
                </div>
                <div class="log-entry-right">
                  <code class="log-method" :class="`method-${entry.method.toLowerCase()}`">{{ entry.method }}</code>
                  <code class="log-endpoint">{{ entry.endpoint }}</code>
                  <span v-if="entry.statusCode" class="log-status" :class="entry.success ? 'status-ok' : 'status-err'">
                    {{ entry.statusCode }}
                  </span>
                  <span class="log-duration">{{ entry.durationMs }}ms</span>
                  <svg class="expand-icon" width="14" height="14" viewBox="0 0 16 16" fill="none" :class="{ rotated: expandedLogId === entry.id }">
                    <path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                  </svg>
                </div>
              </div>

              <!-- 展開內容 -->
              <div v-if="expandedLogId === entry.id" class="log-entry-detail">
                <!-- 錯誤提示 -->
                <div v-if="entry.error" class="log-error-box">
                  <span class="log-error-label">錯誤</span>
                  {{ entry.error }}
                </div>

                <div class="log-detail-grid">
                  <!-- 請求 Body -->
                  <div v-if="entry.requestBody !== undefined" class="log-detail-block">
                    <div class="log-detail-label">
                      <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M4 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2Z" stroke="currentColor" stroke-width="1.3"/><path d="M6 8h4M6 6h4M6 10h2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
                      Request Body
                    </div>
                    <pre class="log-json">{{ formatJson(entry.requestBody) }}</pre>
                  </div>

                  <!-- 原始回應 -->
                  <div class="log-detail-block">
                    <div class="log-detail-label">
                      <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M2 4h12v8a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V4Z" stroke="currentColor" stroke-width="1.3"/><path d="M6 2h4v2H6V2Z" stroke="currentColor" stroke-width="1.3"/></svg>
                      原始回應（{{ entry.statusCode ?? '—' }}）
                    </div>
                    <pre class="log-json">{{ formatJson(entry.responseRaw) }}</pre>
                  </div>
                </div>

                <!-- 欄位分析 -->
                <div class="log-analysis-block">
                  <div class="log-detail-label">
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="1.3"/><path d="M8 7v4M8 5.5v.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
                    欄位說明與分析邏輯
                  </div>
                  <pre class="log-analysis">{{ entry.responseAnalysis }}</pre>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- ════ 資料庫 ════ -->
        <section v-else-if="activeSection === 'db'" class="section-wrap section-wrap--db">

          <!-- DB 標題列 -->
          <div class="db-header">
            <div class="db-header-left">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" class="db-header-icon" aria-hidden="true">
                <ellipse cx="8" cy="4" rx="6" ry="2.2" stroke="currentColor" stroke-width="1.4"/>
                <path d="M2 4v4c0 1.2 2.69 2.2 6 2.2s6-1 6-2.2V4" stroke="currentColor" stroke-width="1.4"/>
                <path d="M2 8v4c0 1.2 2.69 2.2 6 2.2s6-1 6-2.2V8" stroke="currentColor" stroke-width="1.4"/>
              </svg>
              <div>
                <h1 class="section-title">資料庫檢視</h1>
                <p class="section-desc">PostgreSQL · public schema · {{ tables.length }} 個資料表</p>
              </div>
            </div>
            <button class="btn-secondary" :disabled="dbLoading" @click="fetchDbTables">
              <svg width="13" height="13" viewBox="0 0 16 16" fill="none" :class="{ spin: dbLoading }">
                <path d="M13.65 2.35A8 8 0 1 0 14.9 8.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
                <path d="M14.5 2v4h-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              重新整理
            </button>
          </div>

          <div class="db-layout">

            <!-- ── 資料表清單側邊欄 ── -->
            <aside class="db-sidebar">
              <div v-if="dbLoading" class="db-sidebar-loading">
                <span class="spin-icon">◌</span> 載入中…
              </div>
              <template v-else>
                <div v-if="tables.length === 0" class="db-sidebar-empty">無資料表</div>
                <ul v-else class="db-table-list">
                  <li
                    v-for="t in tables"
                    :key="t.name"
                    class="db-table-item"
                    :class="{ active: t.name === selectedTable }"
                    @click="selectDbTable(t.name)"
                  >
                    <div class="db-table-item-accent" />
                    <div class="db-table-item-body">
                      <span class="db-table-name">{{ t.name }}</span>
                      <span class="db-table-count">{{ t.row_count.toLocaleString() }}</span>
                    </div>
                  </li>
                </ul>
              </template>
            </aside>

            <!-- ── 主內容區 ── -->
            <div class="db-content">

              <!-- 空狀態 -->
              <div v-if="!selectedTable" class="db-empty-state">
                <svg width="48" height="48" viewBox="0 0 48 48" fill="none" opacity="0.25">
                  <ellipse cx="24" cy="12" rx="17" ry="6" stroke="currentColor" stroke-width="2"/>
                  <path d="M7 12v12c0 3.3 7.6 6 17 6s17-2.7 17-6V12" stroke="currentColor" stroke-width="2"/>
                  <path d="M7 24v12c0 3.3 7.6 6 17 6s17-2.7 17-6V24" stroke="currentColor" stroke-width="2"/>
                </svg>
                <p class="db-empty-title">選擇資料表</p>
                <p class="db-empty-hint">從左側清單點選一個資料表開始瀏覽</p>
              </div>

              <template v-else>

                <!-- 工具列 -->
                <div class="db-toolbar">
                  <div class="db-toolbar-left">
                    <span class="db-toolbar-name">{{ selectedTable }}</span>
                    <span v-if="selectedTableInfo" class="db-toolbar-count">
                      {{ selectedTableInfo.row_count.toLocaleString() }} 列
                    </span>
                  </div>
                  <div class="db-toolbar-right">
                    <!-- 欄位搜尋（Schema 模式限定） -->
                    <div v-if="dbActiveTab === 'schema'" class="db-col-search">
                      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" class="db-search-icon">
                        <circle cx="6.5" cy="6.5" r="4.5" stroke="currentColor" stroke-width="1.4"/>
                        <path d="M10 10l3.5 3.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
                      </svg>
                      <input v-model="dbColSearch" class="db-col-input" placeholder="搜尋欄位名稱 / 型別…" />
                    </div>
                    <!-- Tab 切換 -->
                    <div class="db-tab-group">
                      <button
                        class="db-tab"
                        :class="{ active: dbActiveTab === 'data' }"
                        @click="dbActiveTab = 'data'; dbColSearch = ''"
                      >資料</button>
                      <button
                        class="db-tab"
                        :class="{ active: dbActiveTab === 'schema' }"
                        @click="dbActiveTab = 'schema'"
                      >結構</button>
                    </div>
                  </div>
                </div>

                <!-- ── Schema 結構 ── -->
                <div v-if="dbActiveTab === 'schema'" class="db-data-area">
                  <div class="db-table-wrap">
                    <div v-if="filteredColumns.length === 0" class="db-no-match">無符合欄位</div>
                    <table v-else class="data-table">
                      <thead>
                        <tr>
                          <th class="th-rownum">#</th>
                          <th>欄位名稱</th>
                          <th>資料型別</th>
                          <th>可 NULL</th>
                          <th>預設值</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="(col, idx) in filteredColumns" :key="col.name">
                          <td class="td-rownum">{{ idx + 1 }}</td>
                          <td class="col-name">{{ col.name }}</td>
                          <td>
                            <span class="type-badge" :class="typeClass(col.type)">{{ col.type }}</span>
                          </td>
                          <td>
                            <span v-if="col.nullable === 'YES'" class="nullable-yes">可空</span>
                            <span v-else class="nullable-no">必填</span>
                          </td>
                          <td class="col-default">{{ col.default || '—' }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </div>

                <!-- ── 資料瀏覽 ── -->
                <div v-else class="db-data-area">
                  <div class="db-table-wrap">
                    <div v-if="dbDataLoading" class="db-loading-overlay">
                      <span class="spin-icon">◌</span> 載入中…
                    </div>
                    <table v-else-if="tableData && dataColumns.length" class="data-table">
                      <thead>
                        <tr>
                          <th class="th-rownum">#</th>
                          <th v-for="col in dataColumns" :key="col">{{ col }}</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="(row, ri) in tableData.data" :key="ri">
                          <td class="td-rownum">{{ (dbPage - 1) * 50 + ri + 1 }}</td>
                          <td
                            v-for="col in dataColumns"
                            :key="col"
                            :class="{ 'null-cell': isNull(row[col]) }"
                          >
                            <span v-if="isNull(row[col])" class="null-pill">NULL</span>
                            <span v-else>{{ dbDisplayVal(row[col]) }}</span>
                          </td>
                        </tr>
                      </tbody>
                    </table>
                    <div v-else-if="!dbDataLoading" class="db-no-match">此資料表無資料</div>
                  </div>

                  <!-- 分頁列：固定在底部 -->
                  <div v-if="tableData && tableData.pages > 1" class="db-pagination">
                    <button class="page-btn" :disabled="dbPage <= 1" @click="goDbPage(dbPage - 1)">
                      <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M10 3L5 8l5 5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>
                      上一頁
                    </button>
                    <div class="page-center">
                      <span class="page-current">第 {{ dbPage }} / {{ tableData.pages }} 頁</span>
                      <span class="page-total">共 {{ tableData.total.toLocaleString() }} 筆</span>
                    </div>
                    <button class="page-btn" :disabled="dbPage >= tableData.pages" @click="goDbPage(dbPage + 1)">
                      下一頁
                      <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M6 3l5 5-5 5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>
                    </button>
                  </div>
                </div>

              </template>
            </div>
          </div>
        </section>

        <!-- ════ 排程管理 ════ -->
        <section v-else-if="activeSection === 'schedule'" class="section-wrap">
          <div class="section-header">
            <div>
              <h1 class="section-title">排程管理</h1>
              <p class="section-desc">設定各任務的自動執行時間，或立即手動觸發</p>
            </div>
            <button class="btn-secondary" :disabled="scheduleLoading" @click="fetchSchedules">
              <svg width="13" height="13" viewBox="0 0 16 16" fill="none" :class="{ spin: scheduleLoading }">
                <path d="M13.65 2.35A8 8 0 1 0 14.9 8.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
                <path d="M14.5 2v4h-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              重新整理
            </button>
          </div>

          <div v-if="scheduleError" class="error-banner">
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none"><path d="M8 1L1 14h14L8 1Z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/><path d="M8 7v3M8 12v.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
            {{ scheduleError }}
          </div>

          <div v-if="scheduleLoading" class="sched-loading">
            <span class="spin-icon">◌</span> 載入中…
          </div>

          <div v-else-if="schedules.length === 0" class="empty-state">
            <svg width="36" height="36" viewBox="0 0 36 36" fill="none"><rect x="4" y="6" width="28" height="26" rx="4" stroke="currentColor" stroke-width="1.6"/><path d="M11 2v6M25 2v6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/><path d="M4 16h28" stroke="currentColor" stroke-width="1.6"/></svg>
            <p>無排程資料</p>
          </div>

          <div v-else class="sched-grid">
            <div
              v-for="entry in schedules"
              :key="entry.id"
              class="sched-card"
              :class="{ 'sched-card--enabled': scheduleDrafts[entry.id]?.enabled }"
            >
              <!-- 卡片 Header -->
              <div class="sched-card-header">
                <div class="sched-task-info">
                  <span class="sched-task-label">{{ entry.label }}</span>
                  <span class="sched-task-id">{{ entry.id }}</span>
                </div>
                <div class="sched-header-actions">
                  <span v-if="scheduleSaved[entry.id]" class="sched-saved-badge">✓ 已儲存</span>
                  <!-- 啟用 Toggle -->
                  <label class="sched-toggle-label">
                    <div
                      class="toggle"
                      :class="{ on: scheduleDrafts[entry.id]?.enabled }"
                      @click="scheduleDrafts[entry.id] && (scheduleDrafts[entry.id].enabled = !scheduleDrafts[entry.id].enabled)"
                    >
                      <div class="toggle-thumb" />
                    </div>
                    <span>{{ scheduleDrafts[entry.id]?.enabled ? '已啟用' : '停用' }}</span>
                  </label>
                </div>
              </div>

              <p class="sched-task-desc">{{ entry.description }}</p>

              <!-- 排程設定（只有啟用才顯示） -->
              <template v-if="scheduleDrafts[entry.id]?.enabled">
                <div class="sched-divider" />

                <!-- 排程類型 -->
                <div class="sched-field">
                  <label class="sched-field-label">排程類型</label>
                  <div class="sched-type-group">
                    <button
                      v-for="t in ['manual', 'daily', 'weekly'] as const"
                      :key="t"
                      class="sched-type-btn"
                      :class="{ active: scheduleDrafts[entry.id]?.type === t }"
                      @click="scheduleDrafts[entry.id] && (scheduleDrafts[entry.id].type = t)"
                    >
                      {{ t === 'manual' ? '手動' : t === 'daily' ? '每日' : '每週' }}
                    </button>
                  </div>
                </div>

                <!-- 時間（daily / weekly） -->
                <div v-if="scheduleDrafts[entry.id]?.type !== 'manual'" class="sched-field sched-time-row">
                  <label class="sched-field-label">執行時間</label>
                  <div class="sched-time-inputs">
                    <input
                      v-model.number="scheduleDrafts[entry.id].hour"
                      type="number" min="0" max="23"
                      class="sched-num-input"
                      placeholder="HH"
                    />
                    <span class="sched-colon">:</span>
                    <input
                      v-model.number="scheduleDrafts[entry.id].minute"
                      type="number" min="0" max="59"
                      class="sched-num-input"
                      placeholder="MM"
                    />
                    <span class="sched-time-hint">（台北時間）</span>
                  </div>
                </div>

                <!-- 排除週末（daily 模式） -->
                <div v-if="scheduleDrafts[entry.id]?.type === 'daily'" class="sched-field">
                  <label class="sched-toggle-row">
                    <input
                      type="checkbox"
                      class="sched-checkbox"
                      :checked="scheduleDrafts[entry.id]?.exclude_weekends"
                      @change="scheduleDrafts[entry.id] && (scheduleDrafts[entry.id].exclude_weekends = ($event.target as HTMLInputElement).checked)"
                    />
                    <span class="sched-toggle-label">排除週末（週六、日不執行）</span>
                  </label>
                </div>

                <!-- 星期（weekly） -->
                <div v-if="scheduleDrafts[entry.id]?.type === 'weekly'" class="sched-field">
                  <label class="sched-field-label">執行星期</label>
                  <div class="sched-weekday-group">
                    <button
                      v-for="(name, idx) in weekdayNames"
                      :key="idx"
                      class="sched-weekday-btn"
                      :class="{ active: scheduleDrafts[entry.id]?.weekday === idx }"
                      @click="scheduleDrafts[entry.id] && (scheduleDrafts[entry.id].weekday = idx)"
                    >{{ name }}</button>
                  </div>
                </div>

                <!-- 天數參數（major_chips） -->
                <div v-if="entry.has_params" class="sched-field">
                  <label class="sched-field-label">天數（days）</label>
                  <div class="sched-time-inputs">
                    <input
                      v-model.number="scheduleDrafts[entry.id].days"
                      type="number" min="1" max="365"
                      class="sched-num-input sched-num-input--wide"
                      placeholder="1"
                    />
                    <span class="sched-time-hint">天</span>
                  </div>
                </div>
              </template>

              <!-- 執行資訊 -->
              <div class="sched-run-info">
                <div class="sched-run-item">
                  <span class="sched-run-label">上次執行</span>
                  <span class="sched-run-val">{{ fmtRunTime(entry.schedule.last_run_at) }}</span>
                </div>
                <div class="sched-run-item">
                  <span class="sched-run-label">下次執行</span>
                  <span class="sched-run-val" :class="{ 'sched-run-upcoming': !!entry.schedule.next_run_at }">
                    {{ fmtRunTime(entry.schedule.next_run_at) }}
                  </span>
                </div>
              </div>

              <!-- 操作按鈕 -->
              <div class="sched-card-footer">
                <button
                  class="btn-secondary btn-sm"
                  :disabled="scheduleRunning[entry.id]"
                  @click="runNow(entry)"
                >
                  <svg v-if="scheduleRunning[entry.id]" width="12" height="12" viewBox="0 0 16 16" fill="none" class="spin"><path d="M13.65 2.35A8 8 0 1 0 14.9 8.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
                  <svg v-else width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M5 3l8 5-8 5V3Z" fill="currentColor"/></svg>
                  {{ scheduleRunning[entry.id] ? '執行中…' : '立即執行' }}
                </button>
                <button
                  class="btn-primary"
                  :class="{ dirty: scheduleDraftDirty(entry) }"
                  :disabled="scheduleSaving[entry.id] || !scheduleDraftDirty(entry)"
                  @click="saveSchedule(entry)"
                >
                  {{ scheduleSaving[entry.id] ? '儲存中…' : '儲存設定' }}
                </button>
              </div>
            </div>
          </div>

          <!-- 非交易日管理 -->
          <div class="holidays-card">
            <div class="holidays-header">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <rect x="1" y="2" width="14" height="13" rx="2" stroke="currentColor" stroke-width="1.4"/>
                <path d="M1 6h14" stroke="currentColor" stroke-width="1.4"/>
                <path d="M5 1v2M11 1v2" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
                <path d="M7 9l1.5 1.5L11 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" opacity=".55"/>
              </svg>
              <span class="holidays-title">非交易日管理</span>
              <span class="holidays-badge">{{ holidays.length }} 天</span>
            </div>
            <p class="holidays-desc">列出不執行排程的非交易假日（例如颱風假、補假等）。每行一個日期，格式 <code class="inline-code">YYYY-MM-DD</code>。週末已由各任務的「排除週末」選項控制，此處無需重複填入。</p>

            <div class="holidays-body">
              <textarea
                v-model="holidaysDraft"
                class="holidays-textarea"
                placeholder="2026-01-01&#10;2026-02-06&#10;2026-02-28"
                spellcheck="false"
                :disabled="holidaysLoading"
              />
              <div class="holidays-chips" v-if="holidays.length">
                <span v-for="d in holidays" :key="d" class="holiday-chip">{{ d }}</span>
              </div>
            </div>

            <div class="holidays-footer">
              <span v-if="holidaysError" class="holidays-err">{{ holidaysError }}</span>
              <button
                class="btn-primary"
                :class="{ 'btn-saved': holidaysSaved }"
                :disabled="holidaysSaving || holidaysLoading"
                @click="saveHolidays"
              >
                {{ holidaysSaving ? '儲存中…' : holidaysSaved ? '已儲存 ✓' : '儲存假日' }}
              </button>
            </div>
          </div>
        </section>

      </main>
    </div>
  </div>
</template>

<style scoped>
/* ═══════════════════════════════════════
   Design Tokens
═══════════════════════════════════════ */
.page {
  --bg:    oklch(9.5%  0.018 256);
  --s1:    oklch(13%   0.020 257);
  --s2:    oklch(16.5% 0.022 258);
  --s3:    oklch(21%   0.024 258);
  --line:  oklch(22%   0.023 258);
  --line2: oklch(33%   0.023 258);
  --blue:  oklch(63%   0.20  264);
  --blue2: oklch(42%   0.17  264);
  --gold:  oklch(76%   0.13  82);
  --t1:    oklch(96%   0.006 218);
  --t2:    oklch(72%   0.013 240);
  --t3:    oklch(50%   0.012 240);
  --up:    oklch(62%   0.18  22);
  --dn:    oklch(64%   0.18  148);
  --warn:  oklch(73%   0.13  72);
  --radius: 10px;
  --font:  'DM Sans', system-ui, 'PingFang TC', 'Microsoft JhengHei', sans-serif;
  --mono:  'Fira Code', 'JetBrains Mono', ui-monospace, monospace;
  height: 100vh;
  overflow: hidden;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
  font-size: 14px;
  line-height: 1.55;
  display: flex;
  flex-direction: column;
  -webkit-font-smoothing: antialiased;
}
.page *, .page *::before, .page *::after { box-sizing: border-box; margin: 0; padding: 0; }

.page.light {
  --bg:    oklch(96.5% 0.009 220);
  --s1:    oklch(100%  0     0);
  --s2:    oklch(97%   0.010 220);
  --s3:    oklch(92%   0.014 220);
  --line:  oklch(88%   0.012 220);
  --line2: oklch(72%   0.015 240);
  --blue:  oklch(47%   0.21  264);
  --blue2: oklch(36%   0.17  264);
  --gold:  oklch(52%   0.16  72);
  --t1:    oklch(10%   0.018 256);
  --t2:    oklch(35%   0.016 240);
  --t3:    oklch(57%   0.012 240);
  --up:    oklch(44%   0.22  22);
  --dn:    oklch(38%   0.20  148);
  --warn:  oklch(50%   0.16  72);
}

/* ── Header ─────────────────────────────────────────────────── */
.header {
  flex-shrink: 0;
  z-index: 50;
  background: color-mix(in oklch, var(--s1) 92%, transparent);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--line);
}
.header__inner {
  padding: 0 20px; height: 52px;
  display: flex; align-items: center; justify-content: space-between;
}
.brand { display: flex; align-items: center; gap: 10px; }
.back-btn {
  display: flex; align-items: center; justify-content: center;
  width: 30px; height: 30px;
  background: var(--s2); border: 1px solid var(--line); border-radius: 7px;
  color: var(--t2); text-decoration: none; flex-shrink: 0;
  transition: background 0.15s, color 0.15s;
}
.back-btn:hover { background: var(--s3); color: var(--t1); }
.brand-divider { width: 1px; height: 18px; background: var(--line2); }
.brand-icon { flex-shrink: 0; }
.brand-info { display: flex; flex-direction: column; gap: 1px; }
.brand-main { font-size: 14px; font-weight: 700; letter-spacing: -0.01em; color: var(--t1); line-height: 1; }
.brand-sub { font-size: 10px; letter-spacing: 0.14em; text-transform: uppercase; color: var(--t3); line-height: 1; }
.header-nav { display: flex; align-items: center; gap: 10px; }
.log-count-badge {
  font-size: 11px; padding: 3px 10px; border-radius: 20px;
  background: color-mix(in oklch, var(--blue) 12%, var(--s2));
  border: 1px solid color-mix(in oklch, var(--blue) 25%, var(--line));
  color: var(--blue);
}

/* ── Admin Layout ────────────────────────────────────────────── */
.admin-layout {
  display: flex; flex: 1;
  min-height: 0; overflow: hidden;
}

/* ── Admin Nav (left sidebar) ────────────────────────────────── */
.admin-nav {
  width: 200px; min-width: 180px;
  background: var(--s1); border-right: 1px solid var(--line);
  padding: 16px 0; display: flex; flex-direction: column; gap: 4px;
  flex-shrink: 0; overflow-y: auto;
}
.nav-group-label {
  font-size: 10px; font-weight: 600; letter-spacing: 0.1em;
  text-transform: uppercase; color: var(--t3); padding: 0 16px 6px;
}
.nav-items { display: flex; flex-direction: column; gap: 2px; padding: 0 8px; }
.nav-item {
  display: flex; align-items: center; gap: 9px;
  padding: 8px 10px; border-radius: 7px;
  background: none; border: none; cursor: pointer;
  color: var(--t2); font-family: var(--font); font-size: 13px; font-weight: 500;
  text-align: left; width: 100%;
  transition: background 0.13s, color 0.13s;
  position: relative;
}
.nav-item:hover { background: var(--s2); color: var(--t1); }
.nav-item.active { background: color-mix(in oklch, var(--blue) 10%, var(--s2)); color: var(--blue); }
.nav-item.active svg { opacity: 1; }
.nav-badge {
  margin-left: auto; font-size: 10px; padding: 1px 6px; border-radius: 10px;
  background: color-mix(in oklch, var(--blue) 15%, var(--s3));
  color: var(--blue); font-weight: 600;
}
.nav-link-item {
  display: flex; align-items: center; gap: 9px;
  padding: 8px 10px; border-radius: 7px;
  color: var(--t3); text-decoration: none; font-size: 13px; font-weight: 500;
  transition: background 0.13s, color 0.13s;
}
.nav-link-item:hover { background: var(--s2); color: var(--t2); }
.nav-separator { height: 1px; background: var(--line); margin: 12px 16px; }

/* ── Admin Main ──────────────────────────────────────────────── */
.admin-main { flex: 1; min-width: 0; overflow-y: auto; }
.section-wrap { padding: 24px 28px; min-height: 100%; display: flex; flex-direction: column; gap: 20px; }
/* DB section：滿版鎖定，scroll 只發生在右側內容區 */
.section-wrap--db {
  height: 100%; padding: 0; gap: 0;
  display: flex; flex-direction: column; overflow: hidden;
}
.section-header {
  display: flex; align-items: flex-start; justify-content: space-between; gap: 16px;
  flex-shrink: 0;
}
.section-title { font-size: 19px; font-weight: 700; letter-spacing: -0.02em; color: var(--t1); line-height: 1.2; }
.section-desc { font-size: 12px; color: var(--t3); margin-top: 3px; }

/* ── Buttons ─────────────────────────────────────────────────── */
.btn-primary {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 7px 16px; border-radius: 8px;
  background: var(--s3); border: 1px solid var(--line2);
  color: var(--t2); font-family: var(--font); font-size: 13px; font-weight: 500;
  cursor: pointer; transition: all 0.15s;
}
.btn-primary:disabled { opacity: 0.35; cursor: default; }
.btn-primary.dirty { background: var(--blue); border-color: var(--blue); color: #fff; }
.btn-primary.dirty:hover { filter: brightness(1.1); }
.btn-secondary {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 7px 14px; border-radius: 8px;
  background: var(--s2); border: 1px solid var(--line);
  color: var(--t2); font-family: var(--font); font-size: 13px; font-weight: 500;
  cursor: pointer; transition: all 0.15s;
}
.btn-secondary:hover { background: var(--s3); border-color: var(--line2); color: var(--t1); }
.btn-secondary:disabled { opacity: 0.4; cursor: default; }
.btn-sm { padding: 5px 12px; font-size: 12px; }

/* ── Error Banner ────────────────────────────────────────────── */
.error-banner {
  display: flex; align-items: center; gap: 8px;
  background: color-mix(in oklch, var(--up) 8%, var(--s1));
  border-left: 3px solid var(--up);
  padding: 10px 16px; border-radius: 0 8px 8px 0;
  font-size: 13px; color: var(--up); flex-shrink: 0;
}

/* ── Saved Badge ─────────────────────────────────────────────── */
.saved-badge {
  font-size: 12px; color: var(--dn); font-weight: 600;
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: none; } }

/* ── Spin ────────────────────────────────────────────────────── */
@keyframes spin { to { transform: rotate(360deg); } }
.spin { animation: spin 0.9s linear infinite; transform-origin: center; }
.spin-icon { display: inline-block; animation: spin 1s linear infinite; }

/* ═══════════════════════════════════════
   Settings Section
═══════════════════════════════════════ */
.settings-layout { display: flex; gap: 0; flex: 1; overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); }
.feat-sidebar {
  width: 220px; min-width: 180px;
  background: var(--s1); border-right: 1px solid var(--line);
  padding: 0; overflow-y: auto; flex-shrink: 0;
}
.feat-sidebar-title {
  font-size: 10px; font-weight: 600; letter-spacing: 0.1em;
  text-transform: uppercase; color: var(--t3);
  padding: 14px 16px 8px;
}
.feat-loading { padding: 12px 16px; color: var(--t3); font-size: 13px; display: flex; align-items: center; gap: 8px; }
.feat-list { list-style: none; padding: 0 8px 8px; }
.feat-item {
  padding: 10px 10px; border-radius: 7px;
  cursor: pointer; transition: background 0.13s;
  margin-bottom: 2px;
}
.feat-item:hover { background: var(--s2); }
.feat-item.active { background: color-mix(in oklch, var(--blue) 10%, var(--s2)); }
.feat-item-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.feat-item-label { font-size: 13px; font-weight: 600; color: var(--t1); }
.badge-category {
  font-size: 10px; padding: 1px 6px; border-radius: 4px;
  background: var(--s3); border: 1px solid var(--line2); color: var(--t3);
  flex-shrink: 0;
}
.feat-item-scheme { display: flex; align-items: center; gap: 6px; margin-top: 4px; }
.scheme-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.scheme-id-text { font-size: 11px; font-family: var(--mono); color: var(--t3); }
.fallback-badge { font-size: 10px; padding: 0px 5px; border-radius: 4px; background: color-mix(in oklch, var(--gold) 15%, var(--s3)); color: var(--gold); border: 1px solid color-mix(in oklch, var(--gold) 30%, var(--line)); }

.feat-content { flex: 1; overflow-y: auto; padding: 20px 24px; display: flex; flex-direction: column; gap: 20px; }
.empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--t3); font-size: 13px; padding: 60px 20px; text-align: center; }
.empty-icon { opacity: 0.35; }

.feat-content-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.feat-title { font-size: 16px; font-weight: 700; color: var(--t1); }
.feat-desc { font-size: 12px; color: var(--t3); margin-top: 3px; }
.feat-header-actions { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }

.setting-section { display: flex; flex-direction: column; gap: 14px; }
.section-badge-title {
  display: flex; align-items: center; gap: 8px;
  font-size: 13px; font-weight: 600; color: var(--t1);
}
.step-chip {
  width: 20px; height: 20px; border-radius: 6px;
  background: var(--s3); border: 1px solid var(--line2);
  color: var(--t2); font-size: 11px; font-weight: 700;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0;
}
.section-hint { font-size: 11px; font-weight: 400; color: var(--t3); margin-left: 2px; }

/* Scheme Cards */
.scheme-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 10px; }
.scheme-card {
  display: flex; flex-direction: column; gap: 8px;
  padding: 12px; border-radius: 9px;
  background: var(--s2); border: 1.5px solid var(--line);
  cursor: pointer; transition: border-color 0.15s, background 0.15s;
}
.scheme-card.selected { border-color: var(--accent); background: color-mix(in oklch, var(--accent) 8%, var(--s2)); }
.scheme-radio { display: none; }
.scheme-card-header { display: flex; align-items: center; gap: 8px; }
.scheme-color-bar { width: 3px; height: 14px; border-radius: 2px; background: var(--accent); flex-shrink: 0; }
.scheme-label { font-size: 12px; font-weight: 600; color: var(--t1); }
.scheme-active-badge { margin-left: auto; font-size: 10px; padding: 1px 6px; border-radius: 4px; background: color-mix(in oklch, var(--accent) 15%, var(--s3)); color: var(--accent); border: 1px solid color-mix(in oklch, var(--accent) 25%, var(--line)); }
.scheme-desc { font-size: 11px; color: var(--t3); line-height: 1.45; }
.scheme-requires { font-size: 11px; color: var(--t3); }
.scheme-requires code { font-family: var(--mono); color: var(--warn); font-size: 10px; }

/* Fallback */
.fallback-row { display: flex; align-items: center; gap: 12px; }
.toggle-label { display: flex; align-items: center; gap: 10px; font-size: 13px; color: var(--t2); cursor: pointer; }
.toggle { width: 36px; height: 20px; border-radius: 10px; background: var(--s3); border: 1px solid var(--line2); position: relative; transition: background 0.2s; }
.toggle.on { background: var(--blue); border-color: var(--blue); }
.toggle-thumb { position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: #fff; transition: transform 0.2s; box-shadow: 0 1px 3px rgba(0,0,0,0.2); }
.toggle.on .toggle-thumb { transform: translateX(16px); }

.fallback-config { display: flex; flex-direction: column; gap: 14px; }
.fallback-group { display: flex; flex-direction: column; gap: 8px; }
.config-label { font-size: 11px; font-weight: 600; color: var(--t3); text-transform: uppercase; letter-spacing: 0.08em; }
.scheme-select-row { display: flex; flex-direction: column; gap: 6px; }
.scheme-select-item { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 8px; background: var(--s2); border: 1.5px solid var(--line); cursor: pointer; transition: border-color 0.15s; }
.scheme-select-item.selected { border-color: var(--accent); }
.scheme-color-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--accent); flex-shrink: 0; }
.scheme-select-label { font-size: 12px; font-weight: 600; color: var(--t1); }
.scheme-select-desc { font-size: 11px; color: var(--t3); }

.radio-row { display: flex; flex-direction: column; gap: 6px; }
.radio-option { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 8px; background: var(--s2); border: 1.5px solid var(--line); cursor: pointer; transition: border-color 0.15s; }
.radio-option.selected { border-color: var(--blue); }
.radio-label { font-size: 13px; font-weight: 600; color: var(--t1); display: block; }
.radio-desc { font-size: 11px; color: var(--t3); display: block; }
.no-fallback-hint { font-size: 12px; color: var(--t3); }

/* Flow Preview */
.flow-preview { display: flex; align-items: center; gap: 10px; padding: 14px 16px; background: var(--s2); border: 1px solid var(--line); border-radius: 9px; }
.flow-node { padding: 6px 12px; border-radius: 6px; font-size: 12px; font-weight: 600; border: 1px solid color-mix(in oklch, var(--c) 30%, var(--line)); background: color-mix(in oklch, var(--c) 10%, var(--s3)); color: var(--c); }
.empty-node { border-color: var(--line2); color: var(--t3); background: var(--s3); font-weight: 400; font-style: italic; }
.flow-arrow { display: flex; flex-direction: column; align-items: center; gap: 2px; color: var(--t3); }
.flow-arrow-label { font-size: 10px; color: var(--t3); white-space: nowrap; }

/* Scheme Table */
.scheme-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.scheme-table th { text-align: left; padding: 8px 12px; font-size: 10px; font-weight: 600; letter-spacing: 0.07em; text-transform: uppercase; color: var(--t3); border-bottom: 1px solid var(--line); }
.scheme-table td { padding: 9px 12px; border-bottom: 1px solid oklch(from var(--line) l c h / 50%); }
.scheme-table tr:last-child td { border-bottom: none; }
.scheme-table-id { display: flex; align-items: center; gap: 6px; margin-bottom: 3px; }
.scheme-table-label { font-size: 12px; font-weight: 600; color: var(--t1); }
.scheme-id-code { font-family: var(--mono); font-size: 10px; color: var(--t3); }
.scheme-table-desc { color: var(--t2); max-width: 280px; line-height: 1.4; }
.need-service-badge { font-size: 10px; padding: 2px 7px; border-radius: 4px; background: color-mix(in oklch, var(--warn) 12%, var(--s3)); color: var(--warn); border: 1px solid color-mix(in oklch, var(--warn) 25%, var(--line)); font-family: var(--mono); }
.no-dep { font-size: 11px; color: var(--t3); }
.status-primary { font-size: 11px; padding: 2px 8px; border-radius: 4px; background: color-mix(in oklch, var(--blue) 12%, var(--s3)); color: var(--blue); }
.status-fallback { font-size: 11px; padding: 2px 8px; border-radius: 4px; background: color-mix(in oklch, var(--gold) 12%, var(--s3)); color: var(--gold); }
.status-idle { font-size: 11px; color: var(--t3); }

/* ═══════════════════════════════════════
   Logs Section
═══════════════════════════════════════ */
.log-actions { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.log-filter-group { display: flex; border: 1px solid var(--line); border-radius: 8px; overflow: hidden; }
.filter-btn {
  padding: 5px 12px; border: none; border-right: 1px solid var(--line);
  background: var(--s2); color: var(--t2); font-family: var(--font); font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.13s;
}
.filter-btn:last-child { border-right: none; }
.filter-btn.active { background: color-mix(in oklch, var(--blue) 12%, var(--s3)); color: var(--blue); }
.filter-btn--err.active { background: color-mix(in oklch, var(--up) 12%, var(--s3)); color: var(--up); }

.log-search-bar {
  display: flex; align-items: center; gap: 8px;
  background: var(--s2); border: 1px solid var(--line); border-radius: 8px;
  padding: 8px 12px; flex-shrink: 0;
}
.search-icon { opacity: 0.4; flex-shrink: 0; }
.log-search {
  flex: 1; background: none; border: none; outline: none;
  font-family: var(--font); font-size: 13px; color: var(--t1);
}
.log-search::placeholder { color: var(--t3); }

.log-empty {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 10px; color: var(--t3); text-align: center; padding: 60px 20px;
}
.log-empty svg { opacity: 0.3; }
.log-empty p { font-size: 14px; font-weight: 500; color: var(--t2); }
.log-empty span { font-size: 12px; max-width: 360px; }

.log-list { display: flex; flex-direction: column; gap: 0; }

.log-entry {
  border: 1px solid var(--line); border-radius: 9px;
  overflow: hidden; margin-bottom: 6px;
  background: var(--s1);
  transition: border-color 0.13s;
}
.log-entry.expanded { border-color: var(--line2); }
.log-entry--err { border-color: color-mix(in oklch, var(--up) 25%, var(--line)); }

.log-entry-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px; cursor: pointer; gap: 12px;
  transition: background 0.1s;
}
.log-entry-row:hover { background: var(--s2); }

.log-entry-left { display: flex; align-items: center; gap: 10px; min-width: 0; }
.log-status-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot-ok { background: var(--dn); box-shadow: 0 0 5px color-mix(in oklch, var(--dn) 40%, transparent); }
.dot-err { background: var(--up); box-shadow: 0 0 5px color-mix(in oklch, var(--up) 40%, transparent); }
.log-meta { min-width: 0; }
.log-action { font-size: 13px; font-weight: 600; color: var(--t1); }
.log-sub { display: flex; align-items: center; gap: 6px; margin-top: 2px; }
.log-trigger { font-size: 11px; color: var(--t3); }
.log-sep { color: var(--line2); }
.log-ts { font-size: 11px; color: var(--t3); font-family: var(--mono); }

.log-entry-right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.log-method {
  font-family: var(--mono); font-size: 10px; font-weight: 600;
  padding: 2px 6px; border-radius: 4px;
}
.method-get { color: var(--dn); background: color-mix(in oklch, var(--dn) 12%, var(--s2)); }
.method-post { color: var(--blue); background: color-mix(in oklch, var(--blue) 12%, var(--s2)); }
.method-put { color: var(--warn); background: color-mix(in oklch, var(--warn) 12%, var(--s2)); }
.method-delete { color: var(--up); background: color-mix(in oklch, var(--up) 12%, var(--s2)); }
.log-endpoint { font-family: var(--mono); font-size: 11px; color: var(--t2); max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-status { font-size: 11px; font-family: var(--mono); padding: 2px 6px; border-radius: 4px; }
.status-ok { color: var(--dn); background: color-mix(in oklch, var(--dn) 10%, var(--s2)); }
.status-err { color: var(--up); background: color-mix(in oklch, var(--up) 10%, var(--s2)); }
.log-duration { font-size: 11px; color: var(--t3); font-family: var(--mono); white-space: nowrap; }
.expand-icon { color: var(--t3); transition: transform 0.2s; }
.expand-icon.rotated { transform: rotate(180deg); }

/* Log Detail */
.log-entry-detail {
  border-top: 1px solid var(--line);
  padding: 14px 14px 16px;
  display: flex; flex-direction: column; gap: 12px;
  background: color-mix(in oklch, var(--s1) 60%, var(--bg));
}
.log-error-box {
  display: flex; align-items: flex-start; gap: 8px;
  background: color-mix(in oklch, var(--up) 8%, var(--s2));
  border: 1px solid color-mix(in oklch, var(--up) 25%, var(--line));
  border-radius: 7px; padding: 10px 12px;
  font-size: 12px; color: var(--up); line-height: 1.5;
}
.log-error-label {
  font-size: 10px; font-weight: 700; letter-spacing: 0.07em;
  text-transform: uppercase; background: var(--up); color: #fff;
  padding: 2px 6px; border-radius: 4px; flex-shrink: 0;
}
.log-detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
@media (max-width: 900px) { .log-detail-grid { grid-template-columns: 1fr; } }
.log-detail-block { display: flex; flex-direction: column; gap: 6px; }
.log-detail-label {
  display: flex; align-items: center; gap: 5px;
  font-size: 10px; font-weight: 600; letter-spacing: 0.08em;
  text-transform: uppercase; color: var(--t3);
}
.log-json {
  background: var(--s2); border: 1px solid var(--line); border-radius: 7px;
  padding: 10px 12px; font-family: var(--mono); font-size: 11px;
  color: var(--t2); white-space: pre-wrap; word-break: break-all;
  max-height: 200px; overflow-y: auto; line-height: 1.55;
}
.log-analysis-block { display: flex; flex-direction: column; gap: 6px; }
.log-analysis {
  background: color-mix(in oklch, var(--blue) 4%, var(--s2));
  border: 1px solid color-mix(in oklch, var(--blue) 15%, var(--line));
  border-radius: 7px; padding: 12px 14px;
  font-family: var(--mono); font-size: 11px;
  color: var(--t2); white-space: pre-wrap; line-height: 1.7;
}

/* ═══════════════════════════════════════
   DB Viewer Section
═══════════════════════════════════════ */

/* DB 標題列 */
.db-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 22px; border-bottom: 1px solid var(--line);
  background: var(--s1); flex-shrink: 0;
}
.db-header-left { display: flex; align-items: center; gap: 12px; }
.db-header-icon { color: var(--t3); flex-shrink: 0; }

/* 主布局：填滿剩餘高度，內部 overflow: hidden 讓子元素各自控制捲動 */
.db-layout { display: flex; flex: 1; min-height: 0; overflow: hidden; }

/* 側邊欄 ─ 資料表清單 */
.db-sidebar {
  width: 230px; min-width: 200px; flex-shrink: 0;
  background: var(--s1); border-right: 1px solid var(--line);
  overflow-y: auto; display: flex; flex-direction: column;
}
.db-sidebar-loading {
  padding: 20px 16px; color: var(--t3); font-size: 12px;
  display: flex; align-items: center; gap: 8px;
}
.db-sidebar-empty { padding: 20px 16px; font-size: 12px; color: var(--t3); }

.db-table-list { list-style: none; padding: 8px; }
.db-table-item {
  display: flex; align-items: stretch;
  border-radius: 8px; cursor: pointer;
  transition: background 0.12s; margin-bottom: 2px;
  overflow: hidden;
  border: 1px solid transparent;
}
.db-table-item:hover { background: var(--s2); }
.db-table-item.active {
  background: color-mix(in oklch, var(--blue) 8%, var(--s2));
  border-color: color-mix(in oklch, var(--blue) 20%, var(--line));
}
.db-table-item-accent {
  width: 3px; flex-shrink: 0; border-radius: 4px 0 0 4px;
  background: transparent;
  transition: background 0.12s;
}
.db-table-item.active .db-table-item-accent { background: var(--blue); }
.db-table-item-body {
  display: flex; align-items: center; justify-content: space-between;
  gap: 8px; flex: 1; padding: 8px 10px;
}
.db-table-name {
  font-size: 12px; font-weight: 500; color: var(--t1);
  font-family: var(--mono); min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.db-table-item.active .db-table-name { color: var(--blue); }
.db-table-count {
  font-size: 10px; color: var(--t3); flex-shrink: 0;
  background: var(--s3); border: 1px solid var(--line);
  padding: 1px 6px; border-radius: 10px; font-family: var(--mono);
}
.db-table-item.active .db-table-count {
  background: color-mix(in oklch, var(--blue) 12%, var(--s2));
  border-color: color-mix(in oklch, var(--blue) 25%, var(--line));
  color: var(--blue);
}

/* 主內容區 */
.db-content { flex: 1; overflow: hidden; display: flex; flex-direction: column; min-width: 0; }

/* 空狀態 */
.db-empty-state {
  flex: 1; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 10px;
  padding: 60px 20px; text-align: center;
}
.db-empty-title { font-size: 15px; font-weight: 600; color: var(--t2); }
.db-empty-hint { font-size: 12px; color: var(--t3); max-width: 260px; line-height: 1.5; }

/* 工具列 */
.db-toolbar {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 10px 16px; border-bottom: 1px solid var(--line);
  background: color-mix(in oklch, var(--s2) 70%, var(--s1));
  flex-shrink: 0;
}
.db-toolbar-left { display: flex; align-items: center; gap: 10px; min-width: 0; }
.db-toolbar-name {
  font-family: var(--mono); font-size: 13px; font-weight: 600; color: var(--t1);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.db-toolbar-count {
  font-size: 11px; color: var(--t3); white-space: nowrap;
  background: var(--s3); padding: 2px 8px; border-radius: 10px;
  border: 1px solid var(--line);
}
.db-toolbar-right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }

/* 欄位搜尋 */
.db-col-search {
  display: flex; align-items: center; gap: 6px;
  background: var(--s1); border: 1px solid var(--line); border-radius: 7px;
  padding: 5px 10px;
}
.db-search-icon { opacity: 0.4; flex-shrink: 0; }
.db-col-input {
  background: none; border: none; outline: none;
  font-family: var(--font); font-size: 12px; color: var(--t1); width: 160px;
}
.db-col-input::placeholder { color: var(--t3); }

/* Tab 切換 */
.db-tab-group {
  display: flex; border: 1px solid var(--line); border-radius: 8px; overflow: hidden;
}
.db-tab {
  padding: 5px 14px; border: none; border-right: 1px solid var(--line);
  background: var(--s2); color: var(--t3);
  font-family: var(--font); font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.12s;
}
.db-tab:last-child { border-right: none; }
.db-tab.active { background: var(--s3); color: var(--t1); }

/* 資料表區域 */
.db-data-area { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-height: 0; }
.db-table-wrap { flex: 1; overflow: auto; position: relative; min-height: 0; }
.db-loading-overlay {
  padding: 24px; color: var(--t3); font-size: 12px;
  display: flex; align-items: center; gap: 8px;
}
.db-no-match { padding: 24px; color: var(--t3); font-size: 13px; }

/* 資料表 */
.data-table {
  width: max-content; min-width: 100%;
  border-collapse: collapse; font-size: 12px;
}
.data-table thead { position: sticky; top: 0; z-index: 2; }
.data-table th {
  text-align: left; padding: 9px 14px;
  font-size: 10px; font-weight: 600; letter-spacing: 0.06em;
  text-transform: uppercase; color: var(--t3);
  background: var(--s2);
  border-bottom: 2px solid var(--line);
  white-space: nowrap;
}
.data-table th.th-rownum {
  color: var(--t3); width: 46px; text-align: right;
  background: var(--s2); border-right: 1px solid var(--line);
}
.data-table tbody tr {
  transition: background 0.1s;
}
.data-table tbody tr:nth-child(even) td {
  background: color-mix(in oklch, var(--s2) 40%, transparent);
}
.data-table tbody tr:hover td { background: color-mix(in oklch, var(--blue) 6%, var(--s2)); }
.data-table td {
  padding: 7px 14px;
  border-bottom: 1px solid color-mix(in oklch, var(--line) 60%, transparent);
  color: var(--t1); font-family: var(--mono);
  white-space: nowrap; max-width: 280px;
  overflow: hidden; text-overflow: ellipsis; font-size: 11px;
  vertical-align: middle;
}
.td-rownum {
  text-align: right; color: var(--t3) !important;
  font-size: 10px !important; padding-right: 10px !important;
  user-select: none;
  border-right: 1px solid var(--line);
  background: var(--s2) !important;
}
.null-cell { }
.null-pill {
  display: inline-block; font-size: 9px; font-weight: 600;
  letter-spacing: 0.08em; text-transform: uppercase;
  padding: 1px 5px; border-radius: 4px;
  background: var(--s3); border: 1px solid var(--line2);
  color: var(--t3); font-family: var(--mono);
}

/* Schema 欄位樣式 */
.col-name {
  font-family: var(--mono); font-size: 12px; font-weight: 600; color: var(--t1);
}
.col-default { font-family: var(--mono); font-size: 11px; color: var(--t3); }

/* 型別 Badge */
.type-badge {
  display: inline-block; font-size: 10px; font-family: var(--mono); font-weight: 500;
  padding: 2px 7px; border-radius: 5px; white-space: nowrap;
  border: 1px solid;
}
.type-int    { color: oklch(65% 0.19 264); background: oklch(65% 0.19 264 / 10%); border-color: oklch(65% 0.19 264 / 22%); }
.type-str    { color: oklch(65% 0.17 148); background: oklch(65% 0.17 148 / 10%); border-color: oklch(65% 0.17 148 / 22%); }
.type-bool   { color: oklch(72% 0.14 82);  background: oklch(72% 0.14 82  / 10%); border-color: oklch(72% 0.14 82  / 22%); }
.type-time   { color: oklch(65% 0.17 200); background: oklch(65% 0.17 200 / 10%); border-color: oklch(65% 0.17 200 / 22%); }
.type-num    { color: oklch(68% 0.17 320); background: oklch(68% 0.17 320 / 10%); border-color: oklch(68% 0.17 320 / 22%); }
.type-json   { color: oklch(66% 0.17 50);  background: oklch(66% 0.17 50  / 10%); border-color: oklch(66% 0.17 50  / 22%); }
.type-other  { color: var(--t3); background: var(--s3); border-color: var(--line2); }

/* NULL 可空 Badge */
.nullable-yes {
  font-size: 10px; padding: 1px 6px; border-radius: 4px;
  background: color-mix(in oklch, var(--warn) 10%, var(--s2));
  border: 1px solid color-mix(in oklch, var(--warn) 22%, var(--line));
  color: var(--warn);
}
.nullable-no {
  font-size: 10px; padding: 1px 6px; border-radius: 4px;
  background: var(--s3); border: 1px solid var(--line2);
  color: var(--t3);
}

/* 分頁列 */
.db-pagination {
  display: flex; align-items: center; justify-content: space-between;
  padding: 11px 16px; border-top: 1px solid var(--line);
  background: color-mix(in oklch, var(--s2) 60%, var(--s1));
  flex-shrink: 0;
}
.page-btn {
  display: flex; align-items: center; gap: 5px;
  padding: 6px 14px; border-radius: 7px;
  border: 1px solid var(--line); background: var(--s2);
  color: var(--t2); cursor: pointer; font-size: 12px;
  font-family: var(--font); transition: all 0.12s;
}
.page-btn:hover:not(:disabled) { background: var(--s3); border-color: var(--line2); color: var(--t1); }
.page-btn:disabled { opacity: 0.3; cursor: default; }
.page-center { display: flex; flex-direction: column; align-items: center; gap: 2px; }
.page-current { font-size: 12px; font-weight: 600; color: var(--t2); font-family: var(--mono); }
.page-total { font-size: 10px; color: var(--t3); font-family: var(--mono); }

/* ═══════════════════════════════════════
   Schedule Section
═══════════════════════════════════════ */
.sched-loading { padding: 20px; color: var(--t3); font-size: 13px; display: flex; align-items: center; gap: 8px; }

.sched-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 14px;
  align-items: start;
}

.sched-card {
  background: var(--s1);
  border: 1.5px solid var(--line);
  border-radius: 12px;
  padding: 18px 20px;
  display: flex; flex-direction: column; gap: 12px;
  transition: border-color 0.15s;
}
.sched-card--enabled { border-color: color-mix(in oklch, var(--blue) 30%, var(--line)); }

.sched-card-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.sched-task-info { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.sched-task-label { font-size: 14px; font-weight: 700; color: var(--t1); }
.sched-task-id { font-size: 10px; font-family: var(--mono); color: var(--t3); }
.sched-task-desc { font-size: 12px; color: var(--t3); line-height: 1.5; margin-top: -4px; }

.sched-header-actions { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.sched-toggle-label { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--t2); cursor: pointer; }
.sched-saved-badge { font-size: 11px; color: var(--dn); font-weight: 600; }

.sched-divider { height: 1px; background: var(--line); margin: 0 -20px; }

.sched-field { display: flex; flex-direction: column; gap: 7px; }
.sched-field-label {
  font-size: 10px; font-weight: 600; letter-spacing: 0.08em;
  text-transform: uppercase; color: var(--t3);
}

.sched-type-group { display: flex; gap: 4px; }
.sched-type-btn {
  flex: 1; padding: 6px 0; border-radius: 7px;
  border: 1.5px solid var(--line);
  background: var(--s2); color: var(--t2);
  font-family: var(--font); font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.13s;
}
.sched-type-btn.active {
  background: color-mix(in oklch, var(--blue) 12%, var(--s2));
  border-color: var(--blue); color: var(--blue);
}

.sched-time-row .sched-time-inputs { display: flex; align-items: center; gap: 6px; }
.sched-time-inputs { display: flex; align-items: center; gap: 6px; }
.sched-num-input {
  width: 52px; padding: 6px 8px;
  background: var(--s2); border: 1px solid var(--line); border-radius: 7px;
  color: var(--t1); font-family: var(--mono); font-size: 13px;
  text-align: center; outline: none;
  transition: border-color 0.13s;
}
.sched-num-input:focus { border-color: var(--blue); }
.sched-num-input--wide { width: 72px; }
.sched-colon { font-size: 16px; font-weight: 600; color: var(--t2); }
.sched-time-hint { font-size: 11px; color: var(--t3); }

.sched-weekday-group { display: flex; gap: 4px; flex-wrap: wrap; }
.sched-weekday-btn {
  padding: 5px 9px; border-radius: 6px;
  border: 1.5px solid var(--line);
  background: var(--s2); color: var(--t2);
  font-family: var(--font); font-size: 11px; font-weight: 500;
  cursor: pointer; transition: all 0.13s;
}
.sched-weekday-btn.active {
  background: color-mix(in oklch, var(--gold) 12%, var(--s2));
  border-color: var(--gold); color: var(--gold);
}

.sched-run-info {
  display: flex; gap: 16px;
  padding: 10px 12px;
  background: var(--s2); border: 1px solid var(--line);
  border-radius: 8px;
}
.sched-run-item { display: flex; flex-direction: column; gap: 3px; flex: 1; }
.sched-run-label { font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.07em; color: var(--t3); }
.sched-run-val { font-size: 12px; font-family: var(--mono); color: var(--t2); }
.sched-run-upcoming { color: var(--blue); }

.sched-card-footer { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding-top: 4px; }

/* ── 排除週末 toggle ─────────────────────────────────────────── */
.sched-toggle-row {
  display: flex; align-items: center; gap: 8px; cursor: pointer;
  user-select: none;
}
.sched-checkbox {
  width: 15px; height: 15px; accent-color: var(--blue); cursor: pointer;
  flex-shrink: 0;
}
.sched-toggle-label { font-size: 13px; color: var(--t1); }

/* ── 非交易日管理 ──────────────────────────────────────────────── */
.holidays-card {
  margin-top: 20px;
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 16px 20px;
  display: flex; flex-direction: column; gap: 12px;
}
.holidays-header {
  display: flex; align-items: center; gap: 8px;
  color: var(--warn);
}
.holidays-title { font-weight: 600; font-size: 14px; color: var(--t1); }
.holidays-badge {
  margin-left: auto;
  background: color-mix(in oklch, var(--warn) 15%, transparent);
  color: var(--warn);
  border: 1px solid color-mix(in oklch, var(--warn) 30%, transparent);
  border-radius: 20px;
  padding: 2px 10px; font-size: 11px; font-weight: 600;
}
.holidays-desc { font-size: 12px; color: var(--t2); line-height: 1.6; }
.inline-code {
  font-family: var(--mono); font-size: 11px;
  background: var(--s3); border-radius: 4px; padding: 1px 5px;
  color: var(--blue);
}
.holidays-body { display: flex; gap: 12px; align-items: flex-start; }
.holidays-textarea {
  flex: 1; min-height: 100px; max-height: 200px;
  background: var(--s1); border: 1px solid var(--line2);
  color: var(--t1); border-radius: 8px; padding: 10px 12px;
  font-family: var(--mono); font-size: 12px; line-height: 1.7;
  resize: vertical; outline: none;
  transition: border-color .15s;
}
.holidays-textarea:focus { border-color: var(--blue); }
.holidays-textarea:disabled { opacity: .5; cursor: not-allowed; }
.holidays-chips {
  display: flex; flex-direction: column; gap: 4px;
  max-height: 200px; overflow-y: auto;
  min-width: 110px;
}
.holiday-chip {
  background: color-mix(in oklch, var(--warn) 12%, transparent);
  border: 1px solid color-mix(in oklch, var(--warn) 25%, transparent);
  color: var(--warn);
  border-radius: 6px; padding: 2px 8px;
  font-family: var(--mono); font-size: 11px; white-space: nowrap;
}
.holidays-footer {
  display: flex; align-items: center; justify-content: flex-end; gap: 12px;
}
.holidays-err { font-size: 12px; color: var(--up); margin-right: auto; }
.btn-saved { background: var(--dn) !important; }
</style>
