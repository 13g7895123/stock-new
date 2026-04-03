<template>
  <div class="page" :class="{ light: !isDark, classic: isClassic }">
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="" />
    <link href="https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700&display=swap" rel="stylesheet" />

    <!-- ══ Bento / Terminal Header ══ -->
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
          <span class="brand-cur">股票列表</span>
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
          <span class="classic-badge">TSM</span>
          <div class="classic-brand-text">
            <span class="classic-brand-sub">Taiwan Stock Monitor</span>
            <span class="classic-brand-name">股票列表</span>
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
                {{ resolveIndustry(industry) }}
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

        <!-- Group Filter + Management -->
        <section class="filter-section">
          <div class="filter-heading">
            <span>群組</span>
            <button v-if="selectedGroupId" class="clear-btn" @click="selectedGroupId = 0">清除</button>
          </div>
          <div v-if="groupsLoading" class="filter-loading">載入中…</div>
          <ul v-else class="filter-list">
            <li>
              <button
                class="filter-item"
                :class="{ active: selectedGroupId === 0 }"
                @click="selectedGroupId = 0"
              >
                全部群組
              </button>
            </li>
            <li v-for="g in groups" :key="g.id" class="tag-row">
              <button
                class="filter-item tag-item"
                :class="{ active: selectedGroupId === g.id }"
                @click="selectedGroupId = g.id"
              >
                <span class="tag-dot" :style="{ background: g.color }"></span>
                {{ g.name }}
              </button>
              <button class="tag-del-btn" title="刪除群組" @click.stop="deleteGroup(g.id)">×</button>
            </li>
          </ul>

          <!-- Create Group -->
          <form class="tag-create group-create" @submit.prevent="createGroup">
            <div class="group-create-row">
              <input
                v-model="newGroupName"
                class="tag-name-input"
                type="text"
                placeholder="群組名稱…"
                maxlength="40"
              />
              <input v-model="newGroupColor" class="tag-color-input" type="color" title="選擇顏色" />
              <button type="submit" class="tag-add-btn" :disabled="!newGroupName.trim()">+</button>
            </div>
            <textarea
              v-model="newGroupDesc"
              class="group-desc-input"
              placeholder="群組說明（選填）"
              rows="2"
              maxlength="200"
            ></textarea>
          </form>
          <p v-if="groupError" class="tag-err">{{ groupError }}</p>
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
              <th>群組</th>
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
                <span v-if="stock.industry" class="industry-pill">{{ resolveIndustry(stock.industry) }}</span>
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
              <td class="td-tags td-groups">
                <div class="tags-cell">
                  <span
                    v-for="g in (stock.groups ?? [])"
                    :key="g.id"
                    class="tag-chip"
                    :style="{ borderColor: g.color, color: g.color }"
                  >{{ g.name }}</span>
                  <button
                    class="tag-assign-btn"
                    title="管理群組"
                    @click="toggleGroupRow(stock.symbol)"
                  >＋</button>
                </div>
                <!-- Group Assign Popover -->
                <div v-if="expandedGroupRow === stock.symbol" class="tag-popover">
                  <p class="tag-popover-head">選擇群組</p>
                  <label
                    v-for="g in groups"
                    :key="g.id"
                    class="tag-popover-item"
                  >
                    <input
                      type="checkbox"
                      :checked="stockHasGroup(stock, g.id)"
                      @change="toggleStockGroup(stock, g.id)"
                    />
                    <span class="tag-dot" :style="{ background: g.color }"></span>
                    {{ g.name }}
                  </label>
                  <div v-if="groups.length === 0" class="tag-popover-no-group">
                    <p class="tag-popover-empty">尚無群組</p>
                    <form class="popover-group-create" @submit.prevent="quickCreateGroup">
                      <input
                        v-model="quickGroupName"
                        class="popover-group-input"
                        type="text"
                        placeholder="輸入群組名稱…"
                        maxlength="40"
                        @click.stop
                      />
                      <input v-model="quickGroupColor" class="tag-color-input" type="color" title="選擇顏色" @click.stop />
                      <button type="submit" class="tag-add-btn" :disabled="!quickGroupName.trim()">+</button>
                    </form>
                  </div>
                </div>
              </td>
              <td class="ca">
                <NuxtLink :to="`/stocks/${stock.symbol}`" class="row-link">K 線</NuxtLink>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-else-if="!tableLoading" class="table-empty">
          {{ searchQuery || selectedIndustry || selectedTagId || selectedGroupId ? '查無符合條件的股票' : '尚無資料。請先同步股票清單。' }}
        </div>

      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAppPrefs } from '~/composables/useAppPrefs'

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
  industry: string
  market: string
  price: number
  change: number
  change_pct: number
  volume: number
  updated_at: string
  tags: Tag[]
  groups: StockGroup[]
}

// ── Theme + Style ───────────────────────────────────────────────
const { isDark, appStyle, isBento, isClassic, toggleTheme, setTheme, setStyle } = useAppPrefs()
const settingsOpen = ref(false)

// ── Date ───────────────────────────────────────────────────────
const today = new Date().toLocaleDateString('zh-TW', {
  year: 'numeric', month: 'long', day: 'numeric', weekday: 'short'
})

// ── Filters ────────────────────────────────────────────────────
const searchQuery      = ref('')
const selectedIndustry = ref('')
const selectedTagId    = ref(0)
const selectedGroupId  = ref(0)

// ── Industry Code → 中文對照 ───────────────────────────
const industryMap: Record<string, string> = {
  '01': '水泥工業', '02': '食品工業', '03': '塑膠工業', '04': '紡織纖維',
  '05': '電機機械', '06': '電器電纜', '08': '化學工業', '09': '生技醫療業',
  '10': '玻璃陶瓷', '11': '造紙工業', '12': '鋼鐵工業', '13': '橡膠工業',
  '14': '汽車工業', '15': '電子工業', '16': '建材營造業', '17': '航運業',
  '18': '觀光餐旅', '19': '金融業', '20': '貿易百貨業', '21': '綜合',
  '22': '其他', '24': '油電燃氣業', '25': '半導體業', '26': '電腦及週邊設備業',
  '27': '光電業', '28': '通信網路業', '29': '電子零組件業', '30': '電子通路業',
  '31': '資訊服務業', '32': '其他電子業', '33': '文化創意業', '34': '農業科技業',
  '35': '電子商務業', '36': '綠能環保', '37': '數位雲端', '38': '運動休閒', '39': '居家生活',
}
function resolveIndustry(code: string): string {
  return industryMap[code] ?? code
}

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

// ── Groups ───────────────────────────────────────────────
const groups        = ref<StockGroup[]>([])
const groupsLoading = ref(true)
const groupError    = ref('')
const newGroupName  = ref('')
const newGroupDesc  = ref('')
const newGroupColor = ref('#3b82f6')

async function fetchGroups() {
  try {
    const res = await $fetch<StockGroup[]>('/api/groups')
    groups.value = Array.isArray(res) ? res : []
  } catch {
    groups.value = []
  } finally {
    groupsLoading.value = false
  }
}

async function createGroup() {
  const name = newGroupName.value.trim()
  if (!name) return
  groupError.value = ''
  try {
    await $fetch('/api/groups', {
      method: 'POST',
      body: { name, description: newGroupDesc.value.trim(), color: newGroupColor.value }
    })
    newGroupName.value = ''
    newGroupDesc.value = ''
    newGroupColor.value = '#3b82f6'
    await fetchGroups()
  } catch {
    groupError.value = '建立群組失敗'
  }
}

async function deleteGroup(id: number) {
  groupError.value = ''
  try {
    await $fetch(`/api/groups/${id}`, { method: 'DELETE' })
    if (selectedGroupId.value === id) selectedGroupId.value = 0
    await Promise.all([fetchGroups(), fetchStocks()])
  } catch {
    groupError.value = '刪除失敗'
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
    if (selectedGroupId.value)  params.group_id  = String(selectedGroupId.value)
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
watch([selectedIndustry, selectedTagId, selectedGroupId], () => fetchStocks())

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

// ── Group Assignment ───────────────────────────────────────────
const expandedGroupRow = ref('')
const quickGroupName  = ref('')
const quickGroupColor = ref('#3b82f6')

function toggleGroupRow(symbol: string) {
  expandedGroupRow.value = expandedGroupRow.value === symbol ? '' : symbol
  quickGroupName.value  = ''
  quickGroupColor.value = '#3b82f6'
}

async function quickCreateGroup() {
  const name = quickGroupName.value.trim()
  if (!name) return
  try {
    await $fetch('/api/groups', {
      method: 'POST',
      body: { name, color: quickGroupColor.value, description: '' }
    })
    quickGroupName.value  = ''
    quickGroupColor.value = '#3b82f6'
    await fetchGroups()
  } catch {
    groupError.value = '建立群組失敗'
  }
}

function stockHasGroup(stock: Stock, groupId: number): boolean {
  return (stock.groups ?? []).some(g => g.id === groupId)
}

async function toggleStockGroup(stock: Stock, groupId: number) {
  const current = (stock.groups ?? []).map(g => g.id)
  const next = stockHasGroup(stock, groupId)
    ? current.filter(id => id !== groupId)
    : [...current, groupId]
  try {
    await $fetch(`/api/stocks/${stock.symbol}/groups`, {
      method: 'PUT',
      body: { group_ids: next }
    })
    const group = groups.value.find(g => g.id === groupId)
    if (stockHasGroup(stock, groupId)) {
      stock.groups = (stock.groups ?? []).filter(g => g.id !== groupId)
    } else if (group) {
      stock.groups = [...(stock.groups ?? []), group]
    }
  } catch {
    groupError.value = '更新群組失敗'
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
  await Promise.all([fetchIndustries(), fetchTags(), fetchGroups(), fetchStocks()])
})
</script>

<style scoped>
/* ── Design Tokens ─────────────────────────────────────────── */
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
  --warn:  oklch(73%   0.13  72);
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
  --warn:  oklch(50%   0.16  72);
}

/* ── Classic Mode ──────────────────────────────────────────── */
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
  --warn:  oklch(72%   0.13  72);
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
  --warn:  oklch(52%   0.14  72);
}

/* ── Header ────────────────────────────────────────────────── */
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
  font-size: 12px;
  color: var(--t3);
  font-variant-numeric: tabular-nums;
}
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

/* ── Toolbar ───────────────────────────────────────────────── */
.toolbar {
  background: color-mix(in oklch, var(--s2) 90%, transparent);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
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
  padding: 9px 14px;
  font-size: 14px;
  font-family: var(--font);
  background: var(--s1);
  border: 1px solid var(--line2);
  border-radius: 9px;
  outline: none;
  color: var(--t1);
  transition: border-color 0.15s, box-shadow 0.15s;
}
.search-input:focus {
  border-color: var(--blue);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--blue) 16%, transparent);
}
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
  border-radius: 6px;
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
  border-radius: 5px;
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
  border-radius: 6px;
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
  padding: 2px 8px;
  border-radius: 100px;
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
  padding: 2px 8px;
  border: 1px solid;
  border-radius: 100px;
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
  border-radius: 10px;
  padding: 10px 12px;
  min-width: 160px;
  box-shadow: 0 8px 24px oklch(0% 0 0 / 0.28);
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
.tag-popover-empty { font-size: 12.5px; color: var(--t3); margin-bottom: 6px; }
.tag-popover-no-group { display: flex; flex-direction: column; gap: 6px; }
.popover-group-create { display: flex; align-items: center; gap: 5px; }
.popover-group-input {
  flex: 1;
  background: var(--s3);
  border: 1px solid var(--line2);
  border-radius: 6px;
  color: var(--t1);
  font-size: 12px;
  padding: 4px 7px;
  outline: none;
  min-width: 0;
}
.popover-group-input:focus { border-color: var(--gold); }

.row-link {
  font-size: 12.5px;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: var(--t3);
  text-decoration: none;
  padding: 3px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  transition: border-color 0.15s, color 0.15s;
}
.row-link:hover { border-color: var(--gold); color: var(--gold); }

/* Groups */
.td-groups { position: relative; min-width: 110px; }
.group-create { flex-direction: column; gap: 6px; }
.group-create-row { display: flex; align-items: center; gap: 6px; }
.group-desc-input {
  width: 100%;
  box-sizing: border-box;
  background: var(--s3);
  border: 1px solid var(--line2);
  border-radius: 7px;
  color: var(--t1);
  font-family: var(--font);
  font-size: 12.5px;
  padding: 6px 9px;
  resize: vertical;
  min-height: 46px;
  outline: none;
  transition: border-color 0.15s;
}
.group-desc-input:focus { border-color: var(--gold); }
.group-desc-input::placeholder { color: var(--t3); }

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

/* ── Settings Panel ────────────────────────────────────────── */
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

/* ── Classic structural overrides ───────────────────────────── */
.page.classic .search-input { border-radius: 4px; }
.page.classic .industry-pill { border-radius: 4px; }
.page.classic .tag-chip { border-radius: 4px; }
.page.classic .row-link { border-radius: 0; }
.page.classic .tag-popover { border-radius: 4px; }
.page.classic .btn-icon { display: none; }
.page.classic .settings-panel { border-radius: 4px; box-shadow: none; }
.page.classic .sp-btn { border-radius: 0; }

/* ── Classic Header ─────────────────────────────────────────── */
.classic-header {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}
.classic-header__inner {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 40px;
  height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.classic-brand { display: flex; align-items: center; gap: 14px; }
.classic-back { font-size: 12.5px; font-weight: 600; color: var(--t3); text-decoration: none; transition: color 0.15s; }
.classic-back:hover { color: var(--gold); }
.classic-sep { color: var(--line2); font-size: 14px; padding: 0 4px; }
.classic-badge { font-size: 11px; font-weight: 700; letter-spacing: 0.14em; color: var(--bg); background: var(--gold); padding: 5px 8px; line-height: 1; flex-shrink: 0; }
.classic-brand-text { display: flex; flex-direction: column; gap: 2px; }
.classic-brand-sub { font-size: 10px; letter-spacing: 0.18em; text-transform: uppercase; color: var(--t3); line-height: 1; }
.classic-brand-name { font-size: 16px; font-weight: 600; letter-spacing: 0.02em; color: var(--t1); line-height: 1; }
.classic-header-right { display: flex; align-items: center; gap: 16px; }
.classic-date { font-size: 12px; color: var(--t3); font-variant-numeric: tabular-nums; }
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
