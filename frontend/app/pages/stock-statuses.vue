<script setup lang="ts">
import { useAppPrefs } from '~/composables/useAppPrefs'

type StatusType = '' | 'disposition' | 'attention' | 'day_trade_restricted'
type MarketType = '' | 'TWSE' | 'TPEX'

interface StockStatus {
  id: number
  symbol: string
  name: string
  market: string
  type: Exclude<StatusType, ''>
  source_date: string
  start_date: string
  end_date: string
  reason: string
  measure: string
  detail: string
  raw_period: string
  source_url: string
  fetched_at: string
}

interface StockStatusesResponse {
  as_of: string
  count: number
  data: StockStatus[]
}

interface SyncStatus {
  id?: number
  status: 'never' | 'running' | 'completed' | 'failed'
  running?: boolean
  started_at?: string
  completed_at?: string | null
  total?: number
  message?: string
}

const { isDark, isClassic, toggleTheme } = useAppPrefs()

const selectedType = ref<StatusType>('')
const selectedMarket = ref<MarketType>('')
const searchQuery = ref('')
const asOf = ref(new Date().toISOString().slice(0, 10))
const rows = ref<StockStatus[]>([])
const responseAsOf = ref('')
const loading = ref(true)
const syncLoading = ref(false)
const syncError = ref('')
const syncStatus = ref<SyncStatus>({ status: 'never' })

const typeTabs: { value: StatusType; label: string }[] = [
  { value: '', label: '全部' },
  { value: 'disposition', label: '處置股' },
  { value: 'attention', label: '注意股' },
  { value: 'day_trade_restricted', label: '限當沖股' },
]

const marketTabs: { value: MarketType; label: string }[] = [
  { value: '', label: '上市 + 上櫃' },
  { value: 'TWSE', label: '上市' },
  { value: 'TPEX', label: '上櫃' },
]

async function fetchStatuses() {
  loading.value = true
  try {
    const query: Record<string, string> = { as_of: asOf.value }
    if (selectedType.value) query.type = selectedType.value
    if (selectedMarket.value) query.market = selectedMarket.value
    const res = await $fetch<StockStatusesResponse>('/api/stock-statuses', { query })
    rows.value = Array.isArray(res?.data) ? res.data : []
    responseAsOf.value = res?.as_of ?? asOf.value
  } catch {
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function fetchSyncStatus() {
  try {
    syncStatus.value = await $fetch<SyncStatus>('/api/stock-statuses/status')
  } catch {
    syncStatus.value = { status: 'failed', message: '無法取得同步狀態' }
  }
}

async function triggerSync() {
  if (syncLoading.value || syncStatus.value.running) return
  syncLoading.value = true
  syncError.value = ''
  try {
    await $fetch('/api/stock-statuses/sync', { method: 'POST' })
    await Promise.all([fetchSyncStatus(), fetchStatuses()])
  } catch (err: any) {
    syncError.value = err?.data?.error || err?.response?._data?.error || '同步失敗'
    await fetchSyncStatus()
  } finally {
    syncLoading.value = false
  }
}

const filteredRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(row =>
    row.symbol.toLowerCase().includes(q) ||
    row.name.toLowerCase().includes(q) ||
    row.reason.toLowerCase().includes(q) ||
    row.measure.toLowerCase().includes(q)
  )
})

const counts = computed(() => {
  const result = { disposition: 0, attention: 0, day_trade_restricted: 0 }
  for (const row of rows.value) result[row.type]++
  return result
})

const syncBadgeText = computed(() => {
  if (syncLoading.value || syncStatus.value.running) return '同步中'
  if (syncStatus.value.status === 'completed') return '已同步'
  if (syncStatus.value.status === 'failed') return '同步失敗'
  return '尚未同步'
})

function statusLabel(type: StatusType): string {
  if (type === 'disposition') return '處置股'
  if (type === 'attention') return '注意股'
  if (type === 'day_trade_restricted') return '限當沖股'
  return '全部'
}

function statusClass(type: StatusType): string {
  if (type === 'disposition') return 'status-chip--disposition'
  if (type === 'attention') return 'status-chip--attention'
  if (type === 'day_trade_restricted') return 'status-chip--daytrade'
  return ''
}

function formatDate(value?: string | null): string {
  if (!value) return '—'
  return value.split('T')[0] ?? value
}

function rowReason(row: StockStatus): string {
  return row.reason || row.measure || row.detail || '—'
}

watch([selectedType, selectedMarket, asOf], fetchStatuses)

onMounted(async () => {
  await Promise.all([fetchStatuses(), fetchSyncStatus()])
})
</script>

<template>
  <div class="page" :class="{ light: !isDark, classic: isClassic }">
    <header class="site-header">
      <div class="site-header__inner">
        <div class="brand">
          <NuxtLink to="/" class="back-link">首頁</NuxtLink>
          <span class="brand-sep">/</span>
          <NuxtLink to="/stocks" class="back-link">股票列表</NuxtLink>
          <span class="brand-sep">/</span>
          <span class="brand-cur">監管狀態</span>
        </div>
        <button class="btn-icon" :aria-label="isDark ? '切換亮色模式' : '切換暗色模式'" @click="toggleTheme">
          <span v-if="isDark">☀</span><span v-else>☾</span>
        </button>
      </div>
    </header>

    <main class="content">
      <section class="topbar">
        <div>
          <p class="eyebrow">{{ responseAsOf || asOf }}</p>
          <h1 class="page-title">處置 / 注意 / 限當沖</h1>
        </div>
        <div class="sync-box">
          <span class="sync-badge" :class="`sync-badge--${syncStatus.status}`">{{ syncBadgeText }}</span>
          <button class="sync-btn" :disabled="syncLoading || syncStatus.running" @click="triggerSync">
            {{ syncLoading || syncStatus.running ? '同步中…' : '同步官方資料' }}
          </button>
        </div>
      </section>

      <p v-if="syncError" class="sync-error">{{ syncError }}</p>

      <section class="summary-strip">
        <div class="summary-item">
          <span class="summary-key">全部</span>
          <strong>{{ rows.length }}</strong>
        </div>
        <div class="summary-item">
          <span class="summary-key">處置股</span>
          <strong>{{ counts.disposition }}</strong>
        </div>
        <div class="summary-item">
          <span class="summary-key">注意股</span>
          <strong>{{ counts.attention }}</strong>
        </div>
        <div class="summary-item">
          <span class="summary-key">限當沖股</span>
          <strong>{{ counts.day_trade_restricted }}</strong>
        </div>
      </section>

      <section class="filters">
        <div class="segmented">
          <button
            v-for="tab in typeTabs"
            :key="tab.value || 'all'"
            class="seg-btn"
            :class="{ active: selectedType === tab.value }"
            @click="selectedType = tab.value"
          >{{ tab.label }}</button>
        </div>
        <div class="segmented market-tabs">
          <button
            v-for="tab in marketTabs"
            :key="tab.value || 'all'"
            class="seg-btn"
            :class="{ active: selectedMarket === tab.value }"
            @click="selectedMarket = tab.value"
          >{{ tab.label }}</button>
        </div>
        <input v-model="asOf" class="date-input" type="date" />
        <input v-model="searchQuery" class="search-input" type="text" placeholder="搜尋代號、名稱或原因…" />
      </section>

      <section class="table-wrap">
        <div v-if="loading" class="table-empty">載入中…</div>
        <div v-else-if="filteredRows.length === 0" class="table-empty">尚無符合條件的股票</div>
        <table v-else class="status-table">
          <thead>
            <tr>
              <th>代號</th>
              <th>名稱</th>
              <th>市場</th>
              <th>狀態</th>
              <th>有效期間</th>
              <th>原因 / 措施</th>
              <th>來源日</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in filteredRows" :key="`${row.id}-${row.type}`">
              <td class="td-symbol"><NuxtLink :to="`/stocks/${row.symbol}`">{{ row.symbol }}</NuxtLink></td>
              <td class="td-name">{{ row.name }}</td>
              <td><span class="market-pill">{{ row.market }}</span></td>
              <td><span class="status-chip" :class="statusClass(row.type)">{{ statusLabel(row.type) }}</span></td>
              <td class="td-period">{{ formatDate(row.start_date) }} 至 {{ formatDate(row.end_date) }}</td>
              <td class="td-reason">{{ rowReason(row) }}</td>
              <td class="td-date">{{ formatDate(row.source_date) }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </main>
  </div>
</template>

<style scoped>
.page {
  --bg:    oklch(9.5%  0.018 256);
  --s1:    oklch(13%   0.020 257);
  --s2:    oklch(16.5% 0.022 258);
  --s3:    oklch(21%   0.024 258);
  --line:  oklch(22%   0.023 258);
  --line2: oklch(33%   0.023 258);
  --blue:  oklch(63%   0.20  264);
  --gold:  oklch(76%   0.13  82);
  --t1:    oklch(96%   0.006 218);
  --t2:    oklch(72%   0.013 240);
  --t3:    oklch(50%   0.012 240);
  --up:    oklch(62%   0.18  22);
  --font:  'DM Sans', system-ui, 'PingFang TC', 'Microsoft JhengHei', sans-serif;
  min-height: 100vh;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
}
.page.light {
  --bg:    oklch(96.5% 0.009 220);
  --s1:    oklch(100%  0     0);
  --s2:    oklch(97%   0.010 220);
  --s3:    oklch(92%   0.014 220);
  --line:  oklch(88%   0.012 220);
  --line2: oklch(72%   0.015 240);
  --blue:  oklch(47%   0.21  264);
  --gold:  oklch(52%   0.16  72);
  --t1:    oklch(10%   0.018 256);
  --t2:    oklch(35%   0.016 240);
  --t3:    oklch(57%   0.012 240);
  --up:    oklch(44%   0.22  22);
}
.page *, .page *::before, .page *::after { box-sizing: border-box; }
.site-header {
  background: color-mix(in oklch, var(--s1) 88%, transparent);
  backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 20;
}
.site-header__inner {
  max-width: 1240px;
  margin: 0 auto;
  padding: 0 32px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.brand { display: flex; align-items: center; gap: 10px; }
.back-link {
  color: var(--t3);
  text-decoration: none;
  font-size: 13px;
  font-weight: 600;
}
.back-link:hover { color: var(--gold); }
.brand-sep { color: var(--line2); }
.brand-cur { color: var(--t1); font-size: 14px; font-weight: 800; }
.btn-icon {
  width: 34px;
  height: 34px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--s2);
  color: var(--t2);
  cursor: pointer;
}
.content {
  max-width: 1240px;
  margin: 0 auto;
  padding: 30px 40px 64px;
}
.topbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 18px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--line);
}
.eyebrow {
  margin: 0 0 6px;
  color: var(--t3);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  font-variant-numeric: tabular-nums;
}
.page-title {
  margin: 0;
  font-size: clamp(26px, 3vw, 38px);
  line-height: 1.1;
}
.sync-box { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; justify-content: flex-end; }
.sync-badge {
  color: var(--t3);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  font-weight: 800;
}
.sync-badge--completed { color: var(--blue); border-color: color-mix(in oklch, var(--blue) 55%, var(--line)); }
.sync-badge--failed { color: var(--up); border-color: color-mix(in oklch, var(--up) 55%, var(--line)); }
.sync-btn {
  height: 34px;
  padding: 0 14px;
  border: 1px solid var(--line2);
  border-radius: 7px;
  background: var(--s2);
  color: var(--t1);
  font-family: var(--font);
  font-weight: 800;
  cursor: pointer;
}
.sync-btn:disabled { opacity: 0.6; cursor: not-allowed; }
.sync-error { color: var(--up); font-size: 13px; margin: 12px 0 0; }
.summary-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border-bottom: 1px solid var(--line);
}
.summary-item {
  padding: 16px 22px 16px 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.summary-key {
  color: var(--t3);
  font-size: 10.5px;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}
.summary-item strong { font-size: 26px; line-height: 1; font-variant-numeric: tabular-nums; }
.filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 18px 0;
  border-bottom: 1px solid var(--line);
}
.segmented {
  display: inline-flex;
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
  background: var(--s1);
}
.seg-btn {
  height: 34px;
  padding: 0 12px;
  border: none;
  border-right: 1px solid var(--line);
  background: transparent;
  color: var(--t2);
  font-family: var(--font);
  font-size: 12px;
  font-weight: 800;
  cursor: pointer;
  white-space: nowrap;
}
.seg-btn:last-child { border-right: none; }
.seg-btn.active { color: var(--gold); background: var(--s2); }
.date-input,
.search-input {
  height: 36px;
  border: 1px solid var(--line2);
  border-radius: 8px;
  background: var(--s1);
  color: var(--t1);
  font-family: var(--font);
  padding: 0 12px;
  outline: none;
}
.search-input { min-width: min(320px, 100%); flex: 1; }
.table-wrap { overflow-x: auto; }
.status-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
.status-table th {
  text-align: left;
  color: var(--t3);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  padding: 13px 16px 11px 0;
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.status-table td {
  padding: 12px 16px 12px 0;
  border-bottom: 1px solid var(--line);
  vertical-align: top;
}
.td-symbol a {
  color: var(--gold);
  text-decoration: none;
  font-weight: 900;
  font-variant-numeric: tabular-nums;
}
.td-name { color: var(--t2); white-space: nowrap; }
.td-period,
.td-date { color: var(--t3); white-space: nowrap; font-variant-numeric: tabular-nums; }
.td-reason { color: var(--t2); min-width: 280px; line-height: 1.55; }
.market-pill,
.status-chip {
  display: inline-flex;
  align-items: center;
  height: 24px;
  padding: 0 9px;
  border-radius: 5px;
  border: 1px solid var(--line2);
  font-size: 11.5px;
  font-weight: 900;
  white-space: nowrap;
}
.market-pill { color: var(--t2); background: var(--s1); }
.status-chip--disposition {
  color: var(--up);
  border-color: color-mix(in oklch, var(--up) 62%, var(--line));
  background: color-mix(in oklch, var(--up) 10%, transparent);
}
.status-chip--attention {
  color: var(--gold);
  border-color: color-mix(in oklch, var(--gold) 62%, var(--line));
  background: color-mix(in oklch, var(--gold) 10%, transparent);
}
.status-chip--daytrade {
  color: var(--blue);
  border-color: color-mix(in oklch, var(--blue) 62%, var(--line));
  background: color-mix(in oklch, var(--blue) 10%, transparent);
}
.table-empty {
  color: var(--t3);
  padding: 56px 0;
  text-align: center;
}
@media (max-width: 760px) {
  .site-header__inner { padding: 0 16px; }
  .content { padding: 22px 16px 44px; }
  .topbar { align-items: flex-start; flex-direction: column; }
  .sync-box { justify-content: flex-start; }
  .summary-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .filters { align-items: stretch; flex-direction: column; }
  .segmented { width: 100%; overflow-x: auto; }
  .seg-btn { flex: 1; }
  .date-input,
  .search-input { width: 100%; }
}
</style>