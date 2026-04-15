<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useAppPrefs } from '~/composables/useAppPrefs'

const { isDark, isClassic, toggleTheme, setTheme, setStyle } = useAppPrefs()

// ─── Types ─────────────────────────────────────────────────────
interface Tag {
  id: number
  name: string
  color: string
}

interface StockGroup {
  id: number
  name: string
  description: string
  color: string
}

interface Stock {
  id: number
  symbol: string
  name: string
  price: number
  change: number
  change_pct: number
  volume: number
  industry: string
  tags?: Tag[]
  groups?: StockGroup[]
}

type PoolType = 'group' | 'tag'

interface PinnedPool {
  type: PoolType
  id: number
}

interface DisplayPool {
  type: PoolType
  id: number
  name: string
  color: string
  description: string
}

interface RealtimeQuote {
  symbol: string
  price: number
  open: number
  high: number
  low: number
  change: number
  change_pct: number
  volume: number
  timestamp: string
}

// ─── API ────────────────────────────────────────────────────────
const { data: groups, refresh: refreshGroups } = await useFetch<StockGroup[]>('/api/groups')
const { data: tags, refresh: refreshTags } = await useFetch<Tag[]>('/api/tags')

// ─── State ──────────────────────────────────────────────────────
const STORAGE_KEY = 'watchlist_pools_v1'
const pinnedPools = ref<PinnedPool[]>([])
const configOpen = ref(false)
const activePool = ref<DisplayPool | null>(null)
const poolStocks = ref<Stock[]>([])
const poolLoading = ref(false)
const settingsOpen = ref(false)

// ─── Trading Hours & Realtime ───────────────────────────────────
function isTradingHours(): boolean {
  const now = new Date()
  const day = now.getDay()
  if (day === 0 || day === 6) return false
  const t = now.getHours() * 60 + now.getMinutes()
  return t >= 9 * 60 && t <= 13 * 60 + 30
}

const realtimeMap = ref<Record<string, RealtimeQuote>>({})
const isLive = ref(false)
const lastUpdateTime = ref('')
let realtimeTimer: ReturnType<typeof setInterval> | null = null

async function fetchRealtimeForPool() {
  if (poolStocks.value.length === 0) return
  const results = await Promise.allSettled(
    poolStocks.value.map(s => $fetch<RealtimeQuote>(`/api/realtime/${s.symbol}`))
  )
  const newMap: Record<string, RealtimeQuote> = { ...realtimeMap.value }
  results.forEach((r, i) => {
    if (r.status === 'fulfilled' && r.value?.price > 0) {
      newMap[poolStocks.value[i].symbol] = r.value
    }
  })
  realtimeMap.value = newMap
  const now = new Date()
  lastUpdateTime.value = now.toLocaleTimeString('zh-TW', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function startRealtimePolling() {
  stopRealtimePolling()
  if (!isTradingHours()) return
  isLive.value = true
  fetchRealtimeForPool()
  realtimeTimer = setInterval(() => {
    if (isTradingHours()) {
      fetchRealtimeForPool()
    } else {
      stopRealtimePolling()
    }
  }, 10_000)
}

function stopRealtimePolling() {
  if (realtimeTimer) {
    clearInterval(realtimeTimer)
    realtimeTimer = null
  }
  isLive.value = false
  realtimeMap.value = {}
  lastUpdateTime.value = ''
}

onBeforeUnmount(() => stopRealtimePolling())

onMounted(() => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) pinnedPools.value = JSON.parse(raw)
  } catch { /* ignore */ }
})

function savePinned() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(pinnedPools.value))
}

function isPinned(type: PoolType, id: number) {
  return pinnedPools.value.some(p => p.type === type && p.id === id)
}

function togglePin(type: PoolType, id: number) {
  const idx = pinnedPools.value.findIndex(p => p.type === type && p.id === id)
  if (idx >= 0) {
    pinnedPools.value.splice(idx, 1)
    if (activePool.value?.type === type && activePool.value?.id === id) {
      activePool.value = null
      poolStocks.value = []
    }
  } else {
    pinnedPools.value.push({ type, id })
  }
  savePinned()
}

// ─── Computed ───────────────────────────────────────────────────
const displayPools = computed<DisplayPool[]>(() => {
  return pinnedPools.value
    .map(p => {
      if (p.type === 'group') {
        const g = groups.value?.find(g => g.id === p.id)
        return g ? { type: 'group' as PoolType, id: p.id, name: g.name, color: g.color, description: g.description } : null
      } else {
        const t = tags.value?.find(t => t.id === p.id)
        return t ? { type: 'tag' as PoolType, id: p.id, name: t.name, color: t.color, description: '' } : null
      }
    })
    .filter((p): p is DisplayPool => p !== null)
})

const allGroups = computed(() => groups.value ?? [])
const allTags = computed(() => tags.value ?? [])

// ─── Computed: merge static + realtime ─────────────────────────
const displayStocks = computed(() =>
  poolStocks.value.map(s => {
    const rt = realtimeMap.value[s.symbol]
    if (!rt || rt.price <= 0) return s
    return { ...s, price: rt.price, change: rt.change, change_pct: rt.change_pct, volume: rt.volume }
  })
)

// ─── Pool Actions ───────────────────────────────────────────────
async function openPool(pool: DisplayPool) {
  if (activePool.value?.type === pool.type && activePool.value?.id === pool.id) {
    activePool.value = null
    poolStocks.value = []
    batchSelectMode.value = false
    selectedSymbols.value = []
    return
  }
  activePool.value = pool
  poolLoading.value = true
  poolStocks.value = []
  batchSelectMode.value = false
  selectedSymbols.value = []
  stopRealtimePolling()
  try {
    const param = pool.type === 'group' ? `group_id=${pool.id}` : `tag_id=${pool.id}`
    const result = await $fetch<Stock[]>(`/api/stocks?${param}`)
    poolStocks.value = Array.isArray(result) ? result : []
    startRealtimePolling()
  } catch {
    poolStocks.value = []
  } finally {
    poolLoading.value = false
  }
}

function closePool() {
  stopRealtimePolling()
  activePool.value = null
  poolStocks.value = []
  batchSelectMode.value = false
  selectedSymbols.value = []
}

// ─── Add Stock Modal ─────────────────────────────────────────────
const addModalOpen = ref(false)
const addSearchQuery = ref('')
const addSearchResults = ref<(Stock & { _inPool?: boolean })[]>([])
const addSelectedSymbols = ref<string[]>([])
const addSearching = ref(false)
const addProcessing = ref(false)
let addSearchTimer: ReturnType<typeof setTimeout> | null = null

function openAddModal() {
  addModalOpen.value = true
  addSearchQuery.value = ''
  addSearchResults.value = []
  addSelectedSymbols.value = []
}

function closeAddModal() {
  addModalOpen.value = false
  if (addSearchTimer) clearTimeout(addSearchTimer)
}

watch(addSearchQuery, (q) => {
  if (addSearchTimer) clearTimeout(addSearchTimer)
  if (!q.trim()) {
    addSearchResults.value = []
    return
  }
  addSearchTimer = setTimeout(async () => {
    addSearching.value = true
    try {
      const res = await $fetch<Stock[]>(`/api/stocks?q=${encodeURIComponent(q.trim())}`)
      const currentSymbols = new Set(poolStocks.value.map(s => s.symbol))
      addSearchResults.value = (Array.isArray(res) ? res : []).slice(0, 30)
        .map(s => ({ ...s, _inPool: currentSymbols.has(s.symbol) } as Stock & { _inPool: boolean }))
    } catch {
      addSearchResults.value = []
    } finally {
      addSearching.value = false
    }
  }, 300)
})

function toggleAddSelect(symbol: string) {
  const idx = addSelectedSymbols.value.indexOf(symbol)
  if (idx >= 0) addSelectedSymbols.value.splice(idx, 1)
  else addSelectedSymbols.value.push(symbol)
}

async function confirmAddStocks() {
  if (!activePool.value || addSelectedSymbols.value.length === 0) return
  addProcessing.value = true
  try {
    const { type, id } = activePool.value
    const base = type === 'group' ? `/api/groups/${id}/members` : `/api/tags/${id}/members`
    await $fetch(base, { method: 'POST', body: { symbols: addSelectedSymbols.value } })
    // 重新載入
    const param = type === 'group' ? `group_id=${id}` : `tag_id=${id}`
    const result = await $fetch<Stock[]>(`/api/stocks?${param}`)
    poolStocks.value = Array.isArray(result) ? result : []
    closeAddModal()
  } catch (err) {
    alert('新增失敗：' + (err as Error).message)
  } finally {
    addProcessing.value = false
  }
}

// ─── Remove Stock ────────────────────────────────────────────────
const removingSymbol = ref('')

async function removeStock(symbol: string) {
  if (!activePool.value) return
  if (!confirm(`確定要從「${activePool.value.name}」移除 ${symbol}？`)) return
  removingSymbol.value = symbol
  try {
    const { type, id } = activePool.value
    const base = type === 'group' ? `/api/groups/${id}/members` : `/api/tags/${id}/members`
    await $fetch(base, { method: 'DELETE', body: { symbols: [symbol] } })
    poolStocks.value = poolStocks.value.filter(s => s.symbol !== symbol)
  } catch (err) {
    alert('移除失敗：' + (err as Error).message)
  } finally {
    removingSymbol.value = ''
  }
}

// ─── Batch Remove ────────────────────────────────────────────────
const batchSelectMode = ref(false)
const selectedSymbols = ref<string[]>([])
const batchRemoving = ref(false)

function toggleBatchMode() {
  batchSelectMode.value = !batchSelectMode.value
  if (!batchSelectMode.value) selectedSymbols.value = []
}

function toggleSelectSymbol(symbol: string) {
  const idx = selectedSymbols.value.indexOf(symbol)
  if (idx >= 0) selectedSymbols.value.splice(idx, 1)
  else selectedSymbols.value.push(symbol)
}

function selectAll() {
  selectedSymbols.value = poolStocks.value.map(s => s.symbol)
}

function clearSelection() {
  selectedSymbols.value = []
}

async function removeBatchSelected() {
  if (!activePool.value || selectedSymbols.value.length === 0) return
  if (!confirm(`確定要移除選取的 ${selectedSymbols.value.length} 檔股票？`)) return
  batchRemoving.value = true
  try {
    const { type, id } = activePool.value
    const base = type === 'group' ? `/api/groups/${id}/members` : `/api/tags/${id}/members`
    await $fetch(base, { method: 'DELETE', body: { symbols: selectedSymbols.value } })
    const removed = new Set(selectedSymbols.value)
    poolStocks.value = poolStocks.value.filter(s => !removed.has(s.symbol))
    selectedSymbols.value = []
    batchSelectMode.value = false
  } catch (err) {
    alert('批次移除失敗：' + (err as Error).message)
  } finally {
    batchRemoving.value = false
  }
}

// ─── Helpers ────────────────────────────────────────────────────
function colorClass(val: number) {
  if (val > 0) return 'up'
  if (val < 0) return 'dn'
  return ''
}

function fmtPct(val: number) {
  const sign = val > 0 ? '+' : ''
  return `${sign}${val.toFixed(2)}%`
}

function fmtPrice(val: number) {
  return val > 0 ? val.toFixed(2) : '—'
}

const today = new Date().toLocaleDateString('zh-TW', {
  year: 'numeric', month: 'long', day: 'numeric', weekday: 'long',
})
</script>

<template>
  <div class="wl-page" :class="{ light: !isDark }">

    <!-- ══ Header ══ -->
    <header class="wl-header">
      <div class="wl-header__inner">
        <div class="wl-brand">
          <NuxtLink to="/" class="wl-back">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
              <path d="M10 13L5 8l5-5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            首頁
          </NuxtLink>
          <span class="wl-brand-sep">/</span>
          <span class="wl-brand-cur">關注股池</span>
        </div>
        <nav class="wl-nav">
          <span class="wl-date">{{ today }}</span>
          <div class="settings-wrap">
            <button class="btn-icon" aria-label="外觀設定" @click="settingsOpen = !settingsOpen">
              <svg width="15" height="15" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="2.3" stroke="currentColor" stroke-width="1.4"/>
                <path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M3.05 12.95l1.06-1.06M11.89 4.11l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
              </svg>
            </button>
            <div v-if="settingsOpen" class="settings-overlay" @click="settingsOpen = false" />
            <div v-if="settingsOpen" class="settings-panel">
              <p class="sp-title">外觀設定</p>
              <div class="sp-group">
                <p class="sp-label">主題</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: !isDark }" @click="setTheme(false)">亮色</button>
                  <button class="sp-btn" :class="{ active: isDark }" @click="setTheme(true)">暗色</button>
                </div>
              </div>
            </div>
          </div>
          <button class="btn-icon" :aria-label="isDark ? '切換亮色' : '切換暗色'" @click="toggleTheme">
            <svg v-if="isDark" width="16" height="16" viewBox="0 0 16 16" fill="none">
              <circle cx="8" cy="8" r="2.8" fill="currentColor"/>
              <path d="M8 1.5V3M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
            <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M13.2 9.3A5.8 5.8 0 0 1 6.7 2.8a.4.4 0 0 0-.46-.5A6.3 6.3 0 1 0 13.7 9.76a.4.4 0 0 0-.5-.46Z" fill="currentColor"/>
            </svg>
          </button>
          <button class="wl-config-btn" @click="configOpen = !configOpen">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
              <path d="M1 4h14M1 8h14M1 12h14" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
            </svg>
            設定股池
          </button>
          <NuxtLink to="/manage-pools" class="wl-config-btn">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
              <circle cx="8" cy="8" r="2.3" stroke="currentColor" stroke-width="1.4"/>
              <path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M3.05 12.95l1.06-1.06M11.89 4.11l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
            批次管理
          </NuxtLink>
        </nav>
      </div>
    </header>

    <!-- ══ Config Drawer ══ -->
    <Transition name="drawer">
      <aside v-if="configOpen" class="config-drawer">
        <div class="config-drawer__head">
          <span class="config-drawer__title">設定關注股池</span>
          <button class="config-close-btn" @click="configOpen = false">✕</button>
        </div>
        <p class="config-hint">勾選要加入關注的群組或標籤，點擊卡片即可查看持股。</p>

        <!-- Groups -->
        <div class="config-section">
          <p class="config-section-title">群組</p>
          <div v-if="allGroups.length === 0" class="config-empty">尚無群組，請至股票列表新增</div>
          <label
            v-for="g in allGroups"
            :key="`g-${g.id}`"
            class="config-item"
            :class="{ checked: isPinned('group', g.id) }"
          >
            <input
              type="checkbox"
              class="config-checkbox"
              :checked="isPinned('group', g.id)"
              @change="togglePin('group', g.id)"
            />
            <span class="config-dot" :style="{ background: g.color }"></span>
            <span class="config-name">{{ g.name }}</span>
            <span v-if="g.description" class="config-desc">{{ g.description }}</span>
          </label>
        </div>

        <!-- Tags -->
        <div class="config-section">
          <p class="config-section-title">標籤</p>
          <div v-if="allTags.length === 0" class="config-empty">尚無標籤，請至股票列表新增</div>
          <label
            v-for="t in allTags"
            :key="`t-${t.id}`"
            class="config-item"
            :class="{ checked: isPinned('tag', t.id) }"
          >
            <input
              type="checkbox"
              class="config-checkbox"
              :checked="isPinned('tag', t.id)"
              @change="togglePin('tag', t.id)"
            />
            <span class="config-dot" :style="{ background: t.color }"></span>
            <span class="config-name">{{ t.name }}</span>
          </label>
        </div>
      </aside>
    </Transition>
    <div v-if="configOpen" class="config-overlay" @click="configOpen = false" />

    <!-- ══ Main Content ══ -->
    <main class="wl-main">

      <!-- Toolbar -->
      <div class="wl-toolbar">
        <div class="wl-toolbar__left">
          <h1 class="wl-title">關注股池</h1>
          <span class="wl-count">{{ displayPools.length }} 個池子</span>
        </div>
        <div class="wl-toolbar__right">
          <button class="wl-config-btn-sm" @click="configOpen = !configOpen">
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
              <path d="M1 4h14M1 8h14M1 12h14" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
            </svg>
            管理池子
          </button>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="displayPools.length === 0" class="wl-empty">
        <div class="wl-empty__icon">
          <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
            <rect x="4" y="4" width="18" height="18" rx="3" stroke="currentColor" stroke-width="2" opacity="0.4"/>
            <rect x="26" y="4" width="18" height="18" rx="3" stroke="currentColor" stroke-width="2" opacity="0.25"/>
            <rect x="4" y="26" width="18" height="18" rx="3" stroke="currentColor" stroke-width="2" opacity="0.2"/>
            <rect x="26" y="26" width="18" height="18" rx="3" stroke="currentColor" stroke-width="2" opacity="0.55"/>
          </svg>
        </div>
        <p class="wl-empty__title">尚未設定任何股池</p>
        <p class="wl-empty__desc">點擊右上角「管理池子」，選擇要追蹤的群組或標籤</p>
        <button class="wl-empty__btn" @click="configOpen = true">開始設定</button>
      </div>

      <!-- Pool Cards Grid -->
      <div v-else class="pool-grid">
        <article
          v-for="pool in displayPools"
          :key="`${pool.type}-${pool.id}`"
          class="pool-card"
          :class="{ 'pool-card--active': activePool?.type === pool.type && activePool?.id === pool.id }"
          @click="openPool(pool)"
        >
          <div class="pool-card__accent" :style="{ background: pool.color }"></div>
          <div class="pool-card__body">
            <div class="pool-card__top">
              <span class="pool-type-badge" :class="`pool-type-badge--${pool.type}`">
                {{ pool.type === 'group' ? '群組' : '標籤' }}
              </span>
              <span class="pool-card__arrow">→</span>
            </div>
            <div class="pool-card__name-row">
              <span class="pool-dot" :style="{ background: pool.color }"></span>
              <h2 class="pool-card__name">{{ pool.name }}</h2>
            </div>
            <p v-if="pool.description" class="pool-card__desc">{{ pool.description }}</p>
            <p v-else class="pool-card__desc pool-card__desc--muted">
              {{ pool.type === 'group' ? '點擊查看群組股票' : '點擊查看標籤股票' }}
            </p>
          </div>
          <button
            class="pool-unpin-btn"
            title="從股池移除"
            @click.stop="togglePin(pool.type, pool.id)"
          >×</button>
        </article>
      </div>

      <!-- Pool Detail ── Stock Cards -->
      <Transition name="fade-slide">
        <section v-if="activePool" class="pool-detail">
          <div class="pool-detail__head">
            <div class="pool-detail__info">
              <span class="pool-dot pool-dot--lg" :style="{ background: activePool.color }"></span>
              <div>
                <h2 class="pool-detail__name">{{ activePool.name }}</h2>
                <p class="pool-detail__meta">
                  {{ activePool.type === 'group' ? '群組' : '標籤' }}
                  <template v-if="!poolLoading">
                    · {{ poolStocks.length }} 檔
                  </template>
                </p>
              </div>
            </div>
            <div class="pool-detail__right">
              <div v-if="isLive" class="live-badge">
                <span class="live-dot" />
                LIVE
                <span v-if="lastUpdateTime" class="live-time">{{ lastUpdateTime }}</span>
              </div>
              <!-- Batch Remove Controls -->
              <template v-if="batchSelectMode">
                <button class="pd-action-btn pd-action-btn--danger" :disabled="selectedSymbols.length === 0 || batchRemoving" @click="removeBatchSelected">
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
                    <path d="M3 6h10M6 2h4M5 6l.5 7h5L11 6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
                  </svg>
                  {{ batchRemoving ? '移除中…' : `移除選取 (${selectedSymbols.length})` }}
                </button>
                <button class="pd-action-btn pd-action-btn--ghost" @click="selectAll">全選</button>
                <button class="pd-action-btn pd-action-btn--ghost" @click="clearSelection">清除</button>
                <button class="pd-action-btn pd-action-btn--ghost" @click="toggleBatchMode">取消</button>
              </template>
              <template v-else>
                <button class="pd-action-btn pd-action-btn--primary" @click="openAddModal">
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
                    <path d="M8 3v10M3 8h10" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                  </svg>
                  新增股票
                </button>
                <button v-if="poolStocks.length > 0" class="pd-action-btn pd-action-btn--ghost" @click="toggleBatchMode">
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
                    <rect x="2" y="2" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.5"/>
                    <rect x="9" y="2" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.5"/>
                    <rect x="2" y="9" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.5"/>
                    <path d="M12 11.5v-2M11 9.5h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                  </svg>
                  批次移除
                </button>
              </template>
              <button class="pool-detail__close" @click="closePool">✕ 收起</button>
            </div>
          </div>

          <div v-if="poolLoading" class="pool-loading">
            <span class="spin-icon">◌</span> 載入中…
          </div>

          <div v-else-if="poolStocks.length === 0" class="pool-empty">
            此池子尚未有股票，請至「<NuxtLink to="/stocks">股票列表</NuxtLink>」指定群組或標籤
          </div>

          <div v-else class="stock-grid">
            <div
              v-for="stock in displayStocks"
              :key="stock.symbol"
              class="stock-card-wrap"
              :class="{ 'batch-mode': batchSelectMode, 'selected': selectedSymbols.includes(stock.symbol) }"
              @click="batchSelectMode ? toggleSelectSymbol(stock.symbol) : undefined"
            >
              <!-- Batch select checkbox -->
              <span v-if="batchSelectMode" class="stock-card__checkbox">
                <svg v-if="selectedSymbols.includes(stock.symbol)" width="14" height="14" viewBox="0 0 16 16" fill="none">
                  <rect x="1" y="1" width="14" height="14" rx="3" fill="oklch(47% 0.21 264)" stroke="oklch(47% 0.21 264)" stroke-width="1.5"/>
                  <path d="M4.5 8l2.5 2.5 5-5" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                <svg v-else width="14" height="14" viewBox="0 0 16 16" fill="none">
                  <rect x="1" y="1" width="14" height="14" rx="3" stroke="currentColor" stroke-width="1.5"/>
                </svg>
              </span>
              <!-- Remove button (non-batch mode) -->
              <button
                v-else
                class="stock-card__remove"
                :disabled="removingSymbol === stock.symbol"
                @click.prevent.stop="removeStock(stock.symbol)"
                title="從池子移除"
              >
                <svg width="10" height="10" viewBox="0 0 16 16" fill="none">
                  <path d="M3 3l10 10M13 3L3 13" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                </svg>
              </button>
              <NuxtLink
                :to="`/stocks/${stock.symbol}`"
                class="stock-card"
                :class="colorClass(stock.change)"
              >
                <div class="stock-card__head">
                  <span class="stock-card__symbol">{{ stock.symbol }}</span>
                  <span
                    class="stock-card__pct"
                    :class="colorClass(stock.change_pct)"
                  >{{ stock.price > 0 ? fmtPct(stock.change_pct) : '—' }}</span>
                </div>
                <p class="stock-card__name">{{ stock.name }}</p>
                <div class="stock-card__price-row">
                  <span class="stock-card__price">{{ fmtPrice(stock.price) }}</span>
                  <span
                    v-if="stock.price > 0"
                    class="stock-card__change"
                    :class="colorClass(stock.change)"
                  >
                    {{ stock.change > 0 ? '+' : '' }}{{ stock.change.toFixed(2) }}
                  </span>
                </div>
                <p v-if="stock.volume > 0" class="stock-card__vol">
                  量 {{ stock.volume.toLocaleString() }}
                </p>
              </NuxtLink>
            </div>

          </div>
        </section>
      </Transition>

    </main>

  <!-- ══ Add Stock Modal ══ -->
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="addModalOpen" class="add-modal-overlay" @click.self="closeAddModal">
        <div class="add-modal">
          <div class="add-modal__head">
            <h3 class="add-modal__title">新增股票到「{{ activePool?.name }}」</h3>
            <button class="add-modal__close" @click="closeAddModal">
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                <path d="M3 3l10 10M13 3L3 13" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
          <div class="add-modal__search">
            <svg class="add-modal__search-icon" width="15" height="15" viewBox="0 0 16 16" fill="none">
              <circle cx="6.5" cy="6.5" r="4" stroke="currentColor" stroke-width="1.5"/>
              <path d="M11 11l3 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            </svg>
            <input
              v-model="addSearchQuery"
              class="add-modal__input"
              placeholder="輸入股票代號或名稱搜尋…"
              autofocus
            />
            <span v-if="addSearching" class="add-modal__spin">◌</span>
          </div>
          <div class="add-modal__results">
            <div v-if="!addSearchQuery.trim()" class="add-modal__hint">請輸入關鍵字搜尋股票</div>
            <div v-else-if="addSearchResults.length === 0 && !addSearching" class="add-modal__hint">無結果</div>
            <label
              v-for="stock in addSearchResults"
              :key="stock.symbol"
              class="add-result-item"
              :class="{ selected: addSelectedSymbols.includes(stock.symbol), 'in-pool': stock._inPool }"
            >
              <input
                type="checkbox"
                class="sr-only"
                :checked="addSelectedSymbols.includes(stock.symbol)"
                :disabled="stock._inPool"
                @change="toggleAddSelect(stock.symbol)"
              />
              <span class="add-result-item__check">
                <svg v-if="addSelectedSymbols.includes(stock.symbol)" width="12" height="12" viewBox="0 0 16 16" fill="none">
                  <path d="M3.5 8l3 3 6-6" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </span>
              <span class="add-result-item__symbol">{{ stock.symbol }}</span>
              <span class="add-result-item__name">{{ stock.name }}</span>
              <span v-if="stock._inPool" class="add-result-item__tag">已在池中</span>
              <span class="add-result-item__industry">{{ stock.industry }}</span>
            </label>
          </div>
          <div class="add-modal__foot">
            <span class="add-modal__count">已選 {{ addSelectedSymbols.length }} 檔</span>
            <button class="add-modal__cancel" @click="closeAddModal">取消</button>
            <button
              class="add-modal__confirm"
              :disabled="addSelectedSymbols.length === 0 || addProcessing"
              @click="confirmAddStocks"
            >
              {{ addProcessing ? '加入中…' : `加入 (${addSelectedSymbols.length})` }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</div>
</template>

<style scoped>
/* ── Variables ────────────────────────────────────────────────── */
.wl-page {
  --bg:       #0d0f14;
  --surface:  #13161e;
  --surface2: #1a1e28;
  --border:   rgba(255,255,255,0.07);
  --text-1:   #e8eaf0;
  --text-2:   #9aa0b4;
  --text-3:   #5a607a;
  --blue:     #3b7ef8;
  --blue-dim: rgba(59,126,248,0.15);
  --green:    #22c55e;
  --red:      #ef4444;
  --gold:     #e6b455;
  --radius:   10px;
  --radius-lg:16px;

  min-height: 100vh;
  background: var(--bg);
  color: var(--text-1);
  font-family: 'DM Sans', system-ui, -apple-system, sans-serif;
}

.wl-page.light {
  --bg:       #f5f6fa;
  --surface:  #ffffff;
  --surface2: #f0f1f5;
  --border:   rgba(0,0,0,0.08);
  --text-1:   #111827;
  --text-2:   #4b5563;
  --text-3:   #9ca3af;
  --blue-dim: rgba(59,126,248,0.08);
}

/* ── Header ──────────────────────────────────────────────────── */
.wl-header {
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  position: sticky;
  top: 0;
  z-index: 100;
}

.wl-header__inner {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 24px;
  height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.wl-brand {
  display: flex;
  align-items: center;
  gap: 8px;
}

.wl-back {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-2);
  text-decoration: none;
  font-size: 0.82rem;
  transition: color 0.15s;
}
.wl-back:hover { color: var(--text-1); }

.wl-brand-sep { color: var(--text-3); font-size: 0.82rem; }
.wl-brand-cur { font-size: 0.9rem; font-weight: 600; color: var(--text-1); }

.wl-nav {
  display: flex;
  align-items: center;
  gap: 12px;
}

.wl-date {
  font-size: 0.78rem;
  color: var(--text-3);
}

.btn-icon {
  width: 32px;
  height: 32px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface2);
  color: var(--text-2);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
}
.btn-icon:hover { border-color: var(--blue); color: var(--blue); }

.wl-config-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border: 1px solid var(--blue);
  border-radius: 8px;
  background: var(--blue-dim);
  color: var(--blue);
  font-size: 0.82rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}
.wl-config-btn:hover { background: var(--blue); color: #fff; }

/* Settings panel (reuse from other pages) */
.settings-wrap { position: relative; }
.settings-overlay { position: fixed; inset: 0; z-index: 200; }
.settings-panel {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 210;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 16px;
  min-width: 180px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.3);
}
.sp-title { font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.08em; color: var(--text-3); margin-bottom: 12px; }
.sp-group { margin-bottom: 12px; }
.sp-label { font-size: 0.78rem; color: var(--text-2); margin-bottom: 6px; }
.sp-btns { display: flex; gap: 6px; }
.sp-btn {
  flex: 1;
  padding: 5px 8px;
  font-size: 0.78rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface2);
  color: var(--text-2);
  cursor: pointer;
  transition: all 0.15s;
}
.sp-btn.active { background: var(--blue); border-color: var(--blue); color: #fff; }

/* ── Config Drawer ─────────────────────────────────────────────  */
.config-overlay {
  position: fixed;
  inset: 0;
  z-index: 299;
  background: rgba(0,0,0,0.4);
}

.config-drawer {
  position: fixed;
  top: 0;
  right: 0;
  height: 100vh;
  width: 320px;
  z-index: 300;
  background: var(--surface);
  border-left: 1px solid var(--border);
  padding: 24px 20px;
  overflow-y: auto;
  box-shadow: -8px 0 32px rgba(0,0,0,0.25);
}

.config-drawer__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.config-drawer__title { font-size: 0.95rem; font-weight: 700; color: var(--text-1); }
.config-close-btn {
  width: 28px;
  height: 28px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface2);
  color: var(--text-2);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8rem;
}
.config-close-btn:hover { color: var(--red); border-color: var(--red); }

.config-hint {
  font-size: 0.78rem;
  color: var(--text-3);
  margin-bottom: 20px;
  line-height: 1.5;
}

.config-section { margin-bottom: 24px; }
.config-section-title {
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-3);
  margin-bottom: 10px;
}

.config-empty {
  font-size: 0.78rem;
  color: var(--text-3);
  padding: 8px 0;
}

.config-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  margin-bottom: 6px;
  cursor: pointer;
  transition: all 0.15s;
  background: var(--surface2);
}
.config-item:hover { border-color: var(--blue); background: var(--blue-dim); }
.config-item.checked { border-color: var(--blue); background: var(--blue-dim); }

.config-checkbox { display: none; }

.config-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.config-name { font-size: 0.85rem; color: var(--text-1); flex: 1; font-weight: 500; }
.config-desc { font-size: 0.72rem; color: var(--text-3); }

/* ── Main ─────────────────────────────────────────────────────── */
.wl-main {
  max-width: 1400px;
  margin: 0 auto;
  padding: 28px 24px 60px;
}

.wl-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}
.wl-toolbar__left { display: flex; align-items: center; gap: 12px; }
.wl-title { font-size: 1.4rem; font-weight: 700; color: var(--text-1); }
.wl-count { font-size: 0.8rem; color: var(--text-3); padding: 3px 10px; background: var(--surface2); border-radius: 20px; }

.wl-config-btn-sm {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text-2);
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.15s;
}
.wl-config-btn-sm:hover { border-color: var(--blue); color: var(--blue); }

/* ── Empty State ─────────────────────────────────────────────── */
.wl-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 360px;
  gap: 12px;
  text-align: center;
}
.wl-empty__icon { color: var(--text-3); margin-bottom: 8px; }
.wl-empty__title { font-size: 1.1rem; font-weight: 600; color: var(--text-2); }
.wl-empty__desc { font-size: 0.85rem; color: var(--text-3); max-width: 300px; line-height: 1.5; }
.wl-empty__btn {
  margin-top: 8px;
  padding: 9px 22px;
  border: 1px solid var(--blue);
  border-radius: 8px;
  background: var(--blue-dim);
  color: var(--blue);
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}
.wl-empty__btn:hover { background: var(--blue); color: #fff; }

/* ── Pool Cards Grid ─────────────────────────────────────────── */
.pool-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
  margin-bottom: 32px;
}

.pool-card {
  position: relative;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  cursor: pointer;
  transition: all 0.2s;
}
.pool-card:hover { border-color: rgba(255,255,255,0.18); transform: translateY(-2px); box-shadow: 0 8px 24px rgba(0,0,0,0.25); }
.pool-card--active { border-color: var(--blue); box-shadow: 0 0 0 2px rgba(59,126,248,0.2); }

.pool-card__accent {
  height: 4px;
  width: 100%;
}

.pool-card__body {
  padding: 16px;
}

.pool-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.pool-type-badge {
  font-size: 0.68rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 2px 8px;
  border-radius: 20px;
}
.pool-type-badge--group { background: rgba(230,180,85,0.15); color: var(--gold); }
.pool-type-badge--tag   { background: rgba(59,126,248,0.15); color: var(--blue); }

.pool-card__arrow {
  color: var(--text-3);
  font-size: 0.85rem;
  transition: transform 0.15s, color 0.15s;
}
.pool-card:hover .pool-card__arrow,
.pool-card--active .pool-card__arrow { transform: translateX(3px); color: var(--text-1); }

.pool-card__name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.pool-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.pool-dot--lg { width: 14px; height: 14px; }

.pool-card__name { font-size: 1rem; font-weight: 700; color: var(--text-1); }
.pool-card__desc { font-size: 0.75rem; color: var(--text-2); line-height: 1.4; }
.pool-card__desc--muted { color: var(--text-3); }

.pool-unpin-btn {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 22px;
  height: 22px;
  border: 1px solid transparent;
  border-radius: 50%;
  background: transparent;
  color: var(--text-3);
  cursor: pointer;
  font-size: 0.8rem;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: all 0.15s;
}
.pool-card:hover .pool-unpin-btn { opacity: 1; }
.pool-unpin-btn:hover { background: rgba(239,68,68,0.15); border-color: var(--red); color: var(--red); }

/* ── Pool Detail ─────────────────────────────────────────────── */
.pool-detail {
  border-top: 1px solid var(--border);
  padding-top: 28px;
  margin-top: 8px;
}

.pool-detail__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.pool-detail__info { display: flex; align-items: center; gap: 12px; }
.pool-detail__right { display: flex; align-items: center; gap: 12px; }
.pool-detail__name { font-size: 1.15rem; font-weight: 700; color: var(--text-1); }
.pool-detail__meta { font-size: 0.78rem; color: var(--text-3); margin-top: 2px; }

.pool-detail__close {
  padding: 6px 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text-2);
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.15s;
}
.pool-detail__close:hover { border-color: var(--text-2); color: var(--text-1); }

.pool-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 40px;
  color: var(--text-3);
  justify-content: center;
  font-size: 0.9rem;
}

.pool-empty {
  padding: 40px;
  text-align: center;
  color: var(--text-3);
  font-size: 0.85rem;
  line-height: 1.6;
}
.pool-empty a { color: var(--blue); text-decoration: none; }
.pool-empty a:hover { text-decoration: underline; }

/* ── Stock Cards Grid ─────────────────────────────────────────── */
.stock-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}

.stock-card {
  display: block;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 14px 16px;
  text-decoration: none;
  transition: all 0.15s;
  position: relative;
  overflow: hidden;
}
.stock-card::before {
  content: '';
  position: absolute;
  inset: 0;
  opacity: 0;
  transition: opacity 0.15s;
}
.stock-card.up::before { background: rgba(34,197,94,0.04); }
.stock-card.dn::before { background: rgba(239,68,68,0.04); }
.stock-card:hover { border-color: rgba(255,255,255,0.18); transform: translateY(-1px); box-shadow: 0 4px 16px rgba(0,0,0,0.2); }
.stock-card:hover::before { opacity: 1; }

.stock-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.stock-card__symbol {
  font-size: 1rem;
  font-weight: 700;
  color: var(--text-1);
  letter-spacing: 0.02em;
}
.stock-card__pct {
  font-size: 0.78rem;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 5px;
}
.stock-card__pct.up { background: rgba(34,197,94,0.15); color: var(--green); }
.stock-card__pct.dn { background: rgba(239,68,68,0.15); color: var(--red); }

.stock-card__name {
  font-size: 0.78rem;
  color: var(--text-2);
  margin-bottom: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.stock-card__price-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.stock-card__price {
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-1);
  letter-spacing: 0.01em;
}
.stock-card__change {
  font-size: 0.8rem;
  font-weight: 500;
}
.stock-card__change.up { color: var(--green); }
.stock-card__change.dn { color: var(--red); }

.stock-card__vol {
  font-size: 0.7rem;
  color: var(--text-3);
  margin-top: 4px;
}

/* ── Live Badge ──────────────────────────────────────────────── */
.live-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 20px;
  background: rgba(34, 197, 94, 0.12);
  border: 1px solid rgba(34, 197, 94, 0.3);
  color: var(--green);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.live-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--green);
  animation: live-pulse 1.4s ease-in-out infinite;
}

.live-time {
  font-weight: 400;
  opacity: 0.75;
  letter-spacing: 0;
}

@keyframes live-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%       { opacity: 0.4; transform: scale(0.7); }
}

/* ── Spin ─────────────────────────────────────────────────────── */
.spin-icon {
  display: inline-block;
  animation: spin 1s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* ── Transitions ─────────────────────────────────────────────── */
.drawer-enter-active,
.drawer-leave-active { transition: transform 0.25s ease, opacity 0.25s ease; }
.drawer-enter-from,
.drawer-leave-to { transform: translateX(100%); opacity: 0; }

.fade-slide-enter-active,
.fade-slide-leave-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.fade-slide-enter-from,
.fade-slide-leave-to { opacity: 0; transform: translateY(10px); }

/* ── Pool Detail Action Buttons ───────────────────────────────── */
.pd-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border-radius: 7px;
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s;
}
.pd-action-btn--primary {
  background: oklch(47% 0.21 264);
  color: #fff;
  border-color: oklch(47% 0.21 264);
}
.pd-action-btn--primary:hover { background: oklch(52% 0.21 264); }
.pd-action-btn--danger {
  background: rgba(239,68,68,0.15);
  color: #ef4444;
  border-color: rgba(239,68,68,0.35);
}
.pd-action-btn--danger:hover:not(:disabled) { background: rgba(239,68,68,0.25); }
.pd-action-btn--ghost {
  background: var(--surface);
  color: var(--text-2);
  border-color: var(--border);
}
.pd-action-btn--ghost:hover { border-color: var(--text-3); color: var(--text-1); }
.pd-action-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* ── Stock Card Wrapper (for remove btn / batch select) ─────── */
.stock-card-wrap {
  position: relative;
}
.stock-card-wrap .stock-card {
  width: 100%;
}
.stock-card__remove {
  position: absolute;
  top: 6px;
  right: 6px;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-3);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s, background 0.15s, color 0.15s;
}
.stock-card-wrap:hover .stock-card__remove {
  opacity: 1;
}
.stock-card__remove:hover {
  background: rgba(239,68,68,0.15);
  color: #ef4444;
  border-color: rgba(239,68,68,0.3);
}
.stock-card__remove:disabled { opacity: 0.3; cursor: not-allowed; }

.stock-card__checkbox {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 2;
  color: var(--text-2);
  cursor: pointer;
  display: flex;
}

.stock-card-wrap.batch-mode { cursor: pointer; }
.stock-card-wrap.batch-mode .stock-card { pointer-events: none; }
.stock-card-wrap.selected .stock-card {
  border-color: oklch(47% 0.21 264);
  background: oklch(47% 0.21 264 / 0.08);
}

/* ── Add Stock Modal ─────────────────────────────────────────── */
.add-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}
.add-modal {
  background: #1a1e28;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 14px;
  width: 100%;
  max-width: 520px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.add-modal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 14px;
  border-bottom: 1px solid rgba(255,255,255,0.07);
}
.add-modal__title {
  font-size: 0.95rem;
  font-weight: 700;
  color: #e8eaf0;
}
.add-modal__close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  border: 1px solid rgba(255,255,255,0.1);
  background: transparent;
  color: #9aa0b4;
  cursor: pointer;
  transition: all 0.15s;
}
.add-modal__close:hover { background: rgba(255,255,255,0.06); color: #e8eaf0; }

.add-modal__search {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 20px;
  border-bottom: 1px solid rgba(255,255,255,0.07);
}
.add-modal__search-icon { color: #5a607a; flex-shrink: 0; }
.add-modal__input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: #e8eaf0;
  font-size: 0.9rem;
}
.add-modal__input::placeholder { color: #5a607a; }
.add-modal__spin {
  color: #5a607a;
  font-size: 0.85rem;
  animation: spin 1s linear infinite;
}

.add-modal__results {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}
.add-modal__hint {
  padding: 30px 20px;
  text-align: center;
  color: #5a607a;
  font-size: 0.85rem;
}
.add-result-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 20px;
  cursor: pointer;
  transition: background 0.12s;
}
.add-result-item:hover:not(.in-pool) { background: rgba(255,255,255,0.04); }
.add-result-item.selected { background: oklch(47% 0.21 264 / 0.1); }
.add-result-item.in-pool { opacity: 0.45; cursor: not-allowed; }
.add-result-item__check {
  width: 18px;
  height: 18px;
  border-radius: 4px;
  border: 1.5px solid rgba(255,255,255,0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.12s;
}
.add-result-item.selected .add-result-item__check {
  background: oklch(47% 0.21 264);
  border-color: oklch(47% 0.21 264);
}
.add-result-item__symbol {
  font-size: 0.88rem;
  font-weight: 700;
  color: #e8eaf0;
  width: 52px;
  flex-shrink: 0;
}
.add-result-item__name {
  font-size: 0.82rem;
  color: #9aa0b4;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.add-result-item__tag {
  font-size: 0.7rem;
  padding: 2px 7px;
  border-radius: 4px;
  background: rgba(255,255,255,0.06);
  color: #5a607a;
  flex-shrink: 0;
}
.add-result-item__industry {
  font-size: 0.72rem;
  color: #5a607a;
  flex-shrink: 0;
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.add-modal__foot {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 20px;
  border-top: 1px solid rgba(255,255,255,0.07);
}
.add-modal__count {
  font-size: 0.78rem;
  color: #9aa0b4;
  flex: 1;
}
.add-modal__cancel {
  padding: 7px 16px;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.12);
  background: transparent;
  color: #9aa0b4;
  font-size: 0.82rem;
  cursor: pointer;
  transition: all 0.15s;
}
.add-modal__cancel:hover { border-color: #9aa0b4; color: #e8eaf0; }
.add-modal__confirm {
  padding: 7px 18px;
  border-radius: 8px;
  border: none;
  background: oklch(47% 0.21 264);
  color: #fff;
  font-size: 0.82rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s;
}
.add-modal__confirm:hover:not(:disabled) { background: oklch(52% 0.21 264); }
.add-modal__confirm:disabled { opacity: 0.45; cursor: not-allowed; }

/* Modal transition */
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .add-modal, .modal-leave-active .add-modal { transition: transform 0.2s ease; }
.modal-enter-from .add-modal { transform: scale(0.96) translateY(10px); }
.modal-leave-to .add-modal { transform: scale(0.96) translateY(10px); }

/* Screen reader only */
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0,0,0,0);
  white-space: nowrap;
  border: 0;
}
</style>
