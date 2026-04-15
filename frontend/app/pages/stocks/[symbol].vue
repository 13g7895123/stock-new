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
const router = useRouter()
const symbol = route.params.symbol as string

// ── 搜尋股票 ──────────────────────────────
const searchQuery = ref('')
const searchFocused = ref(false)

function handleSearch() {
  const input = searchQuery.value.trim()
  if (input) {
    router.push(`/stocks/${input}`)
    searchQuery.value = ''
    searchFocused.value = false
  }
}

function handleSearchKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    handleSearch()
  }
}

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

// ── 主力買賣超 ─────────────────────────────
interface MajorBrokerRecord {
  id: number
  broker_name: string
  buy_vol: number
  sell_vol: number
  net_vol: number
  pct: number
  rank: number
  side: string
}

interface MajorData {
  symbol: string
  days: number
  data_date: string | null
  buy: MajorBrokerRecord[]
  sell: MajorBrokerRecord[]
}

const majorDays = ref(1)
const { data: majorData } = await useFetch<MajorData>(
  () => `/api/major/${symbol}?days=${majorDays.value}`,
)

// ── 主題 + 風格 ─────────────────────────────────────────────────────────
const { isDark, appStyle, isBento, isClassic, toggleTheme, setTheme, setStyle } = useAppPrefs()
const settingsOpen = ref(false)

// ── 即時報價（移到前面避免初始化順序問題）────────────────────────────────
interface RealtimeQuote {
  symbol: string
  name: string
  price: number
  open: number
  high: number
  low: number
  prev_close: number
  change: number
  change_pct: number
  volume: number
  trade_time: string
  trade_date: string
  is_trading: boolean
}

const realtime  = ref<RealtimeQuote | null>(null)
const rtLoading = ref(false)
const rtError   = ref(false)
let   rtTimer: ReturnType<typeof setInterval> | null = null

// ── Canvas K-line Chart ───────────────────
const canvasRef = ref<HTMLCanvasElement | null>(null)
const canvasWrap = ref<HTMLElement | null>(null)
const hoveredIdx = ref<number | null>(null)
let roChart: ResizeObserver | null = null

// ── MA Lines ───────────────────────────────────────────────
interface MALine { period: number; color: string; enabled: boolean }
const maLines = ref<MALine[]>([
  { period: 5,   color: '#f0a842', enabled: true  },
  { period: 10,  color: '#5b9cf6', enabled: true  },
  { period: 24,  color: '#a78ce8', enabled: true  },
  { period: 72,  color: '#4ecfa8', enabled: false },
  { period: 120, color: '#e07b5a', enabled: false },
])

function calcMA(data: DailyPrice[], period: number): (number | null)[] {
  const result: (number | null)[] = new Array(data.length).fill(null)
  let sum = 0
  for (let i = 0; i < data.length; i++) {
    sum += data[i]!.close
    if (i >= period) sum -= data[i - period]!.close
    if (i >= period - 1) result[i] = sum / period
  }
  return result
}

// ── 計算包含即時資料的擴展資料陣列（用於均線計算）──────────────────
function getExtendedData(): DailyPrice[] {
  const data = prices.value ?? []
  if (!realtime.value?.is_trading || realtime.value.price <= 0 || realtime.value.open <= 0) {
    return data
  }
  
  // 建立即時資料點（模擬成 DailyPrice 格式）
  const rt = realtime.value
  const rtData: DailyPrice = {
    id: -1,  // 特殊 ID 表示即時資料
    symbol: symbol,
    date: new Date().toISOString(),
    open: rt.open,
    high: rt.high,
    low: rt.low,
    close: rt.price,  // 即時價格作為收盤價
    volume: rt.volume * 1000,  // 張數轉股數
    tx_value: 0,
    tx_count: 0,
  }
  
  return [...data, rtData]
}

// ── 計算 hover K 棒的均線數值 ──────────────────────────────────────
const hoveredMaValues = computed<Record<number, number | null>>(() => {
  if (hoveredIdx.value === null) return {}
  
  const data = prices.value ?? []
  if (hoveredIdx.value >= data.length) return {}
  
  const result: Record<number, number | null> = {}
  for (const ma of maLines.value) {
    if (!ma.enabled) continue
    const maValues = calcMA(data, ma.period)
    result[ma.period] = maValues[hoveredIdx.value] ?? null
  }
  
  return result
})

// ── Zoom / Pan ─────────────────────────────────────────────
const viewStart = ref(0)
const viewEnd   = ref(0)

let isDragging  = false
let dragStartX  = 0
let dragStartVS = 0
let dragSpan    = 0

function initView() {
  const n = prices.value?.length ?? 0
  viewStart.value = 0
  viewEnd.value   = n
}

function clampView(s: number, e: number, n: number): { s: number; e: number } {
  const minSpan = Math.min(20, n)
  s = Math.max(0, Math.round(s))
  e = Math.min(n, Math.round(e))
  if (e - s < minSpan) {
    if (e >= n) s = Math.max(0, n - minSpan)
    else        e = Math.min(n, s + minSpan)
  }
  return { s, e }
}

// ── Horizontal Lines ───────────────────────────────────────
const hLines = ref<number[]>([])
const isDrawMode = ref(false)

function priceAtY(clientY: number): number | null {
  const data = prices.value
  if (!data?.length || !canvasWrap.value) return null
  const wrap = canvasWrap.value
  const rect = wrap.getBoundingClientRect()
  const y = clientY - rect.top
  const H = CHART_H
  const priceH = Math.round((H - PAD_T - PAD_B - GAP_V) * PRICE_RATIO)
  if (y < PAD_T || y > PAD_T + priceH) return null
  const vs = viewStart.value
  const ve = viewEnd.value
  const slice = data.slice(vs, ve)
  if (!slice.length) return null
  const pMin = slice.reduce((m, d) => Math.min(m, d.low), Infinity)  * 0.9975
  const pMax = slice.reduce((m, d) => Math.max(m, d.high), -Infinity) * 1.0025
  return pMax - (pMax - pMin) * ((y - PAD_T) / priceH)
}

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

  const vs = Math.max(0, viewStart.value)
  const ve = Math.min(viewEnd.value, data.length)
  const slice = data.slice(vs, ve)
  const n = slice.length
  if (n === 0) return

  const barW  = drawW / n
  const candW = Math.max(1, Math.min(barW * 0.65, 14))

  // 檢查是否有即時資料需要顯示
  const hasRealtime = realtime.value?.is_trading && realtime.value.price > 0 && realtime.value.open > 0

  // Price range from visible slice (包含即時資料)
  let pMin = slice.reduce((m, d) => Math.min(m, d.low), Infinity)   * 0.9975
  let pMax = slice.reduce((m, d) => Math.max(m, d.high), -Infinity) * 1.0025
  
  // 如果有即時資料，擴展價格範圍
  if (hasRealtime) {
    const rt = realtime.value!
    pMin = Math.min(pMin, rt.low) * 0.9975
    pMax = Math.max(pMax, rt.high) * 1.0025
  }

  const vMax = (slice.reduce((m, d) => Math.max(m, d.volume), 0) * 1.05) || 1

  const xOf    = (i: number) => i * barW + barW / 2
  const yPrice = (p: number) => PAD_T + priceH * (1 - (p - pMin) / (pMax - pMin))
  const yVol   = (v: number) => volTop + volH * (1 - v / vMax)

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
    const date = slice[i]!.date.split('T')[0]!
    ctx.fillText(date.substring(5), x, H - 6)  // MM-DD
  }

  // ── Separating line between price & volume
  ctx.strokeStyle = c.grid
  ctx.lineWidth = 1
  ctx.beginPath(); ctx.moveTo(0, volTop - 2); ctx.lineTo(drawW, volTop - 2); ctx.stroke()

  // ── Candles
  for (let i = 0; i < n; i++) {
    const d = slice[i]!
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
    const d = slice[i]!
    const x = xOf(i)
    const top = yVol(d.volume)
    ctx.fillStyle = d.close >= d.open ? upVolAlpha : dnVolAlpha
    ctx.fillRect(x - candW / 2, top, candW, volTop + volH - top)
  }

  // ── Realtime K 棒（盤中即時，在最右側繪製）────────────────────
  if (hasRealtime) {
    const rt = realtime.value!
    // 計算即時 K 棒的 X 位置（在最後一根歷史 K 棒右側，留一點間隔）
    const rtX = xOf(n - 1) + barW * 1.2
    const rtIsUp = rt.price >= rt.open
    const rtCol = rtIsUp ? c.up : c.dn
    const rtColAlpha = rtIsUp ? upVolAlpha : dnVolAlpha

    // 繪製即時 K 棒的 Wick（影線）- 使用虛線以示區別
    ctx.strokeStyle = rtCol
    ctx.lineWidth = 1.5
    ctx.setLineDash([2, 2])  // 虛線
    ctx.beginPath()
    ctx.moveTo(rtX, yPrice(rt.high))
    ctx.lineTo(rtX, yPrice(rt.low))
    ctx.stroke()
    ctx.setLineDash([])

    // 繪製即時 K 棒的 Body（實體）
    const rtY1 = Math.min(yPrice(rt.open), yPrice(rt.price))
    const rtBH = Math.max(1, Math.abs(yPrice(rt.price) - yPrice(rt.open)))
    ctx.fillStyle = rtCol
    ctx.globalAlpha = 0.8  // 稍微透明以示區別
    ctx.fillRect(rtX - candW / 2, rtY1, candW, rtBH)
    ctx.globalAlpha = 1.0

    // 繪製即時成交量
    if (rt.volume > 0) {
      const rtVolY = yVol(rt.volume * 1000)  // 轉換成股數（realtime.volume 是張數）
      ctx.fillStyle = rtColAlpha
      ctx.globalAlpha = 0.7
      ctx.fillRect(rtX - candW / 2, rtVolY, candW, volTop + volH - rtVolY)
      ctx.globalAlpha = 1.0
    }

    // 在即時 K 棒上方標註「即時」
    ctx.fillStyle = rtCol
    ctx.font = 'bold 9px "DM Sans", system-ui, sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('即時', rtX, PAD_T + 10)

    // 在右側價格軸標註即時價格（帶背景）
    const rtPriceY = yPrice(rt.price)
    const priceText = rt.price.toFixed(2)
    ctx.font = 'bold 10px "DM Sans", system-ui, sans-serif'
    ctx.textAlign = 'right'
    const textW = ctx.measureText(priceText).width
    
    // 繪製背景矩形
    ctx.fillStyle = rtCol
    ctx.fillRect(W - textW - 8, rtPriceY - 9, textW + 6, 13)
    
    // 繪製白色文字
    ctx.fillStyle = isDark.value ? 'oklch(100% 0 0)' : 'oklch(100% 0 0)'
    ctx.fillText(priceText, W - 4, rtPriceY + 1)
  }

  // ── MA Lines (use full data for warmup, draw only visible range, extend to realtime)
  ctx.setLineDash([])
  
  // 計算包含即時資料的擴展資料（如果有的話）
  const extendedData = getExtendedData()
  const hasRealtimeInMA = extendedData.length > data.length
  
  for (const ma of maLines.value) {
    if (!ma.enabled) continue
    const maValues = calcMA(extendedData, ma.period)
    ctx.beginPath()
    ctx.strokeStyle = ma.color
    ctx.lineWidth   = 1.5
    let started = false
    
    // 繪製歷史資料的均線
    for (let i = 0; i < n; i++) {
      const mv = maValues[vs + i]
      if (mv == null) { started = false; continue }
      const x = xOf(i)
      const y = yPrice(mv)
      if (!started) { ctx.moveTo(x, y); started = true }
      else ctx.lineTo(x, y)
    }
    
    // 如果有即時資料，延伸均線到即時 K 棒
    if (hasRealtimeInMA && hasRealtime) {
      const rtMaValue = maValues[extendedData.length - 1]
      if (rtMaValue != null) {
        const rtX = xOf(n - 1) + barW * 1.2  // 與即時 K 棒的 X 位置一致
        const rtY = yPrice(rtMaValue)
        
        // 使用虛線連接到即時點
        ctx.setLineDash([3, 3])
        ctx.lineTo(rtX, rtY)
        ctx.stroke()
        ctx.setLineDash([])
        
        // 在即時點繪製一個小圓點
        ctx.beginPath()
        ctx.arc(rtX, rtY, 2.5, 0, Math.PI * 2)
        ctx.fillStyle = ma.color
        ctx.fill()
      }
    } else {
      ctx.stroke()
    }
  }

  // ── Horizontal Lines
  for (const price of hLines.value) {
    const y = yPrice(price)
    if (y < PAD_T - 6 || y > PAD_T + priceH + 6) continue
    const hCol = isDark.value ? 'rgba(255,220,80,0.65)' : 'rgba(140,100,0,0.75)'
    ctx.strokeStyle = hCol
    ctx.lineWidth   = 1
    ctx.setLineDash([5, 4])
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(drawW, y)
    ctx.stroke()
    ctx.setLineDash([])
    ctx.fillStyle  = hCol
    ctx.textAlign  = 'right'
    ctx.font       = 'bold 10px "DM Sans", system-ui, sans-serif'
    ctx.fillText(price.toFixed(2), W - 4, y - 3)
  }
  ctx.setLineDash([])

  // ── Crosshair
  const hi = hoveredIdx.value
  if (hi !== null && hi >= vs && hi < ve) {
    const localI = hi - vs
    const x = xOf(localI)
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
  const wrap = canvasWrap.value
  if (!wrap || !prices.value?.length) return
  const rect  = wrap.getBoundingClientRect()
  const x     = e.clientX - rect.left
  const vs    = viewStart.value
  const ve    = viewEnd.value
  const drawW = wrap.clientWidth - PAD_R
  const visN  = ve - vs
  if (visN <= 0) return
  const barW    = drawW / visN
  const localI  = Math.floor(x / barW)
  const idx     = vs + localI
  hoveredIdx.value = (idx >= vs && idx < ve) ? idx : null

  if (isDragging && !isDrawMode.value) {
    const dx   = e.clientX - dragStartX
    const barsPerPx = dragSpan / drawW
    const shift = Math.round(-dx * barsPerPx)
    const cv = clampView(dragStartVS + shift, dragStartVS + shift + dragSpan, prices.value.length)
    viewStart.value = cv.s
    viewEnd.value   = cv.e
  }
  drawChart()
}

function onMouseLeave() {
  hoveredIdx.value = null
  isDragging = false
  drawChart()
}

function onMouseDown(e: MouseEvent) {
  if (!prices.value?.length) return
  if (e.button === 2) return  // handled by onContextMenu
  if (isDrawMode.value) {
    const p = priceAtY(e.clientY)
    if (p !== null) {
      hLines.value = [...hLines.value, parseFloat(p.toFixed(2))]
      drawChart()
    }
  } else {
    isDragging  = true
    dragStartX  = e.clientX
    dragStartVS = viewStart.value
    dragSpan    = viewEnd.value - viewStart.value
  }
}

function onMouseUp() {
  isDragging = false
}

function onContextMenu(e: MouseEvent) {
  e.preventDefault()
  if (hLines.value.length === 0) return
  const p = priceAtY(e.clientY)
  if (p === null) return
  const data  = prices.value!
  const slice = data.slice(viewStart.value, viewEnd.value)
  const pMin  = slice.reduce((m, d) => Math.min(m, d.low), Infinity)   * 0.9975
  const pMax  = slice.reduce((m, d) => Math.max(m, d.high), -Infinity) * 1.0025
  const range = pMax - pMin || 1
  const nearest = hLines.value.reduce((a, b) => Math.abs(b - p) < Math.abs(a - p) ? b : a)
  if (Math.abs(nearest - p) / range < 0.05) {
    hLines.value = hLines.value.filter(h => h !== nearest)
    drawChart()
  }
}

function onWheel(ev: WheelEvent) {
  ev.preventDefault()
  const data = prices.value
  if (!data?.length || !canvasWrap.value) return
  const n    = data.length
  const wrap = canvasWrap.value
  const drawW  = wrap.clientWidth - PAD_R
  const rect   = wrap.getBoundingClientRect()
  const mouseX = ev.clientX - rect.left
  const ratio  = Math.min(1, Math.max(0, mouseX / drawW))
  const span   = viewEnd.value - viewStart.value
  const factor = ev.deltaY > 0 ? 1.15 : (1 / 1.15)
  const newSpan = Math.min(n, Math.max(20, Math.round(span * factor)))
  const pivot   = viewStart.value + ratio * span
  const cv = clampView(pivot - ratio * newSpan, pivot - ratio * newSpan + newSpan, n)
  viewStart.value = cv.s
  viewEnd.value   = cv.e
  drawChart()
}

function initChart() {
  if (!canvasWrap.value) return
  if (roChart) { roChart.disconnect(); roChart = null }
  initView()
  drawChart()
  roChart = new ResizeObserver(() => drawChart())
  roChart.observe(canvasWrap.value)
}

onMounted(async () => { await nextTick(); initChart() })
watch(prices, async () => { await nextTick(); initView(); drawChart() })
watch(isDark, () => drawChart())
watch(realtime, () => drawChart(), { deep: true })
onBeforeUnmount(() => {
  roChart?.disconnect()
  if (chipsStatusPoll) clearInterval(chipsStatusPoll)
  if (rtTimer) clearInterval(rtTimer)
})

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

// ── 即時報價相關函數 ────────────────────────────────
function isTradingHours(): boolean {
  const now = new Date()
  const day = now.getDay()                     // 0=日, 6=六
  if (day === 0 || day === 6) return false
  const hhmm = now.getHours() * 100 + now.getMinutes()
  return hhmm >= 900 && hhmm <= 1330
}

async function fetchRealtime() {
  rtLoading.value = true
  try {
    const data = await $fetch<RealtimeQuote>(`/api/realtime/${symbol}`)
    realtime.value = data
    rtError.value  = false
  } catch {
    rtError.value = true
  } finally {
    rtLoading.value = false
  }
}

// 一載入就抓一次，盤中每 10 秒輪詢
fetchRealtime()
rtTimer = setInterval(() => {
  if (isTradingHours()) fetchRealtime()
}, 10_000)

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

// ── 券商勝率 ─────────────────────────────────────────────────────
interface BrokerWinrate {
  broker_name: string
  total_trades: number
  win_trades: number
  win_rate_pct: number | null
  avg_return_pct: number | null
  avg_holding_days: number | null
  max_return_pct: number | null
  last_entry_date: string | null
}
interface BrokerTradeEvent {
  id: number
  broker_name: string
  entry_date: string
  exit_date: string | null
  entry_close: number
  exit_close: number | null
  return_pct: number | null
  holding_days: number | null
  is_win: boolean | null
}

const { data: winrateData, refresh: refreshWinrate } = await useFetch<BrokerWinrate[]>(
  `/api/stocks/${symbol}/broker-winrate?min_entries=2`,
  { default: () => [] }
)
const expandedWinrateBroker = ref('')
const winrateEvents         = ref<BrokerTradeEvent[]>([])
const winrateEventsLoading  = ref(false)
const winrateTriggerState   = ref<'idle' | 'running' | 'done' | 'error'>('idle')

async function toggleWinrateBroker(brokerName: string) {
  if (expandedWinrateBroker.value === brokerName) {
    expandedWinrateBroker.value = ''
    return
  }
  expandedWinrateBroker.value = brokerName
  winrateEventsLoading.value  = true
  try {
    winrateEvents.value = await $fetch<BrokerTradeEvent[]>(
      `/api/stocks/${symbol}/broker-winrate/events?broker=${encodeURIComponent(brokerName)}`
    )
  } finally {
    winrateEventsLoading.value = false
  }
}

async function triggerWinrate() {
  if (winrateTriggerState.value === 'running') return
  winrateTriggerState.value = 'running'
  try {
    await $fetch('/api/winrate/trigger-single', { method: 'POST', body: { symbol } })
    winrateTriggerState.value = 'done'
    setTimeout(() => { winrateTriggerState.value = 'idle' }, 3000)
    await refreshWinrate()
  } catch {
    winrateTriggerState.value = 'error'
    setTimeout(() => { winrateTriggerState.value = 'idle' }, 3000)
  }
}

function winrateClass(pct: number | null): string {
  if (pct === null) return ''
  if (pct >= 60) return 'wr-high'
  if (pct >= 40) return 'wr-mid'
  return 'wr-low'
}

function returnColorClass(val: number | null): string {
  if (val === null) return ''
  if (val > 0) return 'col-up'
  if (val < 0) return 'col-dn'
  return ''
}

// ── 籌碼金字塔 ─────────────────────────────────────────────────────
interface ChipsDistribution {
  id: number
  snapshot_id: number
  tier_rank: number
  range_label: string
  holder_count: number | null
  holder_pct: number | null
  share_count: number | null
  share_pct: number | null
  cum_holder_pct: number | null
  cum_share_pct: number | null
}

interface ChipsSnapshot {
  id: number
  symbol: string
  data_date: string
  scraped_at: string
  distributions: ChipsDistribution[]
}

const { data: chipsLatest, refresh: refreshChips } = await useFetch<{ data: ChipsSnapshot | null }>(
  `/api/chips/${symbol}/latest`,
  { default: () => ({ data: null }) }
)

const chipsViewMode = ref<'share' | 'holder'>('share')
const chipsTriggerState = ref<'idle' | 'running' | 'done' | 'error'>('idle')
let chipsStatusPoll: ReturnType<typeof setInterval> | null = null

// 依 tier_rank 降序排列（大股東在上方）
const pyramidRows = computed(() => {
  const dists = chipsLatest.value?.data?.distributions ?? []
  return [...dists].sort((a, b) => b.tier_rank - a.tier_rank)
})

function pyramidBarWidth(dist: ChipsDistribution): number {
  const dists = chipsLatest.value?.data?.distributions ?? []
  const maxVal = chipsViewMode.value === 'share'
    ? Math.max(...dists.map(d => d.share_pct ?? 0), 0.001)
    : Math.max(...dists.map(d => d.holder_pct ?? 0), 0.001)
  const val = chipsViewMode.value === 'share' ? (dist.share_pct ?? 0) : (dist.holder_pct ?? 0)
  return (val / maxVal) * 100
}

function pyramidBarClass(dist: ChipsDistribution): string {
  const dists = chipsLatest.value?.data?.distributions ?? []
  const total = dists.length || 1
  const ratio = dist.tier_rank / total  // ratio 大 = 大股東
  if (ratio > 0.67) return 'pyramid-bar--large'
  if (ratio > 0.33) return 'pyramid-bar--medium'
  return 'pyramid-bar--small'
}

async function triggerChips() {
  if (chipsTriggerState.value === 'running') return
  chipsTriggerState.value = 'running'
  try {
    await $fetch('/api/chips/trigger-single', { method: 'POST', body: { symbol } })
  } catch (e: any) {
    if (e?.status !== 409) {
      chipsTriggerState.value = 'error'
      setTimeout(() => { chipsTriggerState.value = 'idle' }, 3000)
      return
    }
    // 409 = 已有任務執行中，繼續輪詢
  }
  let pollCount = 0
  chipsStatusPoll = setInterval(async () => {
    pollCount++
    if (pollCount > 60) {  // 2 分鐘逾時
      clearInterval(chipsStatusPoll!)
      chipsStatusPoll = null
      chipsTriggerState.value = 'error'
      setTimeout(() => { chipsTriggerState.value = 'idle' }, 3000)
      return
    }
    try {
      const s = await $fetch<{ status: string }>('/api/chips/status')
      if (s.status !== 'running') {
        clearInterval(chipsStatusPoll!)
        chipsStatusPoll = null
        chipsTriggerState.value = s.status === 'completed' ? 'done' : 'error'
        await refreshChips()
        setTimeout(() => { chipsTriggerState.value = 'idle' }, 3000)
      }
    } catch { /* 忽略輪詢錯誤 */ }
  }, 2000)
}

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
          <!-- 搜尋框 -->
          <div class="search-box" :class="{ 'search-box--focused': searchFocused }">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" class="search-icon">
              <circle cx="7" cy="7" r="5" stroke="currentColor" stroke-width="1.5" fill="none"/>
              <path d="M11 11l3.5 3.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜尋股票代碼"
              class="search-input"
              @focus="searchFocused = true"
              @blur="searchFocused = false"
              @keydown="handleSearchKeydown"
            />
            <button
              v-if="searchQuery"
              class="search-clear"
              @click="searchQuery = ''"
              type="button"
            >
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
                <path d="M12 4L4 12M4 4l8 8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
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
          <!-- 搜尋框 -->
          <div class="search-box classic-search" :class="{ 'search-box--focused': searchFocused }">
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none" class="search-icon">
              <circle cx="7" cy="7" r="5" stroke="currentColor" stroke-width="1.5" fill="none"/>
              <path d="M11 11l3.5 3.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜尋股票代碼"
              class="search-input"
              @focus="searchFocused = true"
              @blur="searchFocused = false"
              @keydown="handleSearchKeydown"
            />
            <button
              v-if="searchQuery"
              class="search-clear"
              @click="searchQuery = ''"
              type="button"
            >
              <svg width="11" height="11" viewBox="0 0 16 16" fill="none">
                <path d="M12 4L4 12M4 4l8 8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
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

      <!-- ══ Realtime Quote Bar ══ -->
      <div class="rt-bar" :class="{ 'rt-bar--active': realtime?.is_trading, 'rt-bar--error': rtError }">
        <div class="rt-badge" :class="realtime?.is_trading ? 'rt-badge--live' : 'rt-badge--off'">
          <span v-if="realtime?.is_trading" class="rt-dot" />
          {{ realtime?.is_trading ? '盤中即時' : '非交易時段' }}
        </div>
        <template v-if="realtime && !rtError">
          <div class="rt-price" :class="(realtime.change ?? 0) >= 0 ? 'col-up' : 'col-dn'">
            {{ realtime.is_trading ? realtime.price.toFixed(2) : '—' }}
          </div>
          <div v-if="realtime.is_trading" class="rt-delta" :class="realtime.change >= 0 ? 'col-up' : 'col-dn'">
            {{ realtime.change >= 0 ? '+' : '' }}{{ realtime.change.toFixed(2) }}
            （{{ realtime.change >= 0 ? '+' : '' }}{{ realtime.change_pct.toFixed(2) }}%）
          </div>
          <div class="rt-divider" />
          <div class="rt-item"><span class="rt-key">開盤</span><span class="rt-val">{{ realtime.open > 0 ? realtime.open.toFixed(2) : '—' }}</span></div>
          <div class="rt-item"><span class="rt-key">最高</span><span class="rt-val col-up">{{ realtime.high > 0 ? realtime.high.toFixed(2) : '—' }}</span></div>
          <div class="rt-item"><span class="rt-key">最低</span><span class="rt-val col-dn">{{ realtime.low > 0 ? realtime.low.toFixed(2) : '—' }}</span></div>
          <div class="rt-item"><span class="rt-key">成交量</span><span class="rt-val">{{ realtime.volume > 0 ? realtime.volume.toLocaleString() + ' 張' : '—' }}</span></div>
          <div v-if="realtime.trade_time" class="rt-time">{{ realtime.trade_time }}</div>
        </template>
        <span v-else-if="rtLoading" class="rt-loading">載入中…</span>
        <span v-else-if="rtError" class="rt-err">無法取得即時資料</span>
        <button class="rt-refresh" :disabled="rtLoading" @click="fetchRealtime" title="手動刷新">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" :class="{ spinning: rtLoading }">
            <path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/>
            <path d="M3 22v-6h6"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/>
          </svg>
        </button>
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
          <div class="toolbar-left">
            <span class="chart-label">日 K 線圖</span>
            <div class="ma-group">
              <button
                v-for="ma in maLines"
                :key="ma.period"
                class="ma-btn"
                :class="{ 'ma-btn--active': ma.enabled }"
                :style="ma.enabled ? { borderColor: ma.color, color: ma.color } : {}"
                @click="ma.enabled = !ma.enabled; drawChart()"
              >MA{{ ma.period }}</button>
            </div>
          </div>
          <div class="toolbar-right">
            <button
              class="draw-btn"
              :class="{ 'draw-btn--active': isDrawMode }"
              :title="isDrawMode ? '點選價格區新增水平線，右鍵移除' : '開啟畫線模式'"
              @click="isDrawMode = !isDrawMode"
            >
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
                <line x1="2" y1="8" x2="14" y2="8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                <circle cx="2" cy="8" r="1.5" fill="currentColor"/>
                <circle cx="14" cy="8" r="1.5" fill="currentColor"/>
              </svg>
              {{ isDrawMode ? '畫線中' : '畫線' }}
            </button>
            <button
              v-if="hLines.length > 0"
              class="draw-btn"
              title="清除所有水平線"
              @click="hLines = []; drawChart()"
            >清除({{ hLines.length }})</button>
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
        </div>

        <div v-if="!prices || prices.length === 0" class="chart-empty">
          此股票尚無日 K 資料，請先在首頁點擊「同步日 K 資料」。
        </div>
        <div v-else ref="canvasWrap" class="chart-container"
          :class="{ 'chart-container--draw': isDrawMode }"
          @mousemove="onMouseMove"
          @mouseleave="onMouseLeave"
          @mousedown="onMouseDown"
          @mouseup="onMouseUp"
          @wheel.prevent="onWheel"
          @contextmenu="onContextMenu">
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
            
            <!-- 均線數值 -->
            <div v-if="Object.keys(hoveredMaValues).length > 0" class="tt-divider" />
            <span
              v-for="ma in maLines.filter(m => m.enabled)"
              :key="ma.period"
              class="tt-ma"
              :style="{ color: ma.color }"
            >
              <em>MA{{ ma.period }}</em>
              {{ hoveredMaValues[ma.period]?.toFixed(2) ?? '—' }}
            </span>
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

      <!-- ══ 主力買賣超 ══ -->
      <div class="major-panel">
        <div class="major-topbar">
          <div class="major-topbar-left">
            <h2 class="table-heading">主力買賣超</h2>
            <span v-if="majorData?.data_date" class="table-count">資料日期：{{ majorData.data_date }}</span>
            <span v-else class="table-count">尚無資料</span>
          </div>
          <div class="range-group">
            <button
              v-for="d in [1, 5, 10, 20, 60]"
              :key="d"
              class="range-btn"
              :class="{ 'range-btn--active': majorDays === d }"
              @click="majorDays = d"
            >{{ d }}日</button>
          </div>
        </div>

        <div v-if="!majorData?.data_date" class="chart-empty">
          尚無主力資料，請先在首頁觸發「主力進出爬取」。
        </div>
        <div v-else class="major-body">
          <!-- 買超 -->
          <div class="major-side">
            <div class="major-side-header buy-header">買超前 10 名</div>
            <table class="major-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>券商</th>
                  <th class="ra">買進（張）</th>
                  <th class="ra">賣出（張）</th>
                  <th class="ra">買賣超（張）</th>
                  <th class="ra">佔比</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in majorData.buy" :key="r.id">
                  <td class="td-rank td-muted">{{ r.rank }}</td>
                  <td class="td-broker">{{ r.broker_name }}</td>
                  <td class="ra">{{ r.buy_vol.toLocaleString() }}</td>
                  <td class="ra td-muted">{{ r.sell_vol.toLocaleString() }}</td>
                  <td class="ra td-high">+{{ r.net_vol.toLocaleString() }}</td>
                  <td class="ra td-muted">{{ r.pct.toFixed(2) }}%</td>
                </tr>
              </tbody>
            </table>
          </div>
          <!-- 賣超 -->
          <div class="major-side">
            <div class="major-side-header sell-header">賣超前 10 名</div>
            <table class="major-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>券商</th>
                  <th class="ra">買進（張）</th>
                  <th class="ra">賣出（張）</th>
                  <th class="ra">買賣超（張）</th>
                  <th class="ra">佔比</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in majorData.sell" :key="r.id">
                  <td class="td-rank td-muted">{{ r.rank }}</td>
                  <td class="td-broker">{{ r.broker_name }}</td>
                  <td class="ra td-muted">{{ r.buy_vol.toLocaleString() }}</td>
                  <td class="ra">{{ r.sell_vol.toLocaleString() }}</td>
                  <td class="ra td-low">{{ r.net_vol.toLocaleString() }}</td>
                  <td class="ra td-muted">{{ r.pct.toFixed(2) }}%</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ══ 籌碼金字塔 ══ -->
      <div class="major-panel chips-panel">
        <div class="major-topbar">
          <div class="major-topbar-left">
            <h2 class="table-heading">籌碼金字塔</h2>
            <span v-if="chipsLatest?.data" class="table-count">
              資料日期：{{ chipsLatest.data.data_date.split('T')[0] }}
            </span>
            <span v-else class="table-count">持股分佈</span>
          </div>
          <div class="chips-topbar-right">
            <div v-if="chipsLatest?.data" class="range-group">
              <button
                class="range-btn"
                :class="{ 'range-btn--active': chipsViewMode === 'share' }"
                @click="chipsViewMode = 'share'"
              >股數%</button>
              <button
                class="range-btn"
                :class="{ 'range-btn--active': chipsViewMode === 'holder' }"
                @click="chipsViewMode = 'holder'"
              >人數%</button>
            </div>
            <button
              class="range-btn chips-trigger-btn"
              :class="{ 'range-btn--active': chipsTriggerState === 'running' }"
              :disabled="chipsTriggerState === 'running'"
              @click="triggerChips"
            >
              <svg v-if="chipsTriggerState === 'running'" class="chips-spin" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                <path d="M21 12a9 9 0 1 1-6.22-8.56"/>
              </svg>
              {{ chipsTriggerState === 'running' ? '爬取中…' : chipsTriggerState === 'done' ? '✓ 完成' : chipsTriggerState === 'error' ? '失敗' : chipsLatest?.data ? '重新爬取' : '爬取資料' }}
            </button>
          </div>
        </div>

        <div v-if="!chipsLatest?.data" class="chart-empty">
          尚無籌碼金字塔資料，請點「爬取資料」開始抓取。
        </div>
        <div v-else class="pyramid-body">
          <div class="pyramid-rows">
            <div
              v-for="dist in pyramidRows"
              :key="dist.id"
              class="pyramid-row"
            >
              <div class="pyramid-label">{{ dist.range_label }}</div>
              <div class="pyramid-bar-wrap">
                <div
                  class="pyramid-bar"
                  :class="pyramidBarClass(dist)"
                  :style="{ width: pyramidBarWidth(dist) + '%' }"
                ></div>
              </div>
              <div class="pyramid-vals">
                <span class="pyramid-pct">
                  {{ chipsViewMode === 'share'
                    ? (dist.share_pct?.toFixed(2) ?? '—')
                    : (dist.holder_pct?.toFixed(2) ?? '—') }}%
                </span>
                <span class="pyramid-count td-muted">
                  {{ dist.holder_count?.toLocaleString() ?? '—' }} 人
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ══ 券商勝率分析 ══ -->
      <div class="major-panel winrate-panel">
        <div class="major-topbar">
          <div class="major-topbar-left">
            <h2 class="table-heading">券商勝率分析</h2>
            <span class="table-count">至少 2 次交易</span>
          </div>
          <button
            class="range-btn"
            :class="{ 'range-btn--active': winrateTriggerState === 'running' }"
            :disabled="winrateTriggerState === 'running'"
            @click="triggerWinrate"
          >
            {{ winrateTriggerState === 'running' ? '計算中…' : winrateTriggerState === 'done' ? '已更新' : '重新計算' }}
          </button>
        </div>

        <div v-if="!winrateData?.length" class="chart-empty">
          尚無勝率資料，請點「重新計算」。
        </div>
        <div v-else class="wr-list">
          <div
            v-for="row in winrateData"
            :key="row.broker_name"
            class="wr-row"
          >
            <div class="wr-summary" @click="toggleWinrateBroker(row.broker_name)">
              <div class="wr-broker">{{ row.broker_name }}</div>
              <div class="wr-badge" :class="winrateClass(row.win_rate_pct ?? 0)">
                {{ row.win_rate_pct != null ? row.win_rate_pct.toFixed(1) + '%' : '—' }}
              </div>
              <div class="wr-meta">
                <span>交易 {{ row.total_trades }} 次</span>
                <span>平均 <span :class="returnColorClass(row.avg_return_pct)">{{ row.avg_return_pct != null ? (row.avg_return_pct > 0 ? '+' : '') + row.avg_return_pct.toFixed(2) + '%' : '—' }}</span></span>
                <span>持倉 {{ row.avg_holding_days != null ? row.avg_holding_days.toFixed(0) + ' 日' : '—' }}</span>
                <span class="td-muted">最近 {{ row.last_entry_date?.slice(0, 10) ?? '—' }}</span>
              </div>
              <div class="wr-expand-icon">{{ expandedWinrateBroker === row.broker_name ? '▲' : '▼' }}</div>
            </div>

            <!-- Event rows -->
            <div v-if="expandedWinrateBroker === row.broker_name" class="wr-events">
              <div v-if="winrateEventsLoading" class="chart-empty" style="padding:12px 0">載入中…</div>
              <table v-else class="major-table wr-table">
                <thead>
                  <tr>
                    <th>進場日</th>
                    <th>出場日</th>
                    <th class="ra">進場收</th>
                    <th class="ra">出場收</th>
                    <th class="ra">報酬</th>
                    <th class="ra">持倉天</th>
                    <th class="ca">狀態</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="e in winrateEvents" :key="e.id">
                    <td>{{ e.entry_date?.slice(0, 10) }}</td>
                    <td>{{ e.exit_date?.slice(0, 10) ?? '—' }}</td>
                    <td class="ra">{{ e.entry_close.toFixed(2) }}</td>
                    <td class="ra">{{ e.exit_close?.toFixed(2) ?? '—' }}</td>
                    <td class="ra" :class="returnColorClass(e.return_pct ?? null)">
                      {{ e.return_pct !== null ? (e.return_pct > 0 ? '+' : '') + e.return_pct.toFixed(2) + '%' : '—' }}
                    </td>
                    <td class="ra">{{ e.holding_days ?? '—' }}</td>
                    <td class="ca">
                      <span v-if="e.is_win === true"  class="badge-win">獲利</span>
                      <span v-else-if="e.is_win === false" class="badge-loss">虧損</span>
                      <span v-else class="badge-open">持倉</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
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

/* ── Search Box ────────────────────────── */
.search-box {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  height: 34px;
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: 8px;
  transition: all 0.2s;
  min-width: 180px;
}

.search-box--focused {
  border-color: var(--blue);
  background: var(--s3);
}

.search-icon {
  color: var(--t3);
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: var(--t1);
  font-size: 13px;
  font-family: var(--font);
  font-variant-numeric: tabular-nums;
}

.search-input::placeholder {
  color: var(--t3);
}

.search-clear {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  background: transparent;
  border: none;
  color: var(--t3);
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.15s;
  flex-shrink: 0;
  padding: 0;
}

.search-clear:hover {
  color: var(--t1);
  background: var(--s1);
}

.classic-search {
  border-radius: 2px;
  height: 28px;
  min-width: 160px;
  padding: 0 8px;
}

.classic-search .search-input {
  font-size: 12px;
}

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
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px 20px;
  border-bottom: 1px solid var(--line);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.ma-group {
  display: flex;
  gap: 3px;
}

.ma-btn {
  font-family: var(--mono);
  font-size: 10.5px;
  font-weight: 600;
  padding: 3px 9px;
  background: transparent;
  color: var(--t3);
  border: 1px solid var(--line2);
  border-radius: 5px;
  cursor: pointer;
  transition: all 0.15s;
  letter-spacing: 0.02em;
}
.ma-btn:hover { color: var(--t1); border-color: var(--t2); }
.ma-btn--active { opacity: 1; }

.draw-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-family: var(--font);
  font-size: 11px;
  font-weight: 600;
  padding: 4px 11px;
  background: transparent;
  border: 1px solid var(--line2);
  border-radius: 6px;
  color: var(--t3);
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}
.draw-btn:hover { color: var(--t1); border-color: var(--t2); }
.draw-btn--active {
  background: color-mix(in oklch, var(--gold) 12%, transparent);
  border-color: var(--gold);
  color: var(--gold);
}

.chart-container--draw { cursor: cell; }

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

.tt-divider {
  height: 1px;
  background: var(--line);
  margin: 4px 0;
}

.tt-ma {
  display: flex;
  gap: 6px;
  font-size: 11px;
  font-weight: 500;
  align-items: center;
}

.tt-ma em {
  font-style: normal;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.02em;
  min-width: 32px;
  opacity: 0.9;
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

/* ── Major Panel ──────────────────────────── */
.major-panel {
  border: 1px solid var(--line);
  background: var(--s2);
  border-radius: 10px;
  overflow: hidden;
  margin-top: 20px;
}

.major-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 14px 20px 13px;
  border-bottom: 1px solid var(--line);
}

.major-topbar-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.major-body {
  display: grid;
  grid-template-columns: 1fr 1fr;
}

.major-side { overflow-x: auto; }
.major-side:first-child { border-right: 1px solid var(--line); }

.major-side-header {
  padding: 9px 20px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  border-bottom: 1px solid var(--line);
}
.buy-header  { color: var(--up); }
.sell-header { color: var(--dn); }

.major-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13.5px;
}
.major-table th {
  text-align: left;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--t3);
  padding: 9px 14px 8px 0;
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.major-table th:first-child { padding-left: 20px; }
.major-table th.ra { text-align: right; }

.major-table td {
  padding: 9px 14px 9px 0;
  border-bottom: 1px solid var(--line);
  font-variant-numeric: tabular-nums;
  vertical-align: middle;
  color: var(--t1);
}
.major-table td:first-child { padding-left: 20px; }
.major-table tr:last-child td { border-bottom: none; }
.major-table tbody tr:hover td { background: var(--s1); }

.td-rank   { font-family: var(--mono); font-size: 12px; width: 28px; }
.td-broker { font-size: 13px; }

@media (max-width: 860px) {
  .major-body { grid-template-columns: 1fr; }
  .major-side:first-child { border-right: none; border-bottom: 1px solid var(--line); }
}

/* ── Chips Pyramid Panel ────────────────────────────────────── */
.chips-panel { margin-top: 20px; }

.chips-topbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.chips-trigger-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

@keyframes chips-spin { to { transform: rotate(360deg); } }
.chips-spin { animation: chips-spin 0.9s linear infinite; flex-shrink: 0; }

.pyramid-body { padding: 16px 20px 20px; }

.pyramid-rows {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.pyramid-row {
  display: grid;
  grid-template-columns: 200px 1fr 180px;
  align-items: center;
  gap: 12px;
  min-height: 26px;
}

.pyramid-label {
  font-size: 12.5px;
  color: var(--t2);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-align: right;
  font-family: var(--mono);
}

.pyramid-bar-wrap {
  height: 18px;
  background: var(--s3);
  border-radius: 3px;
  overflow: hidden;
}

.pyramid-bar {
  height: 100%;
  border-radius: 3px;
  transition: width 0.4s ease;
  min-width: 2px;
}

.pyramid-bar--large  { background: color-mix(in oklch, var(--gold) 80%, transparent); }
.pyramid-bar--medium { background: color-mix(in oklch, var(--blue) 75%, transparent); }
.pyramid-bar--small  { background: color-mix(in oklch, var(--dn)   70%, transparent); }

.pyramid-vals {
  display: flex;
  gap: 16px;
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
  align-items: center;
}

.pyramid-pct {
  color: var(--t1);
  font-weight: 600;
  min-width: 56px;
}

.pyramid-count { font-size: 12px; }

@media (max-width: 720px) {
  .pyramid-row { grid-template-columns: 130px 1fr; }
  .pyramid-vals { display: none; }
}

/* ── Winrate Panel ─────────────────────────────────────────── */
.winrate-panel { margin-top: 24px; }
.wr-row  { border-bottom: 1px solid var(--line); }
.wr-row:last-child { border-bottom: none; }

.wr-summary {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 20px;
  cursor: pointer;
  transition: background 0.12s;
}
.wr-summary:hover { background: var(--s1); }

.wr-broker {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--t1);
  min-width: 130px;
  flex-shrink: 0;
}

.wr-badge {
  font-size: 12px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 100px;
  white-space: nowrap;
  flex-shrink: 0;
}
.wr-high { background: oklch(37% 0.14 150 / 0.25); color: oklch(62% 0.16 150); }
.wr-mid  { background: oklch(37% 0.10 82  / 0.25); color: oklch(70% 0.14 82);  }
.wr-low  { background: oklch(37% 0.14 25  / 0.25); color: oklch(62% 0.16 25);  }

.wr-meta {
  display: flex;
  gap: 16px;
  font-size: 12.5px;
  color: var(--t2);
  flex: 1;
  flex-wrap: wrap;
}

.wr-expand-icon {
  font-size: 10px;
  color: var(--t3);
  margin-left: auto;
  flex-shrink: 0;
}

.wr-events { padding: 0 20px 12px; }
.wr-table  { width: 100%; }

.badge-win, .badge-loss, .badge-open {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 100px;
}
.badge-win  { background: oklch(37% 0.14 150 / 0.25); color: oklch(62% 0.16 150); }
.badge-loss { background: oklch(37% 0.14 25  / 0.25); color: oklch(62% 0.16 25);  }
.badge-open { background: oklch(37% 0.08 240 / 0.25); color: oklch(62% 0.10 240); }

/* ── Realtime Bar ─────────────────────────── */
.rt-bar {
  display: flex; align-items: center; flex-wrap: wrap;
  gap: 0.5rem 1rem;
  padding: 8px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  margin-bottom: 0.75rem;
  font-size: 0.82rem;
  transition: border-color 0.2s;
}
.rt-bar--active { border-color: oklch(from var(--green) l c h / 0.45); }
.rt-badge {
  display: flex; align-items: center; gap: 5px;
  font-size: 0.7rem; font-weight: 600; letter-spacing: 0.04em;
  padding: 2px 8px; border-radius: 99px;
  background: var(--border); color: var(--muted);
  white-space: nowrap;
}
.rt-badge--live {
  background: oklch(from var(--green) l c h / 0.15);
  color: var(--green);
}
.rt-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--green);
  animation: rt-pulse 1.4s ease-in-out infinite;
  flex-shrink: 0;
}
@keyframes rt-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%       { opacity: 0.4; transform: scale(0.85); }
}
.rt-price {
  font-size: 1.1rem; font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.rt-delta {
  font-size: 0.82rem; font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.rt-divider {
  width: 1px; height: 18px;
  background: var(--border); flex-shrink: 0;
}
.rt-item {
  display: flex; align-items: center; gap: 5px;
  font-variant-numeric: tabular-nums;
}
.rt-key { color: var(--muted); font-size: 0.75rem; }
.rt-val { font-weight: 500; }
.rt-time {
  margin-left: auto; color: var(--muted); font-size: 0.72rem;
  font-variant-numeric: tabular-nums; white-space: nowrap;
}
.rt-loading, .rt-err { color: var(--muted); font-size: 0.78rem; }
.rt-refresh {
  background: none; border: none; cursor: pointer;
  color: var(--muted); padding: 4px;
  border-radius: 5px; display: flex; align-items: center;
  transition: color 0.15s, background 0.15s;
  margin-left: auto;
}
.rt-refresh:hover { color: var(--text); background: var(--border); }
.rt-refresh:disabled { opacity: 0.4; cursor: default; }
.spinning { animation: spin 0.8s linear infinite; display: inline-block; }
</style>
