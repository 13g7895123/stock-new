<script setup lang="ts">
import { useAppPrefs } from '~/composables/useAppPrefs'

useHead({
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700&display=swap',
    },
  ],
})

interface TechnicalResult {
  symbol: string
  name: string
  latest_close: number
  latest_date: string
  prev_week_high: number
  prev_month_high: number
  above_week_high: boolean
  above_month_high: boolean
  near_week_high: boolean
  near_month_high: boolean
}

type TagKey = 'above_week' | 'above_month' | 'near_week' | 'near_month'

interface Tag {
  key: TagKey
  label: string
  desc: string
  color: 'green' | 'blue' | 'amber' | 'purple'
}

const tags: Tag[] = [
  { key: 'above_week',  label: '高於上周高點',     desc: '收盤價已突破前 5 根 K 棒最高收盤',          color: 'green'  },
  { key: 'above_month', label: '高於上月高點',     desc: '收盤價已突破前 20 根 K 棒最高收盤',         color: 'blue'   },
  { key: 'near_week',   label: '即將高於上周高點', desc: '收盤 × 1.1 ≥ 前 5 根最高收盤，尚未突破',   color: 'amber'  },
  { key: 'near_month',  label: '即將高於上月高點', desc: '收盤 × 1.1 ≥ 前 20 根最高收盤，尚未突破',  color: 'purple' },
]

const { data: screener, pending, error } = await useFetch<TechnicalResult[]>('/api/technical/screener')
const selectedTag = ref<TagKey | null>(null)
const searchQuery = ref('')

const { isDark, isClassic, toggleTheme, setTheme, setStyle } = useAppPrefs()
const settingsOpen = ref(false)

const today = new Date().toLocaleDateString('zh-TW', {
  year: 'numeric', month: 'long', day: 'numeric', weekday: 'long',
})

const allStocks = computed<TechnicalResult[]>(() =>
  Array.isArray(screener.value) ? screener.value : []
)

const tagCounts = computed(() => {
  const s = allStocks.value
  return {
    above_week:  s.filter(r => r.above_week_high).length,
    above_month: s.filter(r => r.above_month_high).length,
    near_week:   s.filter(r => r.near_week_high).length,
    near_month:  s.filter(r => r.near_month_high).length,
  }
})

const filtered = computed<TechnicalResult[]>(() => {
  let list = allStocks.value

  if (selectedTag.value === 'above_week')  list = list.filter(r => r.above_week_high)
  else if (selectedTag.value === 'above_month') list = list.filter(r => r.above_month_high)
  else if (selectedTag.value === 'near_week')   list = list.filter(r => r.near_week_high)
  else if (selectedTag.value === 'near_month')  list = list.filter(r => r.near_month_high)

  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(r =>
      r.symbol.toLowerCase().startsWith(q) || r.name.toLowerCase().includes(q)
    )
  }

  return list
})

function toggleTag(key: TagKey) {
  selectedTag.value = selectedTag.value === key ? null : key
}

function fmtPrice(v: number) {
  if (!v) return '—'
  return v.toFixed(2)
}

function diffPct(close: number, high: number): string {
  if (!high || !close) return '—'
  const pct = ((close - high) / high) * 100
  return (pct >= 0 ? '+' : '') + pct.toFixed(2) + '%'
}

function diffClass(close: number, high: number): string {
  if (!high || !close) return ''
  return close >= high ? 'diff-pos' : 'diff-neg'
}
</script>

<template>
  <div class="page" :class="{ light: !isDark, classic: isClassic }">

    <!-- ══ Bento Header ══ -->
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
          <span class="brand-cur">技術分析篩選</span>
        </div>
        <div class="header-right">
          <span class="header-date">{{ today }}</span>
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
              <div class="sp-group">
                <p class="sp-label">版面風格</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: isClassic }" @click="setStyle('classic')">Classic</button>
                  <button class="sp-btn" :class="{ active: !isClassic }" @click="setStyle('bento')">Bento</button>
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
          <span class="classic-badge">TSM</span>
          <div class="classic-brand-text">
            <span class="classic-brand-sub">Taiwan Stock Monitor</span>
            <span class="classic-brand-name">技術分析篩選</span>
          </div>
        </div>
        <div class="classic-header-right">
          <span class="classic-date">{{ today }}</span>
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
                  <button class="sp-btn" :class="{ active: !isClassic }" @click="setStyle('bento')">Bento</button>
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

    <!-- ══ Toolbar ══ -->
    <div class="toolbar">
      <div class="toolbar__inner">
        <div class="toolbar__left">
          <h1 class="page-title">技術分析篩選</h1>
          <span class="stock-count">{{ filtered.length.toLocaleString() }} / {{ allStocks.length.toLocaleString() }}</span>
        </div>
        <div class="toolbar__right">
          <input
            v-model="searchQuery"
            class="search-input"
            type="text"
            placeholder="搜尋代號或名稱…"
          />
        </div>
      </div>
    </div>

    <!-- ══ Main Layout ══ -->
    <div class="layout">

      <!-- ── Tag Sidebar ── -->
      <aside class="sidebar">
        <section class="tag-section">
          <div class="tag-heading">
            <span>篩選條件</span>
            <button v-if="selectedTag" class="clear-btn" @click="selectedTag = null">清除</button>
          </div>
          <ul class="tag-list">
            <li>
              <button
                class="tag-item tag-item--all"
                :class="{ active: !selectedTag }"
                @click="selectedTag = null"
              >
                <span class="tag-label">全部股票</span>
                <span class="tag-count">{{ allStocks.length }}</span>
              </button>
            </li>
            <li v-for="tag in tags" :key="tag.key">
              <button
                class="tag-item"
                :class="[`tag-item--${tag.color}`, { active: selectedTag === tag.key }]"
                @click="toggleTag(tag.key)"
              >
                <span class="tag-label">{{ tag.label }}</span>
                <span class="tag-count">{{ tagCounts[tag.key] }}</span>
              </button>
            </li>
          </ul>
          <div v-if="selectedTag" class="tag-desc">
            {{ tags.find(t => t.key === selectedTag)?.desc }}
          </div>
        </section>
      </aside>

      <!-- ── Stock Table ── -->
      <main class="content">
        <div v-if="pending" class="loading-state">
          <span class="loading-spin">◌</span>
          <span>載入中…</span>
        </div>
        <div v-else-if="error" class="error-state">
          ⚠ 載入失敗，請確認後端服務是否正常
        </div>
        <div v-else-if="filtered.length === 0" class="empty-state">
          <span class="empty-icon">—</span>
          <span>{{ searchQuery ? '無符合搜尋條件的股票' : '目前無符合此篩選條件的股票' }}</span>
        </div>
        <div v-else class="table-wrap">
          <table class="stock-table">
            <thead>
              <tr>
                <th class="th-sym">代號</th>
                <th class="th-name">名稱</th>
                <th class="th-close">收盤</th>
                <th class="th-week">上週高點</th>
                <th class="th-diff">vs 上週</th>
                <th class="th-month">上月高點</th>
                <th class="th-diff">vs 上月</th>
                <th class="th-tags">訊號</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="stock in filtered"
                :key="stock.symbol"
                class="stock-row"
                @click="navigateTo(`/stocks/${stock.symbol}`)"
              >
                <td class="td-sym">{{ stock.symbol }}</td>
                <td class="td-name">{{ stock.name }}</td>
                <td class="td-close">{{ fmtPrice(stock.latest_close) }}</td>
                <td class="td-week">{{ fmtPrice(stock.prev_week_high) }}</td>
                <td class="td-diff" :class="diffClass(stock.latest_close, stock.prev_week_high)">
                  {{ diffPct(stock.latest_close, stock.prev_week_high) }}
                </td>
                <td class="td-month">{{ fmtPrice(stock.prev_month_high) }}</td>
                <td class="td-diff" :class="diffClass(stock.latest_close, stock.prev_month_high)">
                  {{ diffPct(stock.latest_close, stock.prev_month_high) }}
                </td>
                <td class="td-tags">
                  <span v-if="stock.above_week_high"  class="signal signal--green">↑週</span>
                  <span v-if="stock.above_month_high" class="signal signal--blue">↑月</span>
                  <span v-if="stock.near_week_high"   class="signal signal--amber">≈週</span>
                  <span v-if="stock.near_month_high"  class="signal signal--purple">≈月</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
/* ── Variables ──────────────────────────────── */
.page {
  --bg:      oklch(9.5%  0.018 256);
  --surface: oklch(14%   0.018 256);
  --border:  oklch(22%   0.016 256);
  --text:    oklch(92%   0.010 256);
  --muted:   oklch(55%   0.010 256);
  --blue:    oklch(63%   0.20  264);
  --gold:    oklch(76%   0.13  82);
  --green:   oklch(68%   0.18  145);
  --amber:   oklch(75%   0.16  70);
  --purple:  oklch(68%   0.18  295);
  --red:     oklch(62%   0.19  27);
  min-height: 100dvh;
  background: var(--bg);
  color: var(--text);
  font-family: 'DM Sans', sans-serif;
}
.page.light {
  --bg:      oklch(96.5% 0.009 220);
  --surface: oklch(99%   0.003 220);
  --border:  oklch(88%   0.007 220);
  --text:    oklch(15%   0.015 256);
  --muted:   oklch(48%   0.010 256);
  --blue:    oklch(47%   0.21  264);
  --gold:    oklch(52%   0.16  72);
  --green:   oklch(46%   0.17  145);
  --amber:   oklch(52%   0.17  70);
  --purple:  oklch(50%   0.18  295);
  --red:     oklch(48%   0.19  27);
}

/* ── Header ─────────────────────────────────── */
.site-header {
  border-bottom: 1px solid var(--border);
  padding: 0 1.5rem;
}
.site-header__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 52px;
  max-width: 1400px;
  margin: 0 auto;
}
.brand { display: flex; align-items: center; gap: 0.5rem; font-size: 0.85rem; }
.back-link {
  display: flex; align-items: center; gap: 0.3rem;
  color: var(--muted); text-decoration: none;
  transition: color 0.15s;
}
.back-link:hover { color: var(--text); }
.brand-sep { color: var(--border); }
.brand-cur { color: var(--text); font-weight: 500; }
.header-right { display: flex; align-items: center; gap: 0.75rem; }
.header-date { font-size: 0.78rem; color: var(--muted); }
.btn-icon {
  background: none; border: none; cursor: pointer;
  color: var(--muted); padding: 4px;
  border-radius: 6px; display: flex; align-items: center;
  transition: color 0.15s, background 0.15s;
}
.btn-icon:hover { color: var(--text); background: var(--border); }

/* ── Classic Header ──────────────────────────── */
.classic-header {
  border-bottom: 2px solid var(--border);
  background: var(--surface);
  padding: 0 1.5rem;
}
.classic-header__inner {
  display: flex; align-items: center; justify-content: space-between;
  height: 56px; max-width: 1400px; margin: 0 auto;
}
.classic-brand { display: flex; align-items: center; gap: 0.75rem; }
.classic-back {
  color: var(--muted); text-decoration: none; font-size: 0.85rem;
  transition: color 0.15s;
}
.classic-back:hover { color: var(--text); }
.classic-sep { color: var(--border); }
.classic-badge {
  font-size: 0.7rem; font-weight: 700; letter-spacing: 0.1em;
  color: var(--blue); border: 1px solid var(--blue);
  padding: 2px 6px; border-radius: 4px;
}
.classic-brand-text { display: flex; flex-direction: column; }
.classic-brand-sub { font-size: 0.65rem; color: var(--muted); letter-spacing: 0.05em; }
.classic-brand-name { font-size: 0.9rem; font-weight: 600; line-height: 1.1; }
.classic-header-right { display: flex; align-items: center; gap: 0.75rem; }
.classic-date { font-size: 0.78rem; color: var(--muted); }
.classic-settings-btn {
  background: none; border: 1px solid var(--border); cursor: pointer;
  color: var(--muted); padding: 3px 8px; border-radius: 4px;
  font-size: 0.85rem; transition: color 0.15s, border-color 0.15s;
}
.classic-settings-btn:hover { color: var(--text); border-color: var(--muted); }
.classic-toggle-btn {
  background: none; border: 1px solid var(--border); cursor: pointer;
  color: var(--muted); padding: 3px 8px; border-radius: 4px;
  font-size: 0.9rem; transition: color 0.15s;
}
.classic-toggle-btn:hover { color: var(--text); }

/* ── Settings Panel ──────────────────────────── */
.settings-wrap { position: relative; }
.settings-overlay {
  position: fixed; inset: 0; z-index: 10;
}
.settings-panel {
  position: absolute; top: calc(100% + 8px); right: 0;
  background: var(--surface); border: 1px solid var(--border);
  border-radius: 10px; padding: 14px 16px; width: 200px;
  z-index: 20; box-shadow: 0 8px 24px oklch(0% 0 0 / 0.3);
}
.sp-title { font-size: 0.75rem; color: var(--muted); margin: 0 0 10px; font-weight: 600; letter-spacing: 0.05em; }
.sp-group { margin-bottom: 12px; }
.sp-group:last-child { margin-bottom: 0; }
.sp-label { font-size: 0.72rem; color: var(--muted); margin: 0 0 6px; }
.sp-btns { display: flex; gap: 6px; }
.sp-btn {
  flex: 1; padding: 5px 8px; border-radius: 6px;
  border: 1px solid var(--border); background: none;
  color: var(--muted); font-size: 0.78rem; cursor: pointer;
  font-family: inherit; transition: all 0.15s;
  display: flex; align-items: center; justify-content: center; gap: 4px;
}
.sp-btn:hover { border-color: var(--text); color: var(--text); }
.sp-btn.active { border-color: var(--blue); color: var(--blue); background: oklch(from var(--blue) l c h / 0.1); }

/* ── Toolbar ──────────────────────────────────── */
.toolbar {
  border-bottom: 1px solid var(--border);
  padding: 0 1.5rem;
}
.toolbar__inner {
  display: flex; align-items: center; justify-content: space-between;
  height: 52px; max-width: 1400px; margin: 0 auto;
  gap: 1rem;
}
.toolbar__left { display: flex; align-items: center; gap: 0.75rem; }
.page-title { font-size: 1rem; font-weight: 600; margin: 0; }
.stock-count { font-size: 0.78rem; color: var(--muted); }
.toolbar__right { display: flex; align-items: center; }
.search-input {
  height: 32px; padding: 0 10px; border-radius: 7px;
  border: 1px solid var(--border); background: var(--surface);
  color: var(--text); font-size: 0.82rem; font-family: inherit;
  outline: none; width: 200px; transition: border-color 0.15s;
}
.search-input:focus { border-color: var(--blue); }
.search-input::placeholder { color: var(--muted); }

/* ── Layout ────────────────────────────────────── */
.layout {
  display: flex;
  max-width: 1400px;
  margin: 0 auto;
  padding: 1.5rem;
  gap: 1.5rem;
  align-items: flex-start;
}

/* ── Sidebar ────────────────────────────────────── */
.sidebar {
  width: 220px;
  flex-shrink: 0;
  position: sticky;
  top: 1.5rem;
}
.tag-section {}
.tag-heading {
  display: flex; align-items: center; justify-content: space-between;
  font-size: 0.72rem; color: var(--muted); font-weight: 600;
  letter-spacing: 0.05em; text-transform: uppercase;
  padding: 0 0 8px;
}
.clear-btn {
  background: none; border: none; cursor: pointer;
  color: var(--blue); font-size: 0.72rem; padding: 0;
  font-family: inherit; transition: opacity 0.15s;
}
.clear-btn:hover { opacity: 0.7; }
.tag-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 3px; }
.tag-item {
  width: 100%; display: flex; align-items: center; justify-content: space-between;
  padding: 8px 10px; border-radius: 8px;
  border: 1px solid transparent; background: none;
  color: var(--muted); font-size: 0.84rem; font-family: inherit;
  cursor: pointer; text-align: left; transition: all 0.15s;
}
.tag-item:hover { color: var(--text); background: var(--surface); border-color: var(--border); }
.tag-item.active { color: var(--text); background: var(--surface); border-color: var(--border); }
.tag-item--all.active { border-color: var(--blue); color: var(--blue); }
.tag-item--green.active { border-color: var(--green); color: var(--green); background: oklch(from var(--green) l c h / 0.08); }
.tag-item--blue.active  { border-color: var(--blue);  color: var(--blue);  background: oklch(from var(--blue)  l c h / 0.08); }
.tag-item--amber.active { border-color: var(--amber); color: var(--amber); background: oklch(from var(--amber) l c h / 0.08); }
.tag-item--purple.active{ border-color: var(--purple);color: var(--purple);background: oklch(from var(--purple) l c h / 0.08); }
.tag-label { font-weight: 500; }
.tag-count {
  font-size: 0.72rem; padding: 1px 6px;
  border-radius: 99px; background: var(--border);
  color: var(--muted); font-variant-numeric: tabular-nums;
}
.tag-desc {
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--surface);
  border: 1px solid var(--border);
  font-size: 0.78rem;
  color: var(--muted);
  line-height: 1.5;
}

/* ── Content ────────────────────────────────────── */
.content { flex: 1; min-width: 0; }

.loading-state,
.error-state,
.empty-state {
  display: flex; align-items: center; justify-content: center;
  gap: 0.5rem; padding: 3rem;
  color: var(--muted); font-size: 0.9rem;
}
.loading-spin {
  animation: spin 1s linear infinite;
  display: inline-block;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* ── Table ──────────────────────────────────────── */
.table-wrap {
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
  overflow-x: auto;
}
.stock-table {
  width: 100%; border-collapse: collapse;
  font-size: 0.84rem;
}
.stock-table th {
  padding: 9px 12px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  color: var(--muted);
  font-weight: 500;
  text-align: right;
  white-space: nowrap;
}
.stock-table th.th-sym,
.stock-table th.th-name { text-align: left; }
.stock-table th.th-tags { text-align: center; }

.stock-row {
  cursor: pointer;
  border-bottom: 1px solid var(--border);
  transition: background 0.1s;
}
.stock-row:last-child { border-bottom: none; }
.stock-row:hover { background: var(--surface); }

.stock-table td {
  padding: 8px 12px;
  text-align: right;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.td-sym { text-align: left; font-weight: 600; color: var(--blue); font-size: 0.9rem; }
.td-name { text-align: left; color: var(--text); max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
.td-close { font-weight: 500; }

.diff-pos { color: var(--green); }
.diff-neg { color: var(--red); }

/* ── Signal badges ───────────────────────────── */
.td-tags { text-align: center; }
.signal {
  display: inline-block; font-size: 0.7rem; font-weight: 600;
  padding: 2px 6px; border-radius: 5px; margin: 1px 2px;
  white-space: nowrap;
}
.signal--green  { background: oklch(from var(--green)  l c h / 0.18); color: var(--green);  border: 1px solid oklch(from var(--green)  l c h / 0.35); }
.signal--blue   { background: oklch(from var(--blue)   l c h / 0.18); color: var(--blue);   border: 1px solid oklch(from var(--blue)   l c h / 0.35); }
.signal--amber  { background: oklch(from var(--amber)  l c h / 0.18); color: var(--amber);  border: 1px solid oklch(from var(--amber)  l c h / 0.35); }
.signal--purple { background: oklch(from var(--purple) l c h / 0.18); color: var(--purple); border: 1px solid oklch(from var(--purple) l c h / 0.35); }

/* ── Responsive ──────────────────────────────── */
@media (max-width: 700px) {
  .layout { flex-direction: column; padding: 1rem; }
  .sidebar { width: 100%; position: static; }
  .tag-list { flex-direction: row; flex-wrap: wrap; }
  .tag-item { width: auto; }
}
</style>
