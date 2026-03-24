<script setup lang="ts">
useHead({
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&display=swap',
    },
  ],
})

interface Stock {
  id: number
  symbol: string
  name: string
  price: number
  change: number
  change_pct: number
  volume: number
  updated_at: string
}

interface SyncState {
  stage: string
  message: string
  progress: number
  url: string
  total: number
  synced: number
  error: string
}

const { data: stocks, status, refresh } = await useFetch<Stock[]>('/api/stocks')

// ── 籌碼金字塔 ────────────────────────────
interface ChipsStatus {
  status: 'never' | 'running' | 'completed' | 'failed'
  is_fresh: boolean
  next_run: string
  started_at?: string
  completed_at?: string
  total?: number
  success?: number
  fail?: number
  message?: string
}

const { data: chipsStatus, refresh: refreshChips } = await useFetch<ChipsStatus>('/api/chips/status')

const chipsTriggering = ref(false)
const chipsError = ref('')
let chipsPollTimer: ReturnType<typeof setInterval> | null = null

function startChipsPolling() {
  if (chipsPollTimer) return
  chipsPollTimer = setInterval(() => {
    refreshChips()
  }, 3000)
}

function stopChipsPolling() {
  if (!chipsPollTimer) return
  clearInterval(chipsPollTimer)
  chipsPollTimer = null
}

async function triggerChips() {
  if (chipsTriggering.value) return
  chipsTriggering.value = true
  chipsError.value = ''
  try {
    await $fetch('/api/chips/trigger', { method: 'POST' })
    await refreshChips()
    startChipsPolling()
  } catch (err: unknown) {
    const e = err as { response?: { status?: number }; message?: string }
    if (e?.response?.status === 409) {
      chipsError.value = '已有爬取任務執行中'
      startChipsPolling()
    } else {
      chipsError.value = 'scraper 服務未啟動，請執行 docker compose up chips_scraper'
    }
  } finally {
    chipsTriggering.value = false
  }
}

function formatChipsDate(value?: string, options?: Intl.DateTimeFormatOptions) {
  if (!value) return '未知'
  const d = new Date(value)
  if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000) return '未知'
  return d.toLocaleDateString('zh-TW', options ?? {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

const chipsLastSync = computed(() => {
  if (!chipsStatus.value || chipsStatus.value.status === 'never') return '從未爬取'
  return formatChipsDate(chipsStatus.value.completed_at || chipsStatus.value.started_at)
})

const chipsNextRun = computed(() => {
  if (!chipsStatus.value?.next_run) return '—'
  const d = new Date(chipsStatus.value.next_run)
  return d.toLocaleDateString('zh-TW', { month: 'long', day: 'numeric', weekday: 'short' })
})

const chipsProcessed = computed(() =>
  (chipsStatus.value?.success ?? 0) + (chipsStatus.value?.fail ?? 0)
)

const chipsProgressLabel = computed(() => {
  const total = chipsStatus.value?.total ?? 0
  if (!total) return '—'
  return `${chipsProcessed.value} / ${total}`
})

const chipsProgressPct = computed(() => {
  const total = chipsStatus.value?.total ?? 0
  if (!total) return 0
  return Math.min(100, Math.round((chipsProcessed.value / total) * 100))
})

const chipsBadgeClass = computed(() => {
  if (chipsStatus.value?.is_fresh) return 'chips-fresh-badge--ok'
  if (chipsStatus.value?.status === 'completed') return 'chips-fresh-badge--done'
  if (chipsStatus.value?.status === 'failed') return 'chips-fresh-badge--fail'
  return 'chips-fresh-badge--stale'
})

const chipsBadgeText = computed(() => {
  if (chipsStatus.value?.is_fresh) return '本週資料已是最新'
  if (chipsStatus.value?.status === 'running') return '爬取中…'
  if (chipsStatus.value?.status === 'failed') return '本次爬取失敗'
  if (chipsStatus.value?.status === 'completed') return '已完成，但資料未達本週最新'
  if (chipsStatus.value?.status === 'never') return '尚未爬取'
  return '資料已過期'
})

const chipsSummaryTitle = computed(() => {
  if (chipsStatus.value?.status === 'running') return '正在回填籌碼資料'
  if (chipsStatus.value?.status === 'completed') return '最近一次籌碼任務已完成'
  if (chipsStatus.value?.status === 'failed') return '最近一次籌碼任務未完成'
  if (chipsStatus.value?.status === 'never') return '尚未建立籌碼任務'
  return '籌碼資料待更新'
})

const chipsSummaryText = computed(() => {
  const total = chipsStatus.value?.total ?? 0
  const success = chipsStatus.value?.success ?? 0
  const fail = chipsStatus.value?.fail ?? 0

  if (chipsStatus.value?.status === 'running') {
    return chipsStatus.value?.message || `已處理 ${chipsProcessed.value} / ${total}`
  }
  if (chipsStatus.value?.status === 'completed') {
    return `本次共處理 ${total} 檔，成功 ${success} 檔，失敗 ${fail} 檔。`
  }
  if (chipsStatus.value?.status === 'failed') {
    return chipsStatus.value?.message || '爬取程序中斷，請查看錯誤摘要後重新觸發。'
  }
  if (chipsStatus.value?.status === 'never') {
    return '可手動觸發一次全量爬取，之後每週六也會自動執行。'
  }
  return '等待下一次手動或排程更新。'
})

const chipsFailureDetail = computed(() => {
  if (chipsStatus.value?.status !== 'failed') return ''
  const msg = chipsStatus.value?.message?.trim() || ''
  if (!msg) return '未提供細節，建議查看 chips_scraper 容器日誌。'
  if (msg.includes('scraper restarted')) return '爬蟲服務在任務完成前重啟，這次作業已中止。'
  if (msg.includes('job failed:')) return msg.replace('job failed:', '').trim()
  return msg
})

const chipsCompletedAt = computed(() => {
  if (!chipsStatus.value?.completed_at) return '—'
  return formatChipsDate(chipsStatus.value.completed_at, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
})

const chipsSummaryClass = computed(() => {
  if (chipsStatus.value?.status === 'completed') return 'chips-summary--done'
  if (chipsStatus.value?.status === 'failed') return 'chips-summary--fail'
  return ''
})

const chipsCanRetry = computed(() =>
  chipsStatus.value?.status === 'failed' || chipsStatus.value?.status === 'completed'
)

const chipsShouldShowSecondaryActions = computed(() =>
  chipsStatus.value?.status === 'failed' || chipsStatus.value?.status === 'completed' || chipsStatus.value?.status === 'never'
)

watch(
  () => chipsStatus.value?.status,
  (status) => {
    if (status === 'running') startChipsPolling()
    else stopChipsPolling()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stopChipsPolling()
})

const syncing = ref(false)
const syncState = ref<SyncState | null>(null)
const syncLabel = ref('')

function startSSE(url: string, label: string) {
  if (syncing.value) return
  syncing.value = true
  syncState.value = null
  syncLabel.value = label

  const es = new EventSource(url)

  es.onmessage = async (e) => {
    const data: SyncState = JSON.parse(e.data)
    syncState.value = data

    if (data.stage === 'done') {
      es.close()
      syncing.value = false
      await refresh()
    } else if (data.stage === 'error') {
      es.close()
      syncing.value = false
    }
  }

  es.onerror = () => {
    es.close()
    syncing.value = false
    if (!syncState.value || syncState.value.stage !== 'done') {
      syncState.value = {
        stage: 'error', message: '', progress: 0, url: '', total: 0, synced: 0,
        error: '連線失敗，請確認後端服務是否正常',
      }
    }
  }
}

function syncStocks() { startSSE('/api/scraper/stocks', 'stocks') }
function syncPrices() { startSSE('/api/scraper/prices', 'prices') }

const stockList = computed<Stock[]>(() =>
  Array.isArray(stocks.value) ? stocks.value : []
)

// K 線圖快速跳轉
const jumpSymbol = ref('')
const router = useRouter()

const jumpSuggestions = computed(() => {
  const q = jumpSymbol.value.trim().toLowerCase()
  if (!q) return []
  return stockList.value
    .filter(s => s.symbol.toLowerCase().startsWith(q) || s.name.toLowerCase().includes(q))
    .slice(0, 5)
})

function jumpToChart() {
  const q = jumpSymbol.value.trim().toUpperCase()
  if (!q) return
  // 優先精確比對；無結果時直接導航讓 stock 頁處理
  const exact = stockList.value.find(s => s.symbol === q)
  if (exact || jumpSuggestions.value.length === 0) {
    router.push(`/stocks/${q}`)
  } else {
    router.push(`/stocks/${jumpSuggestions.value[0]!.symbol}`)
  }
}

// 統計
const totalStocks = computed(() => stockList.value.length)
const lastSyncDisplay = computed(() => {
  if (!stockList.value.length) return '尚未同步'
  const latest = stockList.value
    .map(s => new Date(s.updated_at))
    .filter(d => !isNaN(d.getTime()))
    .reduce((a, b) => (a > b ? a : b), new Date(0))
  if (latest.getFullYear() < 2000) return '尚未同步'
  return latest.toLocaleDateString('zh-TW', { year: 'numeric', month: 'long', day: 'numeric' })
})

const today = new Date().toLocaleDateString('zh-TW', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  weekday: 'long',
})

// 主題切換
const isDark = ref(
  typeof localStorage !== 'undefined'
    ? localStorage.getItem('tsm-theme') === 'dark'
    : false
)
function toggleTheme() {
  isDark.value = !isDark.value
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('tsm-theme', isDark.value ? 'dark' : 'light')
  }
}
</script>

<template>
  <div class="page" :class="{ light: !isDark }">

    <!-- ══ Site Header ══ -->
    <header class="site-header">
      <div class="site-header__inner">
        <div class="brand">
          <span class="brand-badge">TSM</span>
          <div class="brand-text">
            <span class="brand-sub">Taiwan Stock Monitor</span>
            <span class="brand-name">台股監控系統</span>
          </div>
        </div>
        <div class="header-right">
          <span class="header-api" :class="status === 'error' ? 'header-api--err' : 'header-api--ok'">
            <span class="api-pip" />
            {{ status === 'error' ? 'API 離線' : status === 'pending' ? '連線中' : 'API 正常' }}
          </span>
          <span class="header-date">{{ today }}</span>
          <button class="theme-toggle" :aria-label="isDark ? '切換亮色模式' : '切換暗色模式'" @click="toggleTheme">
            <span v-if="isDark">☀</span>
            <span v-else>☾</span>
          </button>
        </div>
      </div>
    </header>

    <!-- ══ Sync progress bar ══ -->
    <Transition name="slide-down">
      <div v-if="syncState" class="sync-bar" :class="`sync-bar--${syncState.stage}`">
        <div class="sync-bar__inner">
          <span class="sync-bar__icon">
            <span v-if="syncState.stage === 'error'">✕</span>
            <span v-else-if="syncState.stage === 'done'">✓</span>
            <span v-else class="syncing-spin">◌</span>
          </span>
          <span class="sync-bar__msg">{{ syncState.stage === 'error' ? syncState.error : syncState.message }}</span>
          <div v-if="syncState.stage !== 'error'" class="sync-bar__track">
            <div class="sync-bar__fill" :style="{ width: syncState.progress + '%' }" />
          </div>
          <span v-if="syncState.stage !== 'error'" class="sync-bar__pct">{{ syncState.progress }}%</span>
          <a v-if="syncState.url" :href="syncState.url" target="_blank" rel="noopener" class="sync-bar__url">{{ syncState.url }}</a>
        </div>
      </div>
    </Transition>

    <!-- ══ Portal ══ -->
    <section class="portal">
      <!-- ══ Card Grid ══ -->
      <div class="cards-grid">

        <!-- Card 1: Overview (2 cols) -->
        <article class="card card--overview">
          <p class="card-eyebrow">Database Overview</p>
          <div class="overview-body">
            <div class="overview-left">
              <span class="big-num">{{ totalStocks > 0 ? totalStocks.toLocaleString() : '—' }}</span>
              <span class="big-label">上市上櫃股票</span>
            </div>
            <div class="overview-right">
              <div class="meta-row">
                <span class="meta-key">最後同步</span>
                <span class="meta-val">{{ lastSyncDisplay }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-key">資料來源</span>
                <span class="meta-val">TWSE · TPEX</span>
              </div>
            </div>
          </div>
          <NuxtLink to="/stocks" class="ghost-btn">
            瀏覽完整列表 <span class="ghost-btn__arr">→</span>
          </NuxtLink>
        </article>

        <!-- Card 2: Status (2 cols) -->
        <article class="card card--status">
          <p class="card-eyebrow">System Status</p>
          <ul class="status-list">
            <li class="status-item">
              <span class="pip" :class="status !== 'error' ? 'pip--ok' : 'pip--err'" />
              <span class="status-name">API 連線</span>
              <span class="status-val">{{ status === 'error' ? '失敗' : status === 'pending' ? '連線中…' : '正常運作' }}</span>
            </li>
            <li class="status-item">
              <span class="pip" :class="totalStocks > 0 ? 'pip--ok' : 'pip--warn'" />
              <span class="status-name">資料庫</span>
              <span class="status-val">{{ totalStocks > 0 ? `${totalStocks.toLocaleString()} 筆` : '空白' }}</span>
            </li>
            <li class="status-item">
              <span class="pip" :class="syncing ? 'pip--busy' : 'pip--idle'" />
              <span class="status-name">同步作業</span>
              <span class="status-val">{{ syncing ? '進行中…' : '閒置' }}</span>
            </li>
          </ul>
        </article>

        <!-- Card 3: Sync Stocks (1 col) -->
        <article class="card card--action">
          <p class="card-eyebrow">Data Sync</p>
          <h2 class="card-title">同步股票清單</h2>
          <p class="card-desc">從 TWSE 及 TPEX 抓取最新上市、上櫃股票名冊，更新本地資料庫。</p>
          <button
            class="action-btn"
            :class="{ 'action-btn--busy': syncing && syncLabel === 'stocks' }"
            :disabled="syncing"
            @click="syncStocks"
          >{{ syncing && syncLabel === 'stocks' ? '同步中…' : '立即同步' }}</button>
        </article>

        <!-- Card 4: Sync Prices (1 col) -->
        <article class="card card--action">
          <p class="card-eyebrow">Data Sync</p>
          <h2 class="card-title">同步日 K 資料</h2>
          <p class="card-desc">批次更新全部股票之歷史日 K 價量資料，作為技術分析基礎。</p>
          <button
            class="action-btn"
            :class="{ 'action-btn--busy': syncing && syncLabel === 'prices' }"
            :disabled="syncing"
            @click="syncPrices"
          >{{ syncing && syncLabel === 'prices' ? '同步中…' : '立即同步' }}</button>
        </article>

        <!-- Card 5: K-Chart Lookup (2 cols) -->
        <article class="card card--lookup">
          <p class="card-eyebrow">Chart Analysis</p>
          <h2 class="card-title">K 線圖查詢</h2>
          <p class="card-desc">輸入股票代號查看個股蠟燭圖與成交量走勢。</p>
          <div class="lookup-wrap">
            <div class="lookup-field" :class="{ 'lookup-field--active': jumpSymbol }">
              <input
                v-model="jumpSymbol"
                class="lookup-input"
                type="text"
                placeholder="輸入代號，如 2330"
                @keyup.enter="jumpToChart"
              />
              <button class="lookup-go" @click="jumpToChart">前往</button>
            </div>
            <ul v-if="jumpSuggestions.length > 0 && jumpSymbol" class="suggestions">
              <li
                v-for="s in jumpSuggestions"
                :key="s.symbol"
                class="suggestion"
                @click="router.push(`/stocks/${s.symbol}`)"
              >
                <span class="sug-sym">{{ s.symbol }}</span>
                <span class="sug-name">{{ s.name }}</span>
              </li>
            </ul>
          </div>
        </article>

        <!-- Card 6: Chips Pyramid (2 cols) -->
        <article class="card card--chips">
          <p class="card-eyebrow">Chips Pyramid · 籌碼金字塔</p>
          <div class="chips-body">
            <div class="chips-fresh-badge" :class="chipsBadgeClass">
              <span class="pip pip--lg" :class="chipsStatus?.is_fresh ? 'pip--ok' : (chipsStatus?.status === 'running' ? 'pip--busy' : chipsStatus?.status === 'failed' ? 'pip--fail' : 'pip--warn')" />
              <span>{{ chipsBadgeText }}</span>
            </div>
            <div class="chips-summary" :class="chipsSummaryClass">
              <p class="chips-summary__title">{{ chipsSummaryTitle }}</p>
              <p class="chips-summary__text">{{ chipsSummaryText }}</p>
            </div>
            <div class="chips-meta">
              <div class="meta-row">
                <span class="meta-key">上次爬取</span>
                <span class="meta-val">{{ chipsLastSync }}</span>
              </div>
              <div v-if="chipsStatus && chipsStatus.status !== 'never'" class="meta-row">
                <span class="meta-key">成功 / 總計</span>
                <span class="meta-val">{{ chipsStatus.success ?? 0 }} / {{ chipsStatus.total ?? 0 }}</span>
              </div>
              <div v-if="chipsStatus && chipsStatus.status !== 'never'" class="meta-row">
                <span class="meta-key">目前進度</span>
                <span class="meta-val">{{ chipsProgressLabel }}（{{ chipsProgressPct }}%）</span>
              </div>
              <div class="meta-row">
                <span class="meta-key">下次排程</span>
                <span class="meta-val">{{ chipsNextRun }}（週六自動）</span>
              </div>
              <div v-if="chipsStatus && chipsStatus.status !== 'running' && chipsStatus.status !== 'never'" class="meta-row">
                <span class="meta-key">完成時間</span>
                <span class="meta-val">{{ chipsCompletedAt }}</span>
              </div>
            </div>
            <div v-if="chipsStatus?.status === 'running'" class="chips-progress">
              <div class="chips-progress__track">
                <div class="chips-progress__fill" :style="{ width: `${chipsProgressPct}%` }" />
              </div>
              <p class="chips-progress__text">{{ chipsStatus?.message || '爬取進行中…' }}</p>
            </div>
            <div v-else-if="chipsStatus?.status === 'completed'" class="chips-result chips-result--done">
              <span class="chips-result__label">完成摘要</span>
              <span class="chips-result__value">成功 {{ chipsStatus?.success ?? 0 }} 檔，失敗 {{ chipsStatus?.fail ?? 0 }} 檔</span>
            </div>
            <div v-else-if="chipsStatus?.status === 'failed'" class="chips-result chips-result--fail">
              <span class="chips-result__label">失敗原因</span>
              <span class="chips-result__value">{{ chipsFailureDetail }}</span>
            </div>
            <div v-if="chipsShouldShowSecondaryActions" class="chips-actions">
              <button
                v-if="chipsCanRetry"
                class="chips-secondary chips-secondary--retry"
                :disabled="chipsTriggering || chipsStatus?.status === 'running'"
                @click="triggerChips"
              >
                再試一次
              </button>
              <NuxtLink to="/stocks" class="chips-secondary chips-secondary--link">
                查看股票列表
              </NuxtLink>
            </div>
          </div>
          <button
            class="action-btn"
            :class="{ 'action-btn--busy': chipsTriggering || chipsStatus?.status === 'running' }"
            :disabled="chipsTriggering || chipsStatus?.status === 'running'"
            @click="triggerChips"
          >
            {{ chipsTriggering || chipsStatus?.status === 'running' ? '爬取中…' : '手動觸發爬取' }}
          </button>
          <p v-if="chipsError" class="chips-err">⚠ {{ chipsError }}</p>
        </article>

      </div>
    </section>

  </div>
</template>

<style scoped>
/* ── Design Tokens — defined on .page so scoped CSS can resolve them ── */
.page {
  --bg:    oklch(14.5% 0.016 258);
  --s1:    oklch(19%   0.018 258);
  --s2:    oklch(23%   0.018 258);
  --line:  oklch(28%   0.020 258);
  --line2: oklch(36%   0.020 258);
  --t1:    oklch(97%   0.006 82);
  --t2:    oklch(78%   0.012 258);
  --t3:    oklch(58%   0.014 258);
  --gold:  oklch(76%   0.095 80);
  --up:    oklch(59%   0.18  22);
  --dn:    oklch(62%   0.17  148);
  --warn:  oklch(72%   0.13  72);
  --font:  'DM Sans', system-ui, 'PingFang TC', 'Microsoft JhengHei', sans-serif;

  min-height: 100vh;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
  font-size: 16px;
  line-height: 1.55;
  -webkit-font-smoothing: antialiased;
  box-sizing: border-box;
}

.page *, .page *::before, .page *::after { box-sizing: border-box; margin: 0; padding: 0; }

/* ── Light Mode Overrides ──────────────── */
.page.light {
  --bg:    oklch(96.5% 0.007 82);
  --s1:    oklch(93%   0.008 82);
  --s2:    oklch(99%   0.004 82);
  --line:  oklch(84%   0.012 258);
  --line2: oklch(68%   0.015 258);
  --t1:    oklch(13%   0.020 258);
  --t2:    oklch(34%   0.016 258);
  --t3:    oklch(54%   0.014 258);
  --gold:  oklch(48%   0.13  60);
  --up:    oklch(44%   0.21  22);
  --dn:    oklch(38%   0.19  148);
  --warn:  oklch(52%   0.14  72);
}

/* ── Site Header ───────────────────────── */
.site-header {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}

.site-header__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 40px;
  height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
}

.brand-badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  color: var(--bg);
  background: var(--gold);
  padding: 5px 8px;
  line-height: 1;
  flex-shrink: 0;
}

.brand-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.brand-sub {
  font-size: 10px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--t3);
  line-height: 1;
}

.brand-name {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--t1);
  line-height: 1;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 24px;
}

.header-api {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  color: var(--t2);
}

.api-pip {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--t3);
  flex-shrink: 0;
}

.header-api--ok  .api-pip { background: var(--dn); }
.header-api--err .api-pip { background: var(--up); }

.header-date {
  font-size: 12.5px;
  color: var(--t3);
  font-variant-numeric: tabular-nums;
}

.theme-toggle {
  background: none;
  border: 1px solid var(--line2);
  color: var(--t2);
  font-size: 15px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
  line-height: 1;
  padding: 0;
  flex-shrink: 0;
}
.theme-toggle:hover { border-color: var(--gold); color: var(--gold); }

/* ── Sync Bar ──────────────────────────── */
.sync-bar {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  border-left: 2px solid var(--gold);
}
.sync-bar--error { border-left-color: var(--up); }
.sync-bar--done  { border-left-color: var(--dn); }

.sync-bar__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 9px 40px;
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12px;
  color: var(--t2);
}

.sync-bar__icon { font-size: 11px; font-weight: 700; flex-shrink: 0; }
.sync-bar--done  .sync-bar__icon { color: var(--dn); }
.sync-bar--error .sync-bar__icon { color: var(--up); }

.sync-bar__msg { flex-shrink: 0; }

.sync-bar__track {
  flex: 1;
  max-width: 180px;
  height: 2px;
  background: var(--line);
}

.sync-bar__fill {
  height: 100%;
  background: var(--gold);
  transition: width 0.35s cubic-bezier(0.25, 1, 0.5, 1);
}
.sync-bar--done .sync-bar__fill { background: var(--dn); }

.sync-bar__pct {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  color: var(--t3);
  min-width: 32px;
}

.sync-bar__url {
  font-size: 10.5px;
  color: var(--t3);
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 240px;
}
.sync-bar__url:hover { color: var(--gold); }

@keyframes spin {
  to { transform: rotate(360deg); }
}
.syncing-spin { display: inline-block; animation: spin 1.4s linear infinite; }
.spin-icon    { display: inline-block; animation: spin 1.4s linear infinite; color: var(--gold); }

.slide-down-enter-active,
.slide-down-leave-active {
  transition: opacity 0.18s cubic-bezier(0.25, 1, 0.5, 1),
              transform 0.18s cubic-bezier(0.25, 1, 0.5, 1);
}
.slide-down-enter-from,
.slide-down-leave-to { opacity: 0; transform: translateY(-5px); }

/* ── Portal ────────────────────────────── */
.portal {
  max-width: 1200px;
  margin: 0 auto;
  padding: 28px 40px 0;
}

/* ── Card Grid ─────────────────────────── */
.cards-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1px;
  background: var(--line);
  border: 1px solid var(--line);
}

.card {
  background: var(--s2);
  padding: 24px 28px;
  display: flex;
  flex-direction: column;
}

.card--overview { grid-column: span 2; min-height: 220px; }
.card--status   { grid-column: span 2; }
.card--action   { grid-column: span 1; min-height: 200px; }
.card--lookup   { grid-column: span 2; position: relative; }
.card--chips    { grid-column: span 2; }

/* ── Chips card ─────────────────────────── */
.chips-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-bottom: 16px;
}

.chips-fresh-badge {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--line);
  font-size: 13.5px;
  font-weight: 600;
}

.chips-fresh-badge--ok    { border-color: var(--dn);   color: var(--dn); }
.chips-fresh-badge--done  { border-color: color-mix(in oklch, var(--dn) 70%, var(--gold)); color: var(--dn); }
.chips-fresh-badge--fail  { border-color: var(--up);   color: var(--up); }
.chips-fresh-badge--stale { border-color: var(--line2); color: var(--t2); }

.pip--lg { width: 8px; height: 8px; }
.pip--busy { background: var(--warn); animation: pulse 1.4s ease-in-out infinite; }
.pip--fail { background: var(--up); }
@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.3; } }

.chips-summary {
  display: grid;
  gap: 5px;
  padding: 12px 14px;
  background: color-mix(in oklch, var(--s1) 76%, transparent);
  border-left: 2px solid var(--gold);
}

.chips-summary--done {
  background: color-mix(in oklch, var(--dn) 10%, var(--s1));
  border-left-color: var(--dn);
}

.chips-summary--fail {
  background: color-mix(in oklch, var(--up) 10%, var(--s1));
  border-left-color: var(--up);
}

.chips-summary__title {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--t1);
}

.chips-summary__text {
  font-size: 12.5px;
  color: var(--t2);
  line-height: 1.55;
}

.chips-err {
  font-size: 12px;
  color: var(--up);
  margin-top: 8px;
  line-height: 1.5;
}

.chips-progress {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chips-progress__track {
  width: 100%;
  height: 6px;
  background: var(--line);
  overflow: hidden;
}

.chips-progress__fill {
  height: 100%;
  background: linear-gradient(90deg, var(--gold), var(--dn));
  transition: width 0.35s cubic-bezier(0.25, 1, 0.5, 1);
}

.chips-progress__text {
  font-size: 12px;
  color: var(--t2);
  line-height: 1.5;
}

.chips-result {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--line);
}

.chips-result--done {
  border-color: color-mix(in oklch, var(--dn) 55%, var(--line));
  background: color-mix(in oklch, var(--dn) 9%, var(--s1));
}

.chips-result--fail {
  border-color: color-mix(in oklch, var(--up) 55%, var(--line));
  background: color-mix(in oklch, var(--up) 9%, var(--s1));
}

.chips-result__label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--t3);
}

.chips-result__value {
  font-size: 12.5px;
  color: var(--t1);
  line-height: 1.55;
}

.chips-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.chips-secondary {
  min-height: 34px;
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-decoration: none;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}

.chips-secondary--retry {
  border: 1px solid color-mix(in oklch, var(--dn) 60%, var(--line));
  background: color-mix(in oklch, var(--dn) 9%, var(--s1));
  color: var(--dn);
}

.chips-secondary--retry:hover:not(:disabled) {
  border-color: var(--dn);
  background: color-mix(in oklch, var(--dn) 14%, var(--s1));
}

.chips-secondary--retry:disabled {
  opacity: 0.45;
  cursor: default;
}

.chips-secondary--link {
  border: 1px solid var(--line2);
  color: var(--t2);
  background: transparent;
}

.chips-secondary--link:hover {
  border-color: var(--gold);
  color: var(--gold);
}

.card-eyebrow {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 14px;
}

.card-title {
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--t1);
  margin-bottom: 10px;
}

.card-desc {
  font-size: 14.5px;
  color: var(--t2);
  line-height: 1.7;
  flex: 1;
  margin-bottom: 20px;
}

/* Overview card */
.overview-body {
  display: flex;
  align-items: flex-end;
  gap: 40px;
  flex: 1;
  padding-bottom: 24px;
}

.overview-left { display: flex; flex-direction: column; gap: 7px; }

.big-num {
  font-size: clamp(52px, 5.5vw, 72px);
  font-weight: 700;
  letter-spacing: -0.05em;
  line-height: 0.88;
  font-variant-numeric: tabular-nums;
  color: var(--t1);
}

.big-label {
  font-size: 12px;
  letter-spacing: 0.10em;
  text-transform: uppercase;
  color: var(--t3);
}

.overview-right {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-bottom: 4px;
}

.meta-row { display: flex; flex-direction: column; gap: 3px; }

.meta-key {
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
}

.meta-val {
  font-size: 15px;
  color: var(--t2);
  font-variant-numeric: tabular-nums;
}

/* Ghost button */
.ghost-btn {
  align-self: flex-start;
  font-family: var(--font);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: var(--gold);
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: color 0.15s;
  text-decoration: none;
}
.ghost-btn:hover { color: var(--t1); }
.ghost-btn:hover .ghost-btn__arr { transform: translateX(3px); }

.ghost-btn__arr {
  font-size: 13px;
  transition: transform 0.2s cubic-bezier(0.25, 1, 0.5, 1);
}

/* Status card */
.status-list {
  list-style: none;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 13px 0;
  border-bottom: 1px solid var(--line);
  font-size: 15px;
}
.status-item:last-child { border-bottom: none; }

.pip {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.pip--ok   { background: var(--dn); }
.pip--err  { background: var(--up); }
.pip--warn { background: var(--warn); }
.pip--busy { background: var(--gold); }
.pip--idle { background: var(--line2); }

.status-name { flex: 1; color: var(--t2); font-size: 14.5px; }

.status-val {
  font-size: 13px;
  color: var(--t3);
  font-variant-numeric: tabular-nums;
}

/* Action button */
.action-btn {
  align-self: flex-start;
  font-family: var(--font);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.04em;
  padding: 10px 22px;
  background: transparent;
  color: var(--t1);
  border: 1px solid var(--line2);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
}
.action-btn:hover:not(:disabled):not(.action-btn--busy) {
  background: var(--gold);
  border-color: var(--gold);
  color: var(--bg);
}
.action-btn:disabled,
.action-btn--busy { opacity: 0.32; cursor: not-allowed; }

/* Lookup card */
.lookup-wrap { position: relative; }

.lookup-field {
  display: flex;
  border: 1px solid var(--line2);
  transition: border-color 0.15s;
}
.lookup-field:focus-within,
.lookup-field--active { border-color: var(--t1); }

.lookup-input {
  flex: 1;
  padding: 11px 14px;
  background: transparent;
  border: none;
  outline: none;
  font-family: var(--font);
  font-size: 15px;
  color: var(--t1);
  font-variant-numeric: tabular-nums;
}
.lookup-input::placeholder { color: var(--t3); }

.lookup-go {
  padding: 11px 20px;
  background: var(--gold);
  color: var(--bg);
  border: none;
  cursor: pointer;
  font-family: var(--font);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
  transition: background 0.15s;
}
.lookup-go:hover { background: var(--t1); }

.suggestions {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--s1);
  border: 1px solid var(--line2);
  border-top: none;
  list-style: none;
  z-index: 20;
}

.suggestion {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 11px 14px;
  cursor: pointer;
  font-size: 14.5px;
  border-bottom: 1px solid var(--line);
  transition: background 0.1s;
}
.suggestion:last-child { border-bottom: none; }
.suggestion:hover { background: var(--line); }

.sug-sym {
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  min-width: 50px;
  color: var(--gold);
}
.sug-name { color: var(--t2); }

/* ── RWD ───────────────────────────────── */
@media (max-width: 960px) {
  .site-header__inner,
  .sync-bar__inner { padding-left: 16px; padding-right: 16px; }

  .portal      { padding: 16px 16px 0; }

  .header-date { display: none; }

  .cards-grid { grid-template-columns: repeat(2, 1fr); }
  .card--overview { grid-column: span 2; }
  .card--status   { grid-column: span 2; }
  .card--action   { grid-column: span 1; }
  .card--lookup   { grid-column: span 2; }
  .card--chips    { grid-column: span 2; }

  .card { padding: 18px 18px; }

  .big-num { font-size: 48px; }
  .overview-body { flex-direction: column; align-items: flex-start; gap: 16px; padding-bottom: 16px; }

  .sync-bar__track,
  .sync-bar__url { display: none; }
}

@media (max-width: 520px) {
  .cards-grid { grid-template-columns: 1fr; }
  .card--overview,
  .card--status,
  .card--lookup,
  .card--chips { grid-column: span 1; }

  .card { padding: 16px; }
  .card--action { min-height: unset; }

  .big-num { font-size: 40px; }
}
</style>
