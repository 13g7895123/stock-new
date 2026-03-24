<template>
  <div class="page" :class="{ light: !isDark }">
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="" />
    <link href="https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700&display=swap" rel="stylesheet" />

    <!-- ══ Header ══ -->
    <header class="site-header">
      <div class="site-header__inner">
        <nav class="header-left">
          <NuxtLink to="/" class="back-link">← 首頁</NuxtLink>
          <div class="brand">
            <span class="brand-badge">TSM</span>
            <div class="brand-text">
              <span class="brand-sub">Stock Monitor</span>
              <span class="brand-name">Stock Monitor</span>
            </div>
          </div>
        </nav>
        <div class="header-right">
          <span class="header-date">{{ today }}</span>
          <button class="theme-toggle" @click="isDark = !isDark; saveTheme()" :title="isDark ? '切換淺色' : '切換深色'">
            {{ isDark ? '○' : '●' }}
          </button>
        </div>
      </div>
    </header>

    <!-- ══ Toolbar ══ -->
    <div class="toolbar">
      <div class="toolbar__inner">
        <div class="toolbar__left">
          <h1 class="page-title">股票列表</h1>
          <span class="stock-count">{{ filteredStocks.length.toLocaleString() }} / {{ stocks.length.toLocaleString() }}</span>
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

    <!-- ══ Layout ══ -->
    <div class="layout">

      <!-- Sidebar -->
      <aside class="sidebar">

        <!-- Industry Filter -->
        <section class="filter-section">
          <div class="filter-heading">
            <span>族群</span>
            <button v-if="selectedIndustry" class="clear-btn" @click="selectedIndustry = ''">清除</button>
          </div>
          <div v-if="industriesLoading" class="filter-loading">載入中…</div>
          <ul v-else class="filter-list">
            <li>
              <button
                class="filter-item"
                :class="{ active: selectedIndustry === '' }"
                @click="selectedIndustry = ''"
              >
                全部族群
              </button>
            </li>
            <li v-for="industry in industries" :key="industry">
              <button
                class="filter-item"
                :class="{ active: selectedIndustry === industry }"
                @click="selectedIndustry = industry"
              >
                {{ industry }}
              </button>
            </li>
          </ul>
        </section>

        <!-- Tag Filter + Management -->
        <section class="filter-section">
          <div class="filter-heading">
            <span>標籤</span>
            <button v-if="selectedTagId" class="clear-btn" @click="selectedTagId = 0">清除</button>
          </div>
          <div v-if="tagsLoading" class="filter-loading">載入中…</div>
          <ul v-else class="filter-list">
            <li>
              <button
                class="filter-item"
                :class="{ active: selectedTagId === 0 }"
                @click="selectedTagId = 0"
              >
                全部標籤
              </button>
            </li>
            <li v-for="tag in tags" :key="tag.id" class="tag-row">
              <button
                class="filter-item tag-item"
                :class="{ active: selectedTagId === tag.id }"
                @click="selectedTagId = tag.id"
              >
                <span class="tag-dot" :style="{ background: tag.color }"></span>
                {{ tag.name }}
              </button>
              <button class="tag-del-btn" title="刪除標籤" @click.stop="deleteTag(tag.id)">×</button>
            </li>
          </ul>

          <!-- Create Tag -->
          <form class="tag-create" @submit.prevent="createTag">
            <input
              v-model="newTagName"
              class="tag-name-input"
              type="text"
              placeholder="新增標籤…"
              maxlength="40"
            />
            <input v-model="newTagColor" class="tag-color-input" type="color" title="選擇顏色" />
            <button type="submit" class="tag-add-btn" :disabled="!newTagName.trim()">+</button>
          </form>
          <p v-if="tagError" class="tag-err">{{ tagError }}</p>
        </section>

      </aside>

      <!-- Main Table -->
      <main class="main-content">

        <div v-if="tableLoading" class="table-empty">
          <span class="spin-icon">◌</span> 載入中…
        </div>

        <table v-else-if="filteredStocks.length > 0" class="stock-table">
          <thead>
            <tr>
              <th>代號</th>
              <th>名稱</th>
              <th>族群</th>
              <th class="ra">股價</th>
              <th class="ra">漲跌</th>
              <th class="ra">漲跌幅</th>
              <th class="ra">成交量</th>
              <th>標籤</th>
              <th class="ca">K 線</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="stock in filteredStocks" :key="stock.id" :class="{ 'row--tag-open': expandedTagRow === stock.symbol }">
              <td class="td-sym">
                <NuxtLink :to="`/stocks/${stock.symbol}`">{{ stock.symbol }}</NuxtLink>
              </td>
              <td class="td-name">{{ stock.name }}</td>
              <td class="td-industry">
                <span v-if="stock.industry" class="industry-pill">{{ stock.industry }}</span>
                <span v-else class="t3">—</span>
              </td>
              <td class="ra td-price">{{ stock.price > 0 ? stock.price.toFixed(2) : '—' }}</td>
              <td class="ra" :class="colorClass(stock.change)">
                {{ stock.price > 0 ? (stock.change > 0 ? '+' : '') + stock.change.toFixed(2) : '—' }}
              </td>
              <td class="ra" :class="colorClass(stock.change_pct)">
                {{ stock.price > 0 ? (stock.change_pct > 0 ? '+' : '') + stock.change_pct.toFixed(2) + '%' : '—' }}
              </td>
              <td class="ra td-vol">{{ stock.volume > 0 ? stock.volume.toLocaleString() : '—' }}</td>
              <td class="td-tags">
                <div class="tags-cell">
                  <span
                    v-for="tag in (stock.tags ?? [])"
                    :key="tag.id"
                    class="tag-chip"
                    :style="{ borderColor: tag.color, color: tag.color }"
                  >{{ tag.name }}</span>
                  <button
                    class="tag-assign-btn"
                    title="管理標籤"
                    @click="toggleTagRow(stock.symbol)"
                  >＋</button>
                </div>
                <!-- Tag Assign Popover -->
                <div v-if="expandedTagRow === stock.symbol" class="tag-popover">
                  <p class="tag-popover-head">選擇標籤</p>
                  <label
                    v-for="tag in tags"
                    :key="tag.id"
                    class="tag-popover-item"
                  >
                    <input
                      type="checkbox"
                      :checked="stockHasTag(stock, tag.id)"
                      @change="toggleStockTag(stock, tag.id)"
                    />
                    <span class="tag-dot" :style="{ background: tag.color }"></span>
                    {{ tag.name }}
                  </label>
                  <p v-if="tags.length === 0" class="tag-popover-empty">尚無標籤</p>
                </div>
              </td>
              <td class="ca">
                <NuxtLink :to="`/stocks/${stock.symbol}`" class="row-link">K 線</NuxtLink>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-else-if="!tableLoading" class="table-empty">
          {{ searchQuery || selectedIndustry || selectedTagId ? '查無符合條件的股票' : '尚無資料。請先同步股票清單。' }}
        </div>

      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Tag {
  id: number
  name: string
  color: string
}

interface Stock {
  id: number
  symbol: string
  name: string
  industry: string
  market: string
  price: number
  change: number
  change_pct: number
  volume: number
  updated_at: string
  tags: Tag[]
}

// ── Theme ──────────────────────────────────────────────────────
const isDark = ref(true)
onMounted(() => {
  const saved = localStorage.getItem('tsm-theme')
  if (saved !== null) isDark.value = saved === 'dark'
  else isDark.value = false
})
function saveTheme() {
  localStorage.setItem('tsm-theme', isDark.value ? 'dark' : 'light')
}

// ── Date ───────────────────────────────────────────────────────
const today = new Date().toLocaleDateString('zh-TW', {
  year: 'numeric', month: 'long', day: 'numeric', weekday: 'short'
})

// ── Filters ────────────────────────────────────────────────────
const searchQuery    = ref('')
const selectedIndustry = ref('')
const selectedTagId  = ref(0)

// ── Industry List ──────────────────────────────────────────────
const industries = ref<string[]>([])
const industriesLoading = ref(true)

async function fetchIndustries() {
  try {
    const res = await $fetch<string[]>('/api/industries')
    industries.value = (Array.isArray(res) ? res : []).filter(Boolean).sort()
  } catch {
    industries.value = []
  } finally {
    industriesLoading.value = false
  }
}

// ── Tags ───────────────────────────────────────────────────────
const tags        = ref<Tag[]>([])
const tagsLoading = ref(true)
const tagError    = ref('')
const newTagName  = ref('')
const newTagColor = ref('#6b7280')

async function fetchTags() {
  try {
    const res = await $fetch<Tag[]>('/api/tags')
    tags.value = Array.isArray(res) ? res : []
  } catch {
    tags.value = []
  } finally {
    tagsLoading.value = false
  }
}

async function createTag() {
  const name = newTagName.value.trim()
  if (!name) return
  tagError.value = ''
  try {
    await $fetch('/api/tags', {
      method: 'POST',
      body: { name, color: newTagColor.value }
    })
    newTagName.value = ''
    newTagColor.value = '#6b7280'
    await fetchTags()
  } catch {
    tagError.value = '建立標籤失敗'
  }
}

async function deleteTag(id: number) {
  tagError.value = ''
  try {
    await $fetch(`/api/tags/${id}`, { method: 'DELETE' })
    if (selectedTagId.value === id) selectedTagId.value = 0
    await Promise.all([fetchTags(), fetchStocks()])
  } catch {
    tagError.value = '刪除失敗'
  }
}

// ── Stocks ─────────────────────────────────────────────────────
const stocks       = ref<Stock[]>([])
const tableLoading = ref(true)

async function fetchStocks() {
  tableLoading.value = true
  try {
    const params: Record<string, string> = {}
    if (selectedIndustry.value) params.industry  = selectedIndustry.value
    if (selectedTagId.value)    params.tag_id    = String(selectedTagId.value)
    const res = await $fetch<Stock[]>('/api/stocks', { query: params })
    stocks.value = Array.isArray(res) ? res : []
  } catch {
    stocks.value = []
  } finally {
    tableLoading.value = false
  }
}

const filteredStocks = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return stocks.value
  return stocks.value.filter(s =>
    s.symbol.toLowerCase().includes(q) ||
    s.name.toLowerCase().includes(q) ||
    (s.industry ?? '').toLowerCase().includes(q)
  )
})

// Re-fetch when server-side filters change
watch([selectedIndustry, selectedTagId], () => fetchStocks())

// ── Tag Assignment ─────────────────────────────────────────────
const expandedTagRow = ref('')

function toggleTagRow(symbol: string) {
  expandedTagRow.value = expandedTagRow.value === symbol ? '' : symbol
}

function stockHasTag(stock: Stock, tagId: number): boolean {
  return (stock.tags ?? []).some(t => t.id === tagId)
}

async function toggleStockTag(stock: Stock, tagId: number) {
  const current = (stock.tags ?? []).map(t => t.id)
  const next = stockHasTag(stock, tagId)
    ? current.filter(id => id !== tagId)
    : [...current, tagId]
  try {
    await $fetch(`/api/stocks/${stock.symbol}/tags`, {
      method: 'PUT',
      body: { tag_ids: next }
    })
    // Optimistic update
    const tag = tags.value.find(t => t.id === tagId)
    if (stockHasTag(stock, tagId)) {
      stock.tags = (stock.tags ?? []).filter(t => t.id !== tagId)
    } else if (tag) {
      stock.tags = [...(stock.tags ?? []), tag]
    }
  } catch {
    tagError.value = '更新標籤失敗'
  }
}

// ── Color helpers ──────────────────────────────────────────────
function colorClass(val: number) {
  if (val > 0) return 'col-up'
  if (val < 0) return 'col-dn'
  return 'col-flat'
}

// ── Init ───────────────────────────────────────────────────────
onMounted(async () => {
  await Promise.all([fetchIndustries(), fetchTags(), fetchStocks()])
})
</script>

<style scoped>
/* ── Design Tokens ─────────────────────────────────────────── */
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

/* ── Light Mode ────────────────────────────────────────────── */
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
}

/* ── Header ────────────────────────────────────────────────── */
.site-header {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}
.site-header__inner {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 40px;
  height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 20px;
}
.back-link {
  font-size: 13px;
  color: var(--t3);
  text-decoration: none;
  letter-spacing: 0.03em;
  transition: color 0.15s;
}
.back-link:hover { color: var(--gold); }
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
}
.brand-text { display: flex; flex-direction: column; gap: 2px; }
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
.header-right { display: flex; align-items: center; gap: 24px; }
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
}
.theme-toggle:hover { border-color: var(--gold); color: var(--gold); }

/* ── Toolbar ───────────────────────────────────────────────── */
.toolbar {
  background: var(--s2);
  border-bottom: 1px solid var(--line);
}
.toolbar__inner {
  max-width: 1400px;
  margin: 0 auto;
  padding: 14px 40px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.toolbar__left { display: flex; align-items: baseline; gap: 12px; }
.page-title {
  font-size: 19px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.stock-count {
  font-size: 13px;
  color: var(--t3);
  font-variant-numeric: tabular-nums;
}
.search-input {
  width: 260px;
  padding: 9px 13px;
  font-size: 14.5px;
  font-family: var(--font);
  background: var(--s1);
  border: 1px solid var(--line2);
  outline: none;
  color: var(--t1);
  transition: border-color 0.15s;
}
.search-input:focus { border-color: var(--t1); }
.search-input::placeholder { color: var(--t3); }

/* ── Layout ────────────────────────────────────────────────── */
.layout {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 40px 60px;
  display: grid;
  grid-template-columns: 220px 1fr;
  gap: 0;
  align-items: start;
}

/* ── Sidebar ───────────────────────────────────────────────── */
.sidebar {
  padding: 28px 24px 28px 0;
  border-right: 1px solid var(--line);
  position: sticky;
  top: 54px;
  max-height: calc(100vh - 54px);
  overflow-y: auto;
}

.filter-section {
  margin-bottom: 28px;
}

.filter-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 10px;
}

.clear-btn {
  background: none;
  border: none;
  color: var(--gold);
  font-size: 11px;
  cursor: pointer;
  padding: 0;
  font-family: var(--font);
}
.clear-btn:hover { opacity: 0.7; }

.filter-loading {
  font-size: 12.5px;
  color: var(--t3);
}

.filter-list {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.filter-item {
  width: 100%;
  text-align: left;
  background: none;
  border: none;
  color: var(--t2);
  font-size: 13.5px;
  font-family: var(--font);
  padding: 6px 10px;
  cursor: pointer;
  border-left: 2px solid transparent;
  transition: color 0.12s, border-color 0.12s;
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.filter-item:hover { color: var(--t1); }
.filter-item.active {
  color: var(--gold);
  border-left-color: var(--gold);
  font-weight: 600;
}

.tag-row {
  display: flex;
  align-items: center;
  gap: 2px;
}
.tag-row .filter-item { flex: 1; min-width: 0; }
.tag-item { gap: 6px; }
.tag-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.tag-del-btn {
  background: none;
  border: none;
  color: var(--t3);
  font-size: 14px;
  cursor: pointer;
  padding: 4px 6px;
  line-height: 1;
  font-family: var(--font);
  flex-shrink: 0;
}
.tag-del-btn:hover { color: var(--up); }

/* Tag Create Form */
.tag-create {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
}
.tag-name-input {
  flex: 1;
  padding: 6px 9px;
  font-size: 13px;
  font-family: var(--font);
  background: var(--s1);
  border: 1px solid var(--line2);
  outline: none;
  color: var(--t1);
  transition: border-color 0.15s;
  min-width: 0;
}
.tag-name-input:focus { border-color: var(--gold); }
.tag-name-input::placeholder { color: var(--t3); }

.tag-color-input {
  width: 28px;
  height: 28px;
  border: 1px solid var(--line2);
  background: none;
  padding: 2px;
  cursor: pointer;
  flex-shrink: 0;
}

.tag-add-btn {
  width: 28px;
  height: 28px;
  background: var(--gold);
  border: none;
  color: var(--bg);
  font-size: 16px;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-family: var(--font);
  transition: opacity 0.15s;
}
.tag-add-btn:disabled { opacity: 0.4; cursor: default; }
.tag-add-btn:not(:disabled):hover { opacity: 0.8; }

.tag-err {
  font-size: 12px;
  color: var(--up);
  margin-top: 6px;
}

/* ── Main Content ──────────────────────────────────────────── */
.main-content {
  padding: 28px 0 0 32px;
  min-width: 0;
}

/* ── Table ─────────────────────────────────────────────────── */
.stock-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14.5px;
}
.stock-table th {
  text-align: left;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.10em;
  text-transform: uppercase;
  color: var(--t3);
  padding: 12px 10px 10px 0;
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.stock-table th.ra { text-align: right; }
.stock-table th.ca { text-align: center; }

.stock-table td {
  padding: 11px 10px 11px 0;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}
.stock-table tr:last-child td { border-bottom: none; }
.stock-table tbody tr:hover td { background: var(--s1); }
.stock-table tbody .row--tag-open td { background: var(--s1); }

.ra  { text-align: right; font-variant-numeric: tabular-nums; }
.ca  { text-align: center; }
.t3  { color: var(--t3); }

.td-sym a {
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--gold);
  text-decoration: none;
  letter-spacing: 0.02em;
}
.td-sym a:hover { color: var(--t1); }
.td-name { color: var(--t2); max-width: 120px; }
.td-price { font-weight: 600; }
.td-vol { color: var(--t3); }

.industry-pill {
  font-size: 12px;
  color: var(--t2);
  background: var(--s1);
  border: 1px solid var(--line);
  padding: 2px 7px;
  white-space: nowrap;
  max-width: 110px;
  display: inline-block;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: middle;
}

.col-up   { color: var(--up);  font-weight: 500; }
.col-dn   { color: var(--dn);  font-weight: 500; }
.col-flat { color: var(--t3); }

/* Tags Cell */
.td-tags { position: relative; min-width: 110px; }
.tags-cell {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}
.tag-chip {
  font-size: 11.5px;
  font-weight: 600;
  padding: 2px 7px;
  border: 1px solid;
  white-space: nowrap;
  line-height: 1.4;
}
.tag-assign-btn {
  background: none;
  border: 1px solid var(--line2);
  color: var(--t3);
  font-size: 13px;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-family: var(--font);
  padding: 0;
  transition: border-color 0.12s, color 0.12s;
  flex-shrink: 0;
  line-height: 1;
}
.tag-assign-btn:hover { border-color: var(--gold); color: var(--gold); }

/* Tag Popover */
.tag-popover {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  z-index: 100;
  background: var(--s2);
  border: 1px solid var(--line2);
  padding: 10px 12px;
  min-width: 160px;
  box-shadow: 0 8px 24px oklch(0% 0 0 / 0.3);
}
.tag-popover-head {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.10em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 8px;
}
.tag-popover-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--t2);
  cursor: pointer;
  padding: 5px 0;
  user-select: none;
}
.tag-popover-item:hover { color: var(--t1); }
.tag-popover-item input { cursor: pointer; }
.tag-popover-empty { font-size: 12.5px; color: var(--t3); }

.row-link {
  font-size: 12.5px;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: var(--t3);
  text-decoration: none;
  padding: 3px 10px;
  border: 1px solid var(--line);
  transition: border-color 0.15s, color 0.15s;
}
.row-link:hover { border-color: var(--gold); color: var(--gold); }

.table-empty {
  padding: 60px 0;
  text-align: center;
  color: var(--t3);
  font-size: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

@keyframes spin { to { transform: rotate(360deg); } }
.spin-icon { display: inline-block; animation: spin 1.4s linear infinite; color: var(--gold); }

/* ── RWD ───────────────────────────────────────────────────── */
@media (max-width: 1100px) {
  .layout { grid-template-columns: 190px 1fr; }
}

@media (max-width: 900px) {
  .site-header__inner,
  .toolbar__inner,
  .layout { padding-left: 16px; padding-right: 16px; }

  .layout { grid-template-columns: 1fr; }
  .sidebar {
    position: static;
    max-height: none;
    border-right: none;
    border-bottom: 1px solid var(--line);
    padding: 16px 0;
  }
  .filter-list { flex-direction: row; flex-wrap: wrap; gap: 4px; }
  .filter-item { width: auto; border-left: none; border: 1px solid var(--line); }
  .filter-item.active { border-color: var(--gold); }
  .tag-row { width: auto; }
  .main-content { padding: 20px 0 0; }
}

@media (max-width: 640px) {
  .header-date { display: none; }
  .toolbar__inner { flex-direction: column; align-items: flex-start; gap: 10px; }
  .search-input { width: 100%; }
  .stock-table th:nth-child(5),
  .stock-table td:nth-child(5),
  .stock-table th:nth-child(6),
  .stock-table td:nth-child(6),
  .stock-table th:nth-child(7),
  .stock-table td:nth-child(7) { display: none; }
}
</style>
