<script setup lang="ts">
useHead({
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

const route = useRoute()
const symbol = route.params.symbol as string

// ── 型別 ──────────────────────────────────
interface Stock {
  id: number
  symbol: string
  name: string
  updated_at: string
}

interface DailyPrice {
  id: number
  symbol: string
  date: string
  open: number
  high: number
  low: number
  close: number
  volume: number
  tx_value: number
  tx_count: number
}

// ── 資料抓取 ──────────────────────────────
const { data: stock } = await useFetch<Stock>(`/api/stocks/${symbol}`)

const today = new Date()
const from = ref(new Date(today.getFullYear() - 1, today.getMonth(), today.getDate()).toISOString().split('T')[0])
const to = ref(today.toISOString().split('T')[0])

const { data: prices, refresh: refreshPrices } = await useFetch<DailyPrice[]>(
  () => `/api/stocks/${symbol}/prices?from=${from.value}&to=${to.value}&limit=2000`,
)

// ── 主題 + 風格 ─────────────────────────────────────────────────────────
const { isDark, appStyle, isBento, isClassic, toggleTheme, setTheme, setStyle } = useAppPrefs()
const settingsOpen = ref(false)

// ── Canvas K-line Chart ───────────────────
const canvasRef = ref<HTMLCanvasElement | null>(null)
const canvasWrap = ref<HTMLElement | null>(null)
const hoveredIdx = ref<number | null>(null)
let roChart: ResizeObserver | null = null

const CHART_H   = 460
const PRICE_RATIO = 0.73
const PAD_R   = 68   // right: price labels
const PAD_T   = 14
const PAD_B   = 28   // bottom: date labels
const GAP_V   = 8    // gap between price & volume areas

function getColors() {
  return isDark.value
    ? { bg: '#1c1e2f', grid: 'rgba(255,255,255,0.045)', axis: 'rgba(255,255,255,0.18)',
        label: 'rgba(230,224,210,0.52)', up: '#e05252', dn: '#3daa6b',
        xhair: 'rgba(255,255,255,0.22)', tooltip: 'rgba(24,27,44,0.92)' }
    : { bg: '#fafaf8', grid: 'rgba(0,0,0,0.05)',       axis: 'rgba(0,0,0,0.18)',
        label: 'rgba(18,20,38,0.48)',   up: '#c93535', dn: '#1f8a50',
        xhair: 'rgba(0,0,0,0.20)',      tooltip: 'rgba(250,250,248,0.95)' }
}

function drawChart() {
  const canvas = canvasRef.value
  const wrap   = canvasWrap.value
  if (!canvas || !wrap) return

  const W = wrap.clientWidth
  const H = CHART_H
  const dpr = window.devicePixelRatio || 1

  canvas.width  = W * dpr
  canvas.height = H * dpr
  canvas.style.width  = W + 'px'
  canvas.style.height = H + 'px'

  const ctx = canvas.getContext('2d')!
  ctx.scale(dpr, dpr)

  const priceH = Math.round((H - PAD_T - PAD_B - GAP_V) * PRICE_RATIO)
  const volH   = H - PAD_T - PAD_B - GAP_V - priceH
  const volTop = PAD_T + priceH + GAP_V
  const drawW  = W - PAD_R

  const c = getColors()

  // Clear
  ctx.fillStyle = c.bg
  ctx.fillRect(0, 0, W, H)

  const data = prices.value
  if (!data || data.length === 0) {
    ctx.fillStyle = c.label
    ctx.font = '13px "DM Sans", system-ui, sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('尚無資料', W / 2, H / 2)
    return
  }

  const n = data.length
  const barW  = drawW / n
  const candW = Math.max(1, Math.min(barW * 0.65, 14))

  // Price range
  const pMin = Math.min(...data.map(d => d.low))  * 0.9975
  const pMax = Math.max(...data.map(d => d.high)) * 1.0025
  const vMax = Math.max(...data.map(d => d.volume)) * 1.05 || 1

  const xOf     = (i: number) => i * barW + barW / 2
  const yPrice  = (p: number) => PAD_T + priceH * (1 - (p - pMin) / (pMax - pMin))
  const yVol    = (v: number) => volTop + volH * (1 - v / vMax)

  // ── Grid lines (price area)
  ctx.strokeStyle = c.grid
  ctx.lineWidth = 1
  const gridN = 5
  for (let i = 0; i <= gridN; i++) {
    const y = PAD_T + (priceH / gridN) * i
    ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(drawW, y); ctx.stroke()
  }

  // ── Price axis labels
  ctx.fillStyle = c.label
  ctx.textAlign = 'right'
  ctx.font = '10.5px "DM Sans", system-ui, sans-serif'
  for (let i = 0; i <= gridN; i++) {
    const y = PAD_T + (priceH / gridN) * i
    const p = pMax - (pMax - pMin) * (i / gridN)
    ctx.fillText(p.toFixed(2), W - 4, y + 4)
  }

  // ── Date axis labels (x)
  const maxLbl = Math.floor(drawW / 72)
  const step   = Math.max(1, Math.ceil(n / maxLbl))
  ctx.textAlign = 'center'
  ctx.fillStyle = c.label
  ctx.font = '10.5px "DM Sans", system-ui, sans-serif'
  for (let i = 0; i < n; i += step) {
    const x = xOf(i)
    const date = data[i]!.date.split('T')[0]!
    ctx.fillText(date.substring(5), x, H - 6)  // MM-DD
  }

  // ── Separating line between price & volume
  ctx.strokeStyle = c.grid
  ctx.lineWidth = 1
  ctx.beginPath(); ctx.moveTo(0, volTop - 2); ctx.lineTo(drawW, volTop - 2); ctx.stroke()

  // ── Candles
  for (let i = 0; i < n; i++) {
    const d = data[i]!
    const x = xOf(i)
    const isUp = d.close >= d.open
    const col  = isUp ? c.up : c.dn

    // Wick
    ctx.strokeStyle = col
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(x, yPrice(d.high))
    ctx.lineTo(x, yPrice(d.low))
    ctx.stroke()

    // Body
    const y1  = Math.min(yPrice(d.open), yPrice(d.close))
    const bH  = Math.max(1, Math.abs(yPrice(d.close) - yPrice(d.open)))
    ctx.fillStyle = col
    ctx.fillRect(x - candW / 2, y1, candW, bH)
  }

  // ── Volume bars
  const upVolAlpha  = isDark.value ? 'rgba(224,82,82,0.32)'  : 'rgba(201,53,53,0.28)'
  const dnVolAlpha  = isDark.value ? 'rgba(61,170,107,0.32)' : 'rgba(31,138,80,0.28)'
  for (let i = 0; i < n; i++) {
    const d = data[i]!
    const x = xOf(i)
    const top = yVol(d.volume)
    ctx.fillStyle = d.close >= d.open ? upVolAlpha : dnVolAlpha
    ctx.fillRect(x - candW / 2, top, candW, volTop + volH - top)
  }

  // ── Crosshair
  const hi = hoveredIdx.value
  if (hi !== null && hi >= 0 && hi < n) {
    const x = xOf(hi)
    const d = data[hi]!
    ctx.strokeStyle = c.xhair
    ctx.lineWidth = 1
    ctx.setLineDash([3, 4])

    // Vertical
    ctx.beginPath(); ctx.moveTo(x, PAD_T); ctx.lineTo(x, H - PAD_B); ctx.stroke()
    // Horizontal (close price)
    const yC = yPrice(d.close)
    ctx.beginPath(); ctx.moveTo(0, yC); ctx.lineTo(drawW, yC); ctx.stroke()
    ctx.setLineDash([])

    // Price label on axis
    ctx.fillStyle = c.label
    ctx.textAlign = 'right'
    ctx.font = 'bold 10.5px "DM Sans", system-ui, sans-serif'
    ctx.fillText(d.close.toFixed(2), W - 4, yC - 4)
  }
}

function onMouseMove(e: MouseEvent) {
  const canvas = canvasRef.value
  const wrap   = canvasWrap.value
  if (!canvas || !wrap || !prices.value?.length) return
  const rect = wrap.getBoundingClientRect()
  const x = e.clientX - rect.left
  const drawW = wrap.clientWidth - PAD_R
  const n = prices.value.length
  const barW = drawW / n
  const idx = Math.floor(x / barW)
  hoveredIdx.value = idx >= 0 && idx < n ? idx : null
  drawChart()
}

function onMouseLeave() {
  hoveredIdx.value = null
  drawChart()
}

function initChart() {
  if (!canvasWrap.value) return
  if (roChart) { roChart.disconnect(); roChart = null }
  drawChart()
  roChart = new ResizeObserver(() => drawChart())
  roChart.observe(canvasWrap.value)
}

onMounted(async () => { await nextTick(); initChart() })
watch(prices, async () => { await nextTick(); drawChart() })
watch(isDark, () => drawChart())
onBeforeUnmount(() => roChart?.disconnect())

// ── 日期範圍快捷 ──────────────────────────
type RangeKey = '1M' | '3M' | '6M' | '1Y' | '2Y'
const activeRange = ref<RangeKey>('1Y')
const ranges: { label: string; key: RangeKey; months: number }[] = [
  { label: '1M', key: '1M', months: 1 },
  { label: '3M', key: '3M', months: 3 },
  { label: '6M', key: '6M', months: 6 },
  { label: '1Y', key: '1Y', months: 12 },
  { label: '2Y', key: '2Y', months: 24 },
]

function setRange(r: typeof ranges[0]) {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth() - r.months, end.getDate())
  from.value = start.toISOString().split('T')[0]
  to.value   = end.toISOString().split('T')[0]
  activeRange.value = r.key
  refreshPrices()
}

// ── 統計 ──────────────────────────────────
const latest = computed(() => prices.value?.[prices.value.length - 1] ?? null)
const priceChange = computed(() => {
  if (!prices.value || prices.value.length < 2) return null
  const prev = prices.value[prices.value.length - 2]!
  const curr = prices.value[prices.value.length - 1]!
  const diff = curr.close - prev.close
  const pct  = (diff / prev.close) * 100
  return { diff, pct }
})

const periodHigh = computed(() =>
  prices.value?.length ? Math.max(...prices.value.map(p => p.high)).toFixed(2) : '—'
)
const periodLow = computed(() =>
  prices.value?.length ? Math.min(...prices.value.map(p => p.low)).toFixed(2) : '—'
)
const avgVolume = computed(() => {
  if (!prices.value?.length) return '—'
  const avg = prices.value.reduce((a, p) => a + p.volume / 1000, 0) / prices.value.length
  return Math.round(avg).toLocaleString()
})

// ── 單檔刷新（SSE）────────────────────────
type RefreshStage = 'idle' | 'running' | 'done' | 'error'
const refreshStage   = ref<RefreshStage>('idle')
const refreshMsg     = ref('')
const refreshProgress = ref(0)
let   refreshEs: EventSource | null = null

function startRefresh() {
  if (refreshStage.value === 'running') return
  refreshStage.value    = 'running'
  refreshMsg.value      = '連接中...'
  refreshProgress.value = 0

  refreshEs = new EventSource(`/api/scraper/prices/stock/${symbol}`)

  refreshEs.onmessage = (e) => {
    try {
      const ev = JSON.parse(e.data) as {
        stage: string; message?: string; progress: number; error?: string; synced?: number
      }
      refreshProgress.value = ev.progress
      if (ev.stage === 'error') {
        refreshStage.value = 'error'
        refreshMsg.value   = ev.error || '發生錯誤'
        refreshEs?.close()
      } else if (ev.stage === 'done') {
        refreshStage.value = 'done'
        refreshMsg.value   = ev.message ?? '更新完成'
        refreshEs?.close()
        // 重新載入圖表資料
        refreshPrices()
        // 3 秒後恢復 idle
        setTimeout(() => { refreshStage.value = 'idle'; refreshMsg.value = '' }, 3000)
      } else {
        refreshMsg.value = ev.message ?? ''
      }
    } catch {}
  }

  refreshEs.onerror = () => {
    refreshStage.value = 'error'
    refreshMsg.value   = '連線中斷，請重試'
    refreshEs?.close()
  }
}
</script>

<template>
  <div class="page" :class="{ light: !isDark, classic: isClassic }">

    <!-- ══ Site Header ══ -->
    <header v-if="!isClassic" class="site-header">
      <div class="site-header__inner">
        <div class="brand">
          <NuxtLink to="/" class="back-link">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M10 13L5 8l5-5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            首頁
          </NuxtLink>
          <span class="brand-sep" aria-hidden="true">/</span>
          <NuxtLink to="/stocks" class="back-link">股票列表</NuxtLink>
          <span class="brand-sep" aria-hidden="true">/</span>
          <span class="brand-cur">{{ symbol }}</span>
        </div>
        <div class="header-right">
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
                  <button class="sp-btn" :class="{ active: !isDark }" @click="setTheme(false)">
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="2.8" fill="currentColor"/><path d="M8 1.5V3M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
                    亮色
                  </button>
                  <button class="sp-btn" :class="{ active: isDark }" @click="setTheme(true)">
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M13.2 9.3A5.8 5.8 0 0 1 6.7 2.8a.4.4 0 0 0-.46-.5A6.3 6.3 0 1 0 13.7 9.76a.4.4 0 0 0-.5-.46Z" fill="currentColor"/></svg>
                    暗色
                  </button>
                </div>
              </div>
              <div class="sp-group">
                <p class="sp-label">版面風格</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: isClassic }" @click="setStyle('classic')">Classic</button>
                  <button class="sp-btn" :class="{ active: isBento }" @click="setStyle('bento')">Bento</button>
                </div>
              </div>
            </div>
          </div>
          <button class="btn-icon" :aria-label="isDark ? '切換亮色模式' : '切換暗色模式'" @click="toggleTheme">
            <svg v-if="isDark" width="16" height="16" viewBox="0 0 16 16" fill="none">
              <circle cx="8" cy="8" r="2.8" fill="currentColor"/>
              <path d="M8 1.5V3M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
            <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M13.2 9.3A5.8 5.8 0 0 1 6.7 2.8a.4.4 0 0 0-.46-.5A6.3 6.3 0 1 0 13.7 9.76a.4.4 0 0 0-.5-.46Z" fill="currentColor"/>
            </svg>
          </button>
        </div>
      </div>
    </header>

    <!-- ══ Classic Header ══ -->
    <header v-else class="classic-header">
      <div class="classic-header__inner">
        <div class="classic-brand">
          <NuxtLink to="/" class="classic-back">← 首頁</NuxtLink>
          <span class="classic-sep">|</span>
          <NuxtLink to="/stocks" class="classic-back">股票列表</NuxtLink>
          <span class="classic-sep">/</span>
          <span class="classic-badge">{{ symbol }}</span>
        </div>
        <div class="classic-header-right">
          <div class="settings-wrap">
            <button class="classic-settings-btn" aria-label="外觀設定" @click="settingsOpen = !settingsOpen">⚙</button>
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
              <div class="sp-group">
                <p class="sp-label">版面風格</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: isClassic }" @click="setStyle('classic')">Classic</button>
                  <button class="sp-btn" :class="{ active: isBento }" @click="setStyle('bento')">Bento</button>
                </div>
              </div>
            </div>
          </div>
          <button class="classic-toggle-btn" @click="toggleTheme">
            <span v-if="isDark">☀</span><span v-else>☾</span>
          </button>
        </div>
      </div>
    </header>

    <div class="content">

      <!-- ══ Stock Hero ══ -->
      <div class="hero">
        <div class="hero-left">
          <p class="hero-eyebrow">{{ symbol }}</p>
          <h1 class="hero-name">{{ stock?.name ?? symbol }}</h1>
        </div>
        <div class="hero-right">
          <div v-if="latest" class="hero-price">
            <span class="price-num" :class="priceChange && priceChange.diff >= 0 ? 'col-up' : 'col-dn'">
              {{ latest.close.toFixed(2) }}
            </span>
            <div v-if="priceChange" class="price-delta" :class="priceChange.diff >= 0 ? 'col-up' : 'col-dn'">
              <span>{{ priceChange.diff >= 0 ? '+' : '' }}{{ priceChange.diff.toFixed(2) }}</span>
              <span>{{ priceChange.diff >= 0 ? '+' : '' }}{{ priceChange.pct.toFixed(2) }}%</span>
            </div>
            <span class="price-date">{{ latest.date.split('T')[0] }}</span>
          </div>
          <!-- 刷新按鈕 -->
          <button
            class="refresh-btn"
            :class="{ 'refresh-btn--running': refreshStage === 'running', 'refresh-btn--done': refreshStage === 'done', 'refresh-btn--error': refreshStage === 'error' }"
            :disabled="refreshStage === 'running'"
            @click="startRefresh"
            :title="refreshStage === 'running' ? refreshMsg : '更新近 3 個月日K 資料'"
          >
            <svg class="refresh-icon" :class="{ spinning: refreshStage === 'running' }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 2v6h-6"/>
              <path d="M3 12a9 9 0 0 1 15-6.7L21 8"/>
              <path d="M3 22v-6h6"/>
              <path d="M21 12a9 9 0 0 1-15 6.7L3 16"/>
            </svg>
            <span>{{ refreshStage === 'running' ? `${refreshProgress}%` : refreshStage === 'done' ? '✓ 完成' : refreshStage === 'error' ? '失敗' : '更新資料' }}</span>
          </button>
        </div>
      </div>

      <!-- 刷新進度提示 -->
      <div v-if="refreshStage !== 'idle'" class="refresh-toast" :class="`refresh-toast--${refreshStage}`">
        <span v-if="refreshStage === 'running'">
          <span class="toast-bar"><span class="toast-fill" :style="{ width: refreshProgress + '%' }"></span></span>
          {{ refreshMsg }}
        </span>
        <span v-else>{{ refreshMsg }}</span>
      </div>

      <!-- ══ Stat Bar ══ -->
      <div class="stat-bar">
        <div class="stat-item">
          <span class="stat-key">開盤</span>
          <span class="stat-val">{{ latest?.open.toFixed(2) ?? '—' }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">最高</span>
          <span class="stat-val col-up">{{ latest?.high.toFixed(2) ?? '—' }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">最低</span>
          <span class="stat-val col-dn">{{ latest?.low.toFixed(2) ?? '—' }}</span>
        </div>
        <div class="stat-divider" />
        <div class="stat-item">
          <span class="stat-key">區間高點</span>
          <span class="stat-val">{{ periodHigh }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">區間低點</span>
          <span class="stat-val">{{ periodLow }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">均成交量（張）</span>
          <span class="stat-val">{{ avgVolume }}</span>
        </div>
      </div>

      <!-- ══ Chart Panel ══ -->
      <div class="chart-panel">
        <div class="chart-toolbar">
          <span class="chart-label">日 K 線圖</span>
          <div class="range-group">
            <button
              v-for="r in ranges"
              :key="r.key"
              class="range-btn"
              :class="{ 'range-btn--active': activeRange === r.key }"
              @click="setRange(r)"
            >{{ r.label }}</button>
          </div>
        </div>

        <div v-if="!prices || prices.length === 0" class="chart-empty">
          此股票尚無日 K 資料，請先在首頁點擊「同步日 K 資料」。
        </div>
        <div v-else ref="canvasWrap" class="chart-container"
          @mousemove="onMouseMove" @mouseleave="onMouseLeave">
          <canvas ref="canvasRef" />
          <!-- Crosshair tooltip -->
          <div
            v-if="hoveredIdx !== null && prices?.[hoveredIdx]"
            class="chart-tooltip"
          >
            <span class="tt-date">{{ prices[hoveredIdx]!.date.split('T')[0] }}</span>
            <span class="tt-row"><em>開</em>{{ prices[hoveredIdx]!.open.toFixed(2) }}</span>
            <span class="tt-row"><em>高</em>{{ prices[hoveredIdx]!.high.toFixed(2) }}</span>
            <span class="tt-row"><em>低</em>{{ prices[hoveredIdx]!.low.toFixed(2) }}</span>
            <span class="tt-row"><em>收</em>
              <b :class="prices[hoveredIdx]!.close >= prices[hoveredIdx]!.open ? 'col-up' : 'col-dn'">
                {{ prices[hoveredIdx]!.close.toFixed(2) }}
              </b>
            </span>
            <span class="tt-row"><em>量</em>{{ Math.round(prices[hoveredIdx]!.volume / 1000).toLocaleString() }}張</span>
          </div>
        </div>
      </div>

      <!-- ══ OHLCV Table ══ -->
      <div v-if="prices && prices.length > 0" class="table-panel">
        <div class="table-topbar">
          <h2 class="table-heading">近期日K</h2>
          <span class="table-count">最近 30 筆</span>
        </div>

        <table class="ohlcv-table">
          <thead>
            <tr>
              <th>日期</th>
              <th class="ra">開盤</th>
              <th class="ra">最高</th>
              <th class="ra">最低</th>
              <th class="ra">收盤</th>
              <th class="ra">成交量（張）</th>
              <th class="ra">成交金額（千元）</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in [...(prices ?? [])].reverse().slice(0, 30)"
              :key="p.id"
            >
              <td class="td-date">{{ p.date.split('T')[0] }}</td>
              <td class="ra">{{ p.open.toFixed(2) }}</td>
              <td class="ra td-high">{{ p.high.toFixed(2) }}</td>
              <td class="ra td-low">{{ p.low.toFixed(2) }}</td>
              <td class="ra td-close" :class="p.close >= p.open ? 'col-up' : 'col-dn'">{{ p.close.toFixed(2) }}</td>
              <td class="ra td-muted">{{ Math.round(p.volume / 1000).toLocaleString() }}</td>
              <td class="ra td-muted">{{ Math.round(p.tx_value / 1000).toLocaleString() }}</td>
            </tr>
          </tbody>
        </table>
      </div>

    </div>
  </div>
</template>

<style scoped>
/* ── Design Tokens（與首頁一致）──────────── */
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
  --dn:    oklch(64%   0.18  148);
  --font:  'DM Sans', system-ui, 'PingFang TC', 'Microsoft JhengHei', sans-serif;
  --mono:  'Fira Code', 'JetBrains Mono', ui-monospace, monospace;

  min-height: 100vh;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
  font-size: 16px;
  line-height: 1.55;
  -webkit-font-smoothing: antialiased;
}

.page *, .page *::before, .page *::after { box-sizing: border-box; margin: 0; padding: 0; }

/* Light Mode */
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
  --dn:    oklch(38%   0.20  148);
}

/* Classic Mode — Original tokens */
.page.classic {
  --bg:    oklch(14.5% 0.016 258);
  --s1:    oklch(19%   0.018 258);
  --s2:    oklch(23%   0.018 258);
  --s3:    oklch(27%   0.020 258);
  --line:  oklch(28%   0.020 258);
  --line2: oklch(36%   0.020 258);
  --blue:  oklch(56%   0.20  264);
  --gold:  oklch(76%   0.095 80);
  --t1:    oklch(97%   0.006 82);
  --t2:    oklch(78%   0.012 258);
  --t3:    oklch(58%   0.014 258);
  --up:    oklch(59%   0.18  22);
  --dn:    oklch(62%   0.17  148);
  --mono:  'Fira Code', 'JetBrains Mono', ui-monospace, monospace;
}
.page.classic.light {
  --bg:    oklch(96.5% 0.007 82);
  --s1:    oklch(93%   0.008 82);
  --s2:    oklch(99%   0.004 82);
  --s3:    oklch(90%   0.007 82);
  --line:  oklch(84%   0.012 258);
  --line2: oklch(68%   0.015 258);
  --blue:  oklch(44%   0.21  264);
  --gold:  oklch(48%   0.13  60);
  --t1:    oklch(13%   0.020 258);
  --t2:    oklch(34%   0.016 258);
  --t3:    oklch(54%   0.014 258);
  --up:    oklch(44%   0.21  22);
  --dn:    oklch(38%   0.19  148);
}

/* ── Site Header ─────────────────────────────────────────── */
.site-header {
  background: color-mix(in oklch, var(--s1) 85%, transparent);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}

.site-header__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 32px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand { display: flex; align-items: center; gap: 10px; }

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  color: var(--t3);
  text-decoration: none;
  transition: color 0.15s;
}
.back-link:hover { color: var(--gold); }

.brand-sep { color: var(--line2); font-size: 14px; user-select: none; }

.brand-cur {
  font-family: var(--mono);
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--t1);
  font-variant-numeric: tabular-nums;
}

.header-right { display: flex; align-items: center; gap: 10px; }

.btn-icon {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--t2);
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s, color 0.2s;
  flex-shrink: 0;
}
.btn-icon:hover { background: var(--s3); border-color: var(--line2); color: var(--t1); }

/* ── Content ───────────────────────────── */
.content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 28px 40px 60px;
  display: flex;
  flex-direction: column;
  gap: 0;
}

/* ── Hero ──────────────────────────────── */
.hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 20px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 0;
}

.hero-eyebrow {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 6px;
}

.hero-name {
  font-size: clamp(24px, 3vw, 36px);
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--t1);
  line-height: 1.1;
}

.hero-price {
  display: flex;
  align-items: baseline;
  gap: 14px;
  flex-wrap: wrap;
}

.price-num {
  font-size: clamp(32px, 4vw, 52px);
  font-weight: 700;
  letter-spacing: -0.04em;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.price-delta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 14px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.3;
}

.price-date {
  font-size: 12px;
  color: var(--t3);
  font-variant-numeric: tabular-nums;
}

.col-up { color: var(--up); }
.col-dn { color: var(--dn); }

/* ── Hero Right（價格 + 刷新）─────────── */
.hero-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
}

/* ── Refresh Button ────────────────────── */
.refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 8px;
  border: 1px solid var(--line2);
  background: var(--s2);
  color: var(--t2);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
  white-space: nowrap;
}
.refresh-btn:hover:not(:disabled) {
  background: var(--s3);
  color: var(--t1);
  border-color: var(--blue);
}
.refresh-btn:disabled { opacity: 0.65; cursor: not-allowed; }
.refresh-btn--done  { border-color: var(--dn); color: var(--dn); }
.refresh-btn--error { border-color: var(--up); color: var(--up); }

.refresh-icon { flex-shrink: 0; transition: transform 0.3s; }
@keyframes spin { to { transform: rotate(360deg); } }
.refresh-icon.spinning { animation: spin 0.9s linear infinite; }

/* ── Refresh Toast ─────────────────────── */
.refresh-toast {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 14px;
  border-radius: 8px;
  font-size: 12.5px;
  color: var(--t2);
  border: 1px solid var(--line);
  background: var(--s1);
  margin-bottom: 4px;
}
.refresh-toast--done  { border-color: var(--dn); color: var(--dn); }
.refresh-toast--error { border-color: var(--up); color: var(--up); }

.toast-bar {
  display: inline-block;
  width: 80px;
  height: 4px;
  background: var(--line2);
  border-radius: 2px;
  overflow: hidden;
}
.toast-fill {
  display: block;
  height: 100%;
  background: var(--blue);
  transition: width 0.3s ease;
}

/* ── Stat Bar ──────────────────────────── */
.stat-bar {
  display: flex;
  align-items: center;
  gap: 0;
  border-bottom: 1px solid var(--line);
  overflow-x: auto;
  scrollbar-width: none;
}
.stat-bar::-webkit-scrollbar { display: none; }

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 24px 14px 0;
  min-width: 90px;
  flex-shrink: 0;
}

.stat-divider {
  width: 1px;
  height: 32px;
  background: var(--line);
  flex-shrink: 0;
  margin: 0 24px 0 0;
}

.stat-key {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
}

.stat-val {
  font-size: 15px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--t1);
}

/* ── Chart Panel ───────────────────────── */
.chart-panel {
  border: 1px solid var(--line);
  border-top: none;
  background: var(--s2);
  border-radius: 0 0 10px 10px;
  margin-bottom: 1px;
  overflow: hidden;
}

.chart-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--line);
}

.chart-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
}

.range-group {
  display: flex;
  gap: 2px;
}

.range-btn {
  font-family: var(--font);
  font-size: 11.5px;
  font-weight: 600;
  padding: 5px 14px;
  background: transparent;
  color: var(--t3);
  border: 1px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  letter-spacing: 0.04em;
  transition: color 0.15s, border-color 0.15s;
}

.range-btn:hover { color: var(--t1); border-color: var(--line2); }
.range-btn--active {
  color: var(--gold);
  border-color: var(--gold);
}

.chart-container {
  width: 100%;
  position: relative;
  cursor: crosshair;
}

.chart-container canvas {
  display: block;
}

.chart-tooltip {
  position: absolute;
  top: 16px;
  left: 16px;
  background: var(--s1);
  border: 1px solid var(--line2);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 110px;
  z-index: 10;
  box-shadow: 0 4px 16px oklch(0% 0 0 / 0.2);
}

.tt-date {
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.08em;
  color: var(--t3);
  margin-bottom: 2px;
}

.tt-row {
  display: flex;
  gap: 6px;
  color: var(--t1);
}

.tt-row em {
  font-style: normal;
  color: var(--t3);
  font-size: 10.5px;
  width: 14px;
}

.chart-empty {
  padding: 64px 20px;
  text-align: center;
  color: var(--t3);
  font-size: 14.5px;
  line-height: 1.8;
}

.chart-loading {
  height: 460px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--t3);
  font-size: 14px;
}

/* ── OHLCV Table ───────────────────────── */
.table-panel {
  border: 1px solid var(--line);
  border-top: none;
  background: var(--s2);
}

.table-topbar {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 14px 20px 13px;
  border-bottom: 1px solid var(--line);
}

.table-heading {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--t1);
}

.table-count {
  font-size: 11px;
  color: var(--t3);
}

.ohlcv-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14.5px;
}

.ohlcv-table th {
  text-align: left;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--t3);
  padding: 11px 16px 10px 0;
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.ohlcv-table th:first-child { padding-left: 20px; }
.ohlcv-table th.ra { text-align: right; }

.ohlcv-table td {
  padding: 10px 16px 10px 0;
  border-bottom: 1px solid var(--line);
  font-variant-numeric: tabular-nums;
  vertical-align: middle;
  color: var(--t1);
}
.ohlcv-table td:first-child { padding-left: 20px; }
.ohlcv-table tr:last-child td { border-bottom: none; }
.ohlcv-table tbody tr:hover td { background: var(--s1); }

.ra { text-align: right; }

.td-date { color: var(--t2); font-size: 13.5px; }
.td-high { color: var(--up); font-weight: 500; }
.td-low  { color: var(--dn); font-weight: 500; }
.td-close { font-weight: 700; }
.td-muted { color: var(--t3); }

/* ── RWD ───────────────────────────────── */
@media (max-width: 720px) {
  .site-header__inner { padding: 0 16px; }
  .content { padding: 16px 16px 40px; }

  .brand-cur { font-size: 13px; }

  .hero { align-items: flex-start; flex-direction: column; }
  .hero-price { gap: 10px; }
  .price-num { font-size: 36px; }

  .stat-item { min-width: 76px; padding-left: 0; }

  .ohlcv-table th:nth-child(7),
  .ohlcv-table td:nth-child(7) { display: none; }
}

/* ── Settings Panel ──────────────────────────────────────── */
.settings-wrap { position: relative; }
.settings-overlay { position: fixed; inset: 0; z-index: 99; }
.settings-panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 100;
  background: var(--s2);
  border: 1px solid var(--line2);
  border-radius: 12px;
  padding: 16px;
  min-width: 196px;
  box-shadow: 0 8px 32px oklch(0% 0 0 / 0.28);
}
.sp-title { font-size: 10.5px; font-weight: 700; letter-spacing: 0.14em; text-transform: uppercase; color: var(--t3); margin-bottom: 12px; }
.sp-group { margin-bottom: 12px; }
.sp-group:last-child { margin-bottom: 0; }
.sp-label { font-size: 10.5px; letter-spacing: 0.10em; text-transform: uppercase; color: var(--t3); margin-bottom: 6px; }
.sp-btns { display: flex; gap: 6px; }
.sp-btn {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  font-family: var(--font);
  font-size: 12px;
  font-weight: 600;
  padding: 7px 8px;
  background: transparent;
  border: 1px solid var(--line2);
  border-radius: 7px;
  color: var(--t2);
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}
.sp-btn:hover { border-color: var(--t2); color: var(--t1); }
.sp-btn.active { background: var(--blue); border-color: var(--blue); color: oklch(97% 0.01 220); }


/* ── Classic structural overrides ──────────────────────────── */
.page.classic .site-header { backdrop-filter: none; -webkit-backdrop-filter: none; background: var(--s1); }
.page.classic .chart-panel { border-radius: 4px; }
.page.classic .chart-toolbar { border-radius: 4px 4px 0 0; }
.page.classic .range-btn { border-radius: 0; }
.page.classic .chart-tooltip { border-radius: 4px; box-shadow: none; }
.page.classic .settings-panel { border-radius: 4px; box-shadow: none; }
.page.classic .sp-btn { border-radius: 0; }
.page.classic .btn-icon { display: none; }

/* ── Classic Header ─────────────────────────────────────────── */
.classic-header {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}
.classic-header__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 32px;
  height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.classic-brand { display: flex; align-items: center; gap: 10px; }
.classic-back { font-size: 12.5px; font-weight: 600; color: var(--t3); text-decoration: none; transition: color 0.15s; }
.classic-back:hover { color: var(--gold); }
.classic-sep { color: var(--line2); font-size: 13px; padding: 0 2px; user-select: none; }
.classic-badge {
  font-family: var(--mono, monospace);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: var(--t1);
  background: var(--s2);
  border: 1px solid var(--line2);
  padding: 3px 8px;
  line-height: 1.4;
}
.classic-header-right { display: flex; align-items: center; gap: 10px; }
.classic-settings-btn {
  background: none;
  border: 1px solid var(--line2);
  color: var(--t2);
  font-size: 14px;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
}
.classic-settings-btn:hover { border-color: var(--gold); color: var(--gold); }
.classic-toggle-btn {
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
}
.classic-toggle-btn:hover { border-color: var(--gold); color: var(--gold); }
</style>
