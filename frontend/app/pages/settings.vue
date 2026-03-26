<script setup lang="ts">
useHead({
  title: '系統設定 | Stock',
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

const { isDark } = useAppPrefs()

// ── 型別 ──────────────────────────────────────────────────────
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

// ── 狀態 ──────────────────────────────────────────────────────
const features = ref<Feature[]>([])
const selectedId = ref<string | null>(null)
const loading = ref(false)
const saving = ref(false)
const savedId = ref<string | null>(null)
const errorMsg = ref('')

// 每個功能的「草稿設定」（修改中但未儲存）
const drafts = ref<Record<string, FeatureConfig>>({})

// ── API ────────────────────────────────────────────────────────
async function fetchSettings() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fetch('/api/settings/features')
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    features.value = await res.json()
    // 初始化草稿
    for (const f of features.value) {
      drafts.value[f.id] = { ...f.config }
    }
    if (!selectedId.value && features.value.length) {
      const first = features.value[0]
      if (first) selectedId.value = first.id
    }
  } catch (e: any) {
    errorMsg.value = e.message
  } finally {
    loading.value = false
  }
}

async function saveFeature(id: string) {
  saving.value = true
  savedId.value = null
  errorMsg.value = ''
  try {
    const res = await fetch(`/api/settings/features/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(drafts.value[id]),
    })
    if (!res.ok) {
      const data = await res.json()
      throw new Error(data.error || `HTTP ${res.status}`)
    }
    const updated: Feature = await res.json()
    // 更新本地 features 並同步草稿
    const idx = features.value.findIndex(f => f.id === id)
    if (idx !== -1) features.value[idx] = updated
    drafts.value[id] = { ...updated.config }
    savedId.value = id
    setTimeout(() => { savedId.value = null }, 2000)
  } catch (e: any) {
    errorMsg.value = e.message
  } finally {
    saving.value = false
  }
}

// ── 計算屬性 ────────────────────────────────────────────────────
const selectedFeature = computed(() =>
  features.value.find(f => f.id === selectedId.value),
)

const EMPTY_CONFIG: FeatureConfig = { primary: '', fallback_enabled: false, fallback: '', fallback_trigger: 'error' }

const draft = computed<FeatureConfig>(() =>
  selectedId.value ? (drafts.value[selectedId.value] ?? EMPTY_CONFIG) : EMPTY_CONFIG,
)

const isDirty = computed(() => {
  if (!selectedId.value || !selectedFeature.value) return false
  const d = drafts.value[selectedId.value]
  const orig = selectedFeature.value.config
  return JSON.stringify(d) !== JSON.stringify(orig)
})

// fallback 方案的可選選項（排除 primary 本身）
const fallbackOptions = computed<SchemeInfo[]>(() => {
  if (!selectedFeature.value) return []
  return selectedFeature.value.schemes.filter(s => s.id !== draft.value.primary)
})

// 當 primary 改變時，若 fallback 和 primary 相同則清空
watch(
  () => draft.value.primary,
  (newPrimary) => {
    if (!selectedId.value) return
    const d = drafts.value[selectedId.value]
    if (d && d.fallback === newPrimary) {
      d.fallback = fallbackOptions.value[0]?.id ?? ''
    }
  },
)

// ── Scheme 顏色 ──────────────────────────────────────────────
const schemeColors: Record<string, string> = {
  go_http:            '#4ecfa8',
  python_http:        '#5b9cf6',
  python_playwright:  '#e07b5a',
  twse_tpex_api:      '#a78ce8',
  broker_api:         '#f0a842',
}
function schemeColor(id: string) {
  return schemeColors[id] ?? '#8b949e'
}

const categoryLabels: Record<string, string> = {
  scraper: '爬蟲',
  sync: '同步',
}

onMounted(fetchSettings)
</script>

<template>
  <div class="page" :class="{ light: !isDark }">
    <!-- Header -->
    <header class="header">
      <div class="header__inner">
        <div class="brand">
          <NuxtLink to="/" class="back-btn" aria-label="返回首頁">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M10 3L5 8l5 5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </NuxtLink>
          <div class="brand-divider" />
          <div class="brand-info">
            <span class="brand-main">系統設定</span>
            <span class="brand-sub">Data Source Configuration</span>
          </div>
        </div>
        <nav class="header-nav">
          <button class="btn-refresh" :disabled="loading" @click="fetchSettings">
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none" :class="{ spin: loading }">
              <path d="M13.65 2.35A8 8 0 1 0 14.9 8.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
              <path d="M14.5 2v4h-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            重新整理
          </button>
        </nav>
      </div>
    </header>

    <!-- Error Banner -->
    <div v-if="errorMsg" class="error-banner">
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M8 1L1 14h14L8 1Z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/><path d="M8 7v3M8 12v.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
      {{ errorMsg }}
    </div>

    <div class="layout">
      <!-- 左側功能列表 -->
      <aside class="sidebar">
        <div class="sidebar-section-title">功能列表</div>
        <div v-if="loading" class="loading-text">
          <span class="spin-icon">◌</span> 載入中…
        </div>
        <ul v-else class="feature-list">
          <li
            v-for="f in features"
            :key="f.id"
            class="feature-item"
            :class="{ active: f.id === selectedId }"
            @click="selectedId = f.id"
          >
            <div class="feature-item-top">
              <span class="feature-item-label">{{ f.label }}</span>
              <span class="badge-category">{{ categoryLabels[f.category] ?? f.category }}</span>
            </div>
            <div class="feature-item-scheme">
              <span
                class="scheme-dot"
                :style="{ background: schemeColor(drafts[f.id]?.primary ?? '') }"
              />
              <span class="scheme-id-text">{{ drafts[f.id]?.primary }}</span>
              <span v-if="drafts[f.id]?.fallback_enabled" class="fallback-badge">備案</span>
            </div>
          </li>
        </ul>
      </aside>

      <!-- 右側設定內容 -->
      <main class="content">
        <div v-if="!selectedFeature" class="empty-state">
          <svg width="40" height="40" viewBox="0 0 40 40" fill="none" class="empty-icon" aria-hidden="true">
            <circle cx="20" cy="20" r="7" stroke="currentColor" stroke-width="1.8"/>
            <path d="M20 3v3M20 34v3M3 20h3M34 20h3M7.4 7.4l2.1 2.1M30.5 30.5l2.1 2.1M7.4 32.6l2.1-2.1M30.5 9.5l2.1-2.1" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
          </svg>
          <p>從左側選擇一個功能</p>
        </div>

        <template v-else-if="selectedFeature">
          <!-- 功能標頭 -->
          <div class="content-header">
            <div>
              <h2 class="content-title">{{ selectedFeature.label }}</h2>
              <p class="content-desc">{{ selectedFeature.description }}</p>
            </div>
            <div class="header-actions">
              <span v-if="savedId === selectedFeature.id" class="saved-badge">✓ 已儲存</span>
              <button
                class="btn-save"
                :class="{ dirty: isDirty }"
                :disabled="saving || !isDirty"
                @click="saveFeature(selectedFeature.id)"
              >
                {{ saving ? '儲存中…' : '儲存設定' }}
              </button>
            </div>
          </div>

          <!-- section: 主要方案 -->
          <section class="setting-section">
            <div class="section-title">
              <span class="section-icon">①</span> 主要方案
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
                <input
                  v-model="draft.primary"
                  type="radio"
                  :value="s.id"
                  class="scheme-radio"
                />
                <div class="scheme-card-header">
                  <span class="scheme-color-bar" />
                  <span class="scheme-label">{{ s.label }}</span>
                  <span v-if="draft.primary === s.id" class="scheme-active-badge">主要</span>
                </div>
                <p class="scheme-desc">{{ s.description }}</p>
                <div v-if="s.need_service" class="scheme-requires">
                  需要外部服務：<code>{{ s.need_service }}</code>
                </div>
              </label>
            </div>
          </section>

          <!-- section: 備案設定 -->
          <section class="setting-section">
            <div class="section-title">
              <span class="section-icon">②</span> 備案設定
              <span class="section-hint">主方案失敗時的行為</span>
            </div>

            <div class="fallback-row">
              <label class="toggle-label">
                <div
                  class="toggle"
                  :class="{ on: draft.fallback_enabled }"
                  @click="draft.fallback_enabled = !draft.fallback_enabled"
                >
                  <div class="toggle-thumb" />
                </div>
                <span>{{ draft.fallback_enabled ? '啟用備案' : '停用備案（失敗時直接報錯）' }}</span>
              </label>
            </div>

            <template v-if="draft.fallback_enabled">
              <!-- 備案方案選擇 -->
              <div class="fallback-config">
                <div class="fallback-group">
                  <label class="config-label">備案方案</label>
                  <div v-if="fallbackOptions.length === 0" class="no-fallback-hint">
                    沒有其他可用方案（該功能只有一個方案）
                  </div>
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
                        <span class="radio-desc">主方案無法連線或回傳錯誤時啟用備案</span>
                      </div>
                    </label>
                    <label class="radio-option" :class="{ selected: draft.fallback_trigger === 'empty_data' }">
                      <input v-model="draft.fallback_trigger" type="radio" value="empty_data" class="scheme-radio" />
                      <div>
                        <span class="radio-label">資料為空時</span>
                        <span class="radio-desc">主方案成功但資料為空時也啟用備案</span>
                      </div>
                    </label>
                  </div>
                </div>
              </div>

              <!-- 備案流程預覽 -->
              <div class="flow-preview">
                <div class="flow-node primary-node" :style="{ '--c': schemeColor(draft.primary) }">
                  {{ selectedFeature.schemes.find(s => s.id === draft.primary)?.label ?? draft.primary }}
                </div>
                <div class="flow-arrow">
                  <span class="flow-arrow-label">
                    {{ draft.fallback_trigger === 'error' ? '連線失敗 →' : '資料為空 →' }}
                  </span>
                  →
                </div>
                <div
                  v-if="draft.fallback"
                  class="flow-node fallback-node"
                  :style="{ '--c': schemeColor(draft.fallback) }"
                >
                  {{ selectedFeature.schemes.find(s => s.id === draft.fallback)?.label ?? draft.fallback }}
                </div>
                <div v-else class="flow-node empty-node">（未選擇備案）</div>
              </div>
            </template>
          </section>

          <!-- section: 方案說明彙整 -->
          <section class="setting-section">
            <div class="section-title">
              <span class="section-icon">③</span> 所有方案說明
            </div>
            <table class="scheme-table">
              <thead>
                <tr>
                  <th>方案</th>
                  <th>說明</th>
                  <th>依賴</th>
                  <th>狀態</th>
                </tr>
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

  --radius: 12px;
  --font:   'DM Sans', system-ui, 'PingFang TC', 'Microsoft JhengHei', sans-serif;
  --mono:   'Fira Code', 'JetBrains Mono', ui-monospace, monospace;

  min-height: 100vh;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
  font-size: 15px;
  line-height: 1.55;
  display: flex;
  flex-direction: column;
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
  position: sticky;
  top: 0;
  z-index: 50;
  background: color-mix(in oklch, var(--s1) 85%, transparent);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--line);
}

.header__inner {
  padding: 0 24px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.back-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--t2);
  text-decoration: none;
  transition: background 0.2s, border-color 0.2s, color 0.2s;
  flex-shrink: 0;
}
.back-btn:hover { background: var(--s3); border-color: var(--line2); color: var(--t1); }

.brand-divider {
  width: 1px;
  height: 20px;
  background: var(--line2);
  flex-shrink: 0;
}

.brand-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.brand-main {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--t1);
  line-height: 1;
}

.brand-sub {
  font-size: 10px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
  line-height: 1;
}

.header-nav {
  display: flex;
  align-items: center;
  gap: 12px;
}

.btn-refresh {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 14px;
  border-radius: 8px;
  background: var(--s2);
  border: 1px solid var(--line);
  color: var(--t2);
  font-family: var(--font);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s, color 0.2s;
}
.btn-refresh:hover { background: var(--s3); border-color: var(--line2); color: var(--t1); }
.btn-refresh:disabled { opacity: 0.4; cursor: default; }

/* ── Error Banner ───────────────────────────────────────────── */
.error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  background: color-mix(in oklch, var(--up) 8%, var(--s1));
  border-left: 3px solid var(--up);
  padding: 10px 24px;
  font-size: 13px;
  color: var(--up);
}

/* ── Layout ─────────────────────────────────────────────────── */
.layout {
  display: flex;
  flex: 1;
  overflow: hidden;
  height: calc(100vh - 56px);
}

/* ── Sidebar ─────────────────────────────────────────────────── */
.sidebar {
  width: 256px;
  min-width: 200px;
  background: var(--s1);
  border-right: 1px solid var(--line);
  overflow-y: auto;
  padding: 14px 0;
  flex-shrink: 0;
}

.sidebar-section-title {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
  padding: 0 16px 10px;
}

.loading-text {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px;
  color: var(--t3);
  font-size: 13px;
}

.spin-icon {
  display: inline-block;
  animation: spin 1.2s linear infinite;
}

.feature-list { list-style: none; }

.feature-item {
  padding: 10px 16px;
  cursor: pointer;
  border-left: 2.5px solid transparent;
  transition: background 0.15s, border-color 0.15s;
}
.feature-item:hover { background: var(--s2); }
.feature-item.active {
  background: var(--s2);
  border-left-color: var(--blue);
}

.feature-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.feature-item-label {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--t1);
}

.badge-category {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  padding: 2px 7px;
  border-radius: 20px;
  background: var(--s3);
  color: var(--t3);
  border: 1px solid var(--line);
}

.feature-item-scheme {
  display: flex;
  align-items: center;
  gap: 6px;
}

.scheme-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.scheme-id-text {
  font-size: 11.5px;
  font-family: var(--mono);
  color: var(--t3);
}

.fallback-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 20px;
  background: color-mix(in oklch, var(--dn) 12%, var(--s2));
  color: var(--dn);
  border: 1px solid color-mix(in oklch, var(--dn) 35%, var(--line));
}

/* ── Content ─────────────────────────────────────────────────── */
.content {
  flex: 1;
  overflow-y: auto;
  background: var(--bg);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 14px;
  color: var(--t3);
}

.empty-icon { opacity: 0.4; }

/* ── Content Header ─────────────────────────────────────────── */
.content-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 24px 28px 0;
  border-bottom: 1px solid var(--line);
  padding-bottom: 20px;
}

.content-title {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--t1);
  margin-bottom: 5px;
}

.content-desc {
  font-size: 13px;
  color: var(--t2);
  line-height: 1.6;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
  margin-left: 16px;
}

.saved-badge {
  font-size: 12px;
  font-weight: 600;
  color: var(--dn);
  background: color-mix(in oklch, var(--dn) 12%, var(--s2));
  border: 1px solid color-mix(in oklch, var(--dn) 40%, var(--line));
  padding: 5px 12px;
  border-radius: 7px;
}

.btn-save {
  padding: 8px 20px;
  border-radius: 8px;
  background: var(--s2);
  border: 1px solid var(--line2);
  color: var(--t3);
  font-family: var(--font);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-save.dirty {
  background: color-mix(in oklch, var(--blue) 14%, var(--s2));
  border-color: var(--blue);
  color: var(--blue);
  box-shadow: 0 0 0 0 color-mix(in oklch, var(--blue) 25%, transparent);
}
.btn-save.dirty:hover {
  background: color-mix(in oklch, var(--blue) 20%, var(--s2));
  box-shadow: 0 2px 12px color-mix(in oklch, var(--blue) 25%, transparent);
}
.btn-save:disabled { opacity: 0.4; cursor: default; }

/* ── Sections ────────────────────────────────────────────────── */
.setting-section {
  padding: 22px 28px;
  border-bottom: 1px solid var(--line);
}

.setting-section:last-child { border-bottom: none; }

.section-title {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: var(--t1);
  margin-bottom: 16px;
}

.section-icon {
  font-size: 11px;
  font-weight: 700;
  background: var(--s3);
  border: 1px solid var(--line2);
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--t2);
}

.section-hint {
  font-size: 12px;
  color: var(--t3);
  font-weight: 400;
}

/* ── Scheme Grid (主方案) ────────────────────────────────────── */
.scheme-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 10px;
}

.scheme-card {
  border: 1px solid var(--line);
  border-radius: var(--radius);
  background: var(--s1);
  padding: 14px 16px;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
  overflow: hidden;
}
.scheme-card:hover {
  border-color: var(--line2);
  background: var(--s2);
}
.scheme-card.selected {
  border-color: var(--accent, var(--blue));
  background: color-mix(in oklch, var(--accent, var(--blue)) 6%, var(--s2));
}
.scheme-card.selected::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 2px;
  background: var(--accent, var(--blue));
}
.scheme-radio { position: absolute; opacity: 0; pointer-events: none; }

.scheme-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.scheme-color-bar {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--accent, var(--blue));
  flex-shrink: 0;
}

.scheme-label {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--t1);
  flex: 1;
}

.scheme-active-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 20px;
  background: color-mix(in oklch, var(--accent, var(--blue)) 18%, transparent);
  color: var(--accent, var(--blue));
  border: 1px solid color-mix(in oklch, var(--accent, var(--blue)) 45%, transparent);
}

.scheme-desc { font-size: 12px; color: var(--t2); line-height: 1.55; }

.scheme-requires {
  margin-top: 9px;
  font-size: 11.5px;
  color: var(--warn);
}
.scheme-requires code { font-family: var(--mono); }

/* ── Fallback Controls ───────────────────────────────────────── */
.fallback-row { margin-bottom: 16px; }

.toggle-label {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  user-select: none;
  font-size: 13.5px;
  color: var(--t1);
}

.toggle {
  width: 38px;
  height: 21px;
  border-radius: 21px;
  background: var(--line2);
  position: relative;
  transition: background 0.2s;
  cursor: pointer;
  flex-shrink: 0;
}
.toggle.on { background: color-mix(in oklch, var(--dn) 80%, var(--s3)); }

.toggle-thumb {
  position: absolute;
  top: 2.5px;
  left: 2.5px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--t1);
  transition: left 0.15s;
}
.toggle.on .toggle-thumb { left: 19.5px; }

.fallback-config { display: flex; flex-direction: column; gap: 16px; }

.config-label {
  display: block;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 8px;
}

.no-fallback-hint {
  font-size: 13px;
  color: var(--t3);
  font-style: italic;
}

.scheme-select-row { display: flex; flex-direction: column; gap: 7px; }

.scheme-select-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--line);
  border-radius: 9px;
  cursor: pointer;
  transition: all 0.15s;
}
.scheme-select-item:hover { background: var(--s2); border-color: var(--line2); }
.scheme-select-item.selected {
  border-color: var(--accent, var(--blue));
  background: color-mix(in oklch, var(--accent, var(--blue)) 6%, var(--s2));
}

.scheme-color-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent, var(--blue));
  flex-shrink: 0;
  margin-top: 4px;
}

.scheme-select-label {
  font-size: 13.5px;
  color: var(--t1);
  font-weight: 600;
}
.scheme-select-desc { font-size: 12px; color: var(--t2); }

.radio-row { display: flex; flex-direction: column; gap: 7px; }

.radio-option {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--line);
  border-radius: 9px;
  cursor: pointer;
  transition: all 0.15s;
}
.radio-option:hover { background: var(--s2); border-color: var(--line2); }
.radio-option.selected {
  border-color: var(--blue);
  background: color-mix(in oklch, var(--blue) 6%, var(--s2));
}

.radio-label {
  font-size: 13.5px;
  color: var(--t1);
  font-weight: 600;
  display: block;
  margin-bottom: 2px;
}
.radio-desc { font-size: 12px; color: var(--t2); }

/* ── Flow Preview ───────────────────────────────────────────── */
.flow-preview {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  padding: 14px 16px;
  border-radius: 10px;
  background: var(--s1);
  border: 1px solid var(--line);
}

.flow-node {
  padding: 6px 16px;
  border-radius: 20px;
  font-size: 12.5px;
  font-weight: 600;
  border: 1px solid color-mix(in oklch, var(--c, var(--t3)) 45%, transparent);
  background: color-mix(in oklch, var(--c, var(--t3)) 12%, transparent);
  color: var(--c, var(--t3));
}

.empty-node { --c: var(--t3); }

.flow-arrow {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  color: var(--t3);
  font-size: 1rem;
}

.flow-arrow-label {
  font-size: 10px;
  color: var(--t3);
  white-space: nowrap;
  font-weight: 500;
}

/* ── Scheme Table ───────────────────────────────────────────── */
.scheme-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.scheme-table th {
  padding: 9px 12px;
  text-align: left;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--t3);
  border-bottom: 1px solid var(--line);
}

.scheme-table td {
  padding: 11px 12px;
  border-bottom: 1px solid var(--line);
  vertical-align: top;
}
.scheme-table tr:last-child td { border-bottom: none; }
.scheme-table tbody tr:hover td { background: var(--s1); }

.scheme-table-id {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-bottom: 4px;
}

.scheme-table-label {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--t1);
}

.scheme-id-code {
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--t3);
  background: var(--s3);
  padding: 1px 7px;
  border-radius: 5px;
}

.scheme-table-desc {
  font-size: 12.5px;
  color: var(--t2);
  line-height: 1.55;
}

.need-service-badge {
  font-family: var(--mono);
  font-size: 11.5px;
  background: color-mix(in oklch, var(--blue) 12%, var(--s2));
  color: var(--blue);
  padding: 2px 8px;
  border-radius: 5px;
  border: 1px solid color-mix(in oklch, var(--blue) 30%, var(--line));
}

.no-dep {
  font-size: 12px;
  color: var(--t3);
}

.status-primary {
  font-size: 11px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 20px;
  background: color-mix(in oklch, var(--blue) 14%, var(--s2));
  color: var(--blue);
  border: 1px solid color-mix(in oklch, var(--blue) 40%, var(--line));
}

.status-fallback {
  font-size: 11px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 20px;
  background: color-mix(in oklch, var(--dn) 12%, var(--s2));
  color: var(--dn);
  border: 1px solid color-mix(in oklch, var(--dn) 35%, var(--line));
}

.status-idle {
  font-size: 12px;
  color: var(--t3);
}

/* ── Animations ──────────────────────────────────────────────── */
@keyframes spin { to { transform: rotate(360deg); } }
.spin { display: inline-block; animation: spin 1s linear infinite; }

/* ── Responsive ──────────────────────────────────────────────── */
@media (max-width: 768px) {
  .layout { flex-direction: column; height: auto; overflow: visible; }
  .sidebar { width: 100%; border-right: none; border-bottom: 1px solid var(--line); padding: 8px 0; }
  .content { overflow: visible; }
  .scheme-grid { grid-template-columns: 1fr; }
}
</style>
