<script setup lang="ts">
import { useAppPrefs } from '~/composables/useAppPrefs'

interface UploadedFile {
  id: number
  original_name: string
  stored_name: string
  size: number
  content_type: string
  uploaded_at: string
}

const { isDark, isClassic, toggleTheme } = useAppPrefs()

const files = ref<UploadedFile[]>([])
const loading = ref(true)
const uploading = ref(false)
const uploadError = ref('')
const uploadSuccess = ref('')
const dragging = ref(false)

const fileInput = ref<HTMLInputElement | null>(null)

async function fetchFiles() {
  loading.value = true
  try {
    const res = await $fetch<{ data: UploadedFile[] }>('/api/files')
    files.value = Array.isArray(res?.data) ? res.data : []
  } catch {
    files.value = []
  } finally {
    loading.value = false
  }
}

async function uploadFile(file: File) {
  if (uploading.value) return
  uploading.value = true
  uploadError.value = ''
  uploadSuccess.value = ''
  try {
    const form = new FormData()
    form.append('file', file)
    await $fetch('/api/files', { method: 'POST', body: form })
    uploadSuccess.value = `「${file.name}」上傳成功`
    await fetchFiles()
  } catch (err: any) {
    uploadError.value = err?.data?.error || err?.response?._data?.error || '上傳失敗，請重試'
  } finally {
    uploading.value = false
  }
}

function onFileInputChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) uploadFile(file)
  input.value = ''
}

function onDrop(e: DragEvent) {
  dragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file) uploadFile(file)
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  dragging.value = true
}

function onDragLeave() {
  dragging.value = false
}

function downloadUrl(file: UploadedFile) {
  return `/api/files/${file.id}/download`
}

function fullDownloadUrl(file: UploadedFile): string {
  if (import.meta.client) {
    return `${window.location.origin}/api/files/${file.id}/download`
  }
  return `/api/files/${file.id}/download`
}

const copyStates = ref<Record<number, boolean>>({})

async function copyUrl(file: UploadedFile) {
  const url = fullDownloadUrl(file)
  try {
    await navigator.clipboard.writeText(url)
  } catch {
    const el = document.createElement('textarea')
    el.value = url
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
  }
  copyStates.value[file.id] = true
  setTimeout(() => { copyStates.value[file.id] = false }, 2000)
}

async function deleteFile(file: UploadedFile) {
  if (!confirm(`確定要刪除「${file.original_name}」？`)) return
  try {
    await $fetch(`/api/files/${file.id}`, { method: 'DELETE' })
    await fetchFiles()
  } catch (err: any) {
    alert(err?.data?.error || '刪除失敗')
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function formatDate(value: string): string {
  if (!value) return '—'
  return new Date(value).toLocaleString('zh-TW', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

onMounted(fetchFiles)

function fileTypeIcon(contentType: string): string {
  if (!contentType) return '📄'
  if (contentType.startsWith('image/')) return '🖼'
  if (contentType === 'application/pdf') return '📕'
  if (contentType.includes('zip') || contentType.includes('compressed')) return '🗜'
  if (contentType.includes('spreadsheet') || contentType.includes('csv')) return '📊'
  if (contentType.includes('word') || contentType.includes('document')) return '📝'
  if (contentType.startsWith('text/')) return '📃'
  return '📄'
}
</script>

<template>
  <div class="page" :class="{ light: !isDark, classic: isClassic }">
    <header class="site-header">
      <div class="site-header__inner">
        <div class="brand">
          <NuxtLink to="/" class="back-link">首頁</NuxtLink>
          <span class="brand-sep">/</span>
          <span class="brand-cur">檔案暫存</span>
        </div>
        <button class="btn-icon" :aria-label="isDark ? '切換亮色模式' : '切換暗色模式'" @click="toggleTheme">
          <span v-if="isDark">☀</span><span v-else>☾</span>
        </button>
      </div>
    </header>

    <main class="content">
      <section class="topbar">
        <div>
          <p class="eyebrow">FILE STORAGE</p>
          <h1 class="page-title">檔案暫存</h1>
        </div>
        <div class="stat-chip">{{ files.length }} 個檔案</div>
      </section>

      <!-- 上傳區 -->
      <section
        class="upload-zone"
        :class="{ 'upload-zone--drag': dragging, 'upload-zone--uploading': uploading }"
        @drop.prevent="onDrop"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @click="fileInput?.click()"
      >
        <input
          ref="fileInput"
          type="file"
          class="hidden-input"
          @change="onFileInputChange"
        />
        <div class="upload-icon">
          <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
            <polyline points="17 8 12 3 7 8"/>
            <line x1="12" y1="3" x2="12" y2="15"/>
          </svg>
        </div>
        <p class="upload-text">
          <template v-if="uploading">上傳中…</template>
          <template v-else>拖曳檔案至此，或<span class="upload-link">點擊選擇</span></template>
        </p>
        <p class="upload-hint">支援任意格式</p>
      </section>

      <p v-if="uploadError" class="msg msg--error">{{ uploadError }}</p>
      <p v-if="uploadSuccess" class="msg msg--success">{{ uploadSuccess }}</p>

      <!-- 檔案列表 -->
      <section class="file-list-section">
        <div v-if="loading" class="empty-state">載入中…</div>
        <div v-else-if="files.length === 0" class="empty-state">尚無檔案，請上傳第一個檔案</div>
        <table v-else class="file-table">
          <thead>
            <tr>
              <th>檔案名稱</th>
              <th class="col-size">大小</th>
              <th class="col-date">上傳時間</th>
              <th class="col-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="file in files" :key="file.id" class="file-row">
              <td class="col-name">
                <div class="file-name-row">
                  <span class="file-icon">{{ fileTypeIcon(file.content_type) }}</span>
                  <span class="file-name">{{ file.original_name }}</span>
                </div>
                <div class="file-url-row">
                  <code class="url-text" :title="fullDownloadUrl(file)">{{ fullDownloadUrl(file) }}</code>
                </div>
              </td>
              <td class="col-size">{{ formatSize(file.size) }}</td>
              <td class="col-date">{{ formatDate(file.uploaded_at) }}</td>
              <td class="col-actions">
                <a :href="downloadUrl(file)" download class="btn-action btn-action--download">下載</a>
                <button
                  class="btn-action btn-action--copy"
                  :class="{ 'btn-action--copied': copyStates[file.id] }"
                  @click="copyUrl(file)"
                >
                  {{ copyStates[file.id] ? '✓ 已複製' : '複製連結' }}
                </button>
                <button class="btn-action btn-action--delete" @click="deleteFile(file)">刪除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
    </main>
  </div>
</template>

<style scoped>
/* ── Design tokens ─────────────────────────────────────── */
.page {
  --bg: oklch(14% 0.02 265);
  --s1: oklch(18% 0.02 265);
  --s2: oklch(22% 0.025 265);
  --s3: oklch(28% 0.03 265);
  --t1: oklch(96% 0.005 265);
  --t2: oklch(72% 0.01 265);
  --t3: oklch(48% 0.01 265);
  --gold: oklch(78% 0.15 85);
  --blue: oklch(65% 0.18 240);
  --up: oklch(65% 0.18 145);
  --dn: oklch(62% 0.2 25);
  --radius: 10px;
  background: var(--bg);
  color: var(--t1);
  min-height: 100dvh;
  font-family: 'DM Sans', sans-serif;
}

.page.light {
  --bg: oklch(97% 0.005 265);
  --s1: oklch(100% 0 0);
  --s2: oklch(94% 0.005 265);
  --s3: oklch(90% 0.008 265);
  --t1: oklch(15% 0.02 265);
  --t2: oklch(40% 0.015 265);
  --t3: oklch(60% 0.01 265);
}

.page.classic {
  --bg: #1a1a2e;
  --s1: #16213e;
  --s2: #0f3460;
  --s3: #533483;
  --t1: #e0e0e0;
  --t2: #a0a0c0;
  --t3: #6060a0;
  --gold: #e2b714;
  --blue: #4fc3f7;
}

/* ── Layout ─────────────────────────────────────────────── */
.site-header {
  position: sticky;
  top: 0;
  z-index: 50;
  background: var(--s1);
  border-bottom: 1px solid var(--s3);
}
.site-header__inner {
  max-width: 900px;
  margin: 0 auto;
  padding: 14px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.brand { display: flex; align-items: center; gap: 6px; font-size: 0.85rem; color: var(--t2); }
.back-link { color: var(--t2); text-decoration: none; }
.back-link:hover { color: var(--t1); }
.brand-sep { color: var(--t3); }
.brand-cur { color: var(--t1); font-weight: 600; }
.btn-icon {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1.1rem;
  color: var(--t2);
  padding: 4px 8px;
  border-radius: 6px;
}
.btn-icon:hover { color: var(--t1); background: var(--s3); }

.content {
  max-width: 900px;
  margin: 0 auto;
  padding: 32px 20px 64px;
}

/* ── Topbar ─────────────────────────────────────────────── */
.topbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 28px;
  flex-wrap: wrap;
}
.eyebrow { font-size: 0.7rem; letter-spacing: .15em; color: var(--t3); margin: 0 0 4px; text-transform: uppercase; }
.page-title { font-size: 1.6rem; font-weight: 700; margin: 0; color: var(--t1); }
.stat-chip {
  padding: 6px 14px;
  border-radius: 20px;
  background: var(--s2);
  color: var(--t2);
  font-size: 0.8rem;
  white-space: nowrap;
}

/* ── Upload Zone ────────────────────────────────────────── */
.upload-zone {
  border: 2px dashed var(--s3);
  border-radius: var(--radius);
  padding: 40px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  transition: border-color .2s, background .2s;
  background: var(--s1);
  margin-bottom: 16px;
}
.upload-zone:hover,
.upload-zone--drag {
  border-color: var(--blue);
  background: color-mix(in oklch, var(--blue) 6%, var(--s1));
}
.upload-zone--uploading {
  opacity: .6;
  pointer-events: none;
}
.upload-icon { color: var(--t3); }
.upload-text { margin: 0; font-size: 0.95rem; color: var(--t2); }
.upload-link { color: var(--blue); margin-left: 4px; }
.upload-hint { margin: 0; font-size: 0.78rem; color: var(--t3); }
.hidden-input { display: none; }

/* ── Messages ────────────────────────────────────────────── */
.msg {
  padding: 10px 16px;
  border-radius: 8px;
  font-size: 0.87rem;
  margin-bottom: 16px;
}
.msg--error { background: color-mix(in oklch, var(--dn) 15%, var(--s1)); color: var(--dn); }
.msg--success { background: color-mix(in oklch, var(--up) 15%, var(--s1)); color: var(--up); }

/* ── File Table ─────────────────────────────────────────── */
.file-list-section { margin-top: 8px; }
.empty-state {
  padding: 40px;
  text-align: center;
  color: var(--t3);
  font-size: 0.9rem;
  background: var(--s1);
  border-radius: var(--radius);
}
.file-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
}
.file-table th {
  text-align: left;
  padding: 10px 12px;
  color: var(--t3);
  font-weight: 500;
  font-size: 0.78rem;
  border-bottom: 1px solid var(--s3);
  text-transform: uppercase;
  letter-spacing: .06em;
}
.file-row td {
  padding: 12px 12px;
  border-bottom: 1px solid color-mix(in oklch, var(--s3) 50%, transparent);
  vertical-align: middle;
}
.file-row:hover td { background: var(--s1); }
.col-name { max-width: 360px; }
.col-size { width: 90px; color: var(--t2); white-space: nowrap; }
.col-date { width: 160px; color: var(--t2); white-space: nowrap; }
.col-actions { width: 190px; text-align: right; white-space: nowrap; }

.file-icon { margin-right: 8px; font-size: 1rem; }
.file-name-row { display: flex; align-items: center; }
.file-name {
  color: var(--t1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 320px;
  display: inline-block;
  vertical-align: middle;
}
.file-url-row {
  margin-top: 4px;
  padding-left: 26px;
}
.url-text {
  font-family: 'DM Mono', 'Fira Mono', monospace;
  font-size: 0.72rem;
  color: var(--t3);
  background: var(--s2);
  padding: 2px 6px;
  border-radius: 4px;
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  user-select: all;
  cursor: text;
}

.btn-action {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 0.78rem;
  font-weight: 500;
  cursor: pointer;
  text-decoration: none;
  transition: opacity .15s;
  border: none;
}
.btn-action:hover { opacity: .8; }
.btn-action--download {
  background: color-mix(in oklch, var(--blue) 20%, var(--s2));
  color: var(--blue);
  margin-right: 4px;
}
.btn-action--copy {
  background: color-mix(in oklch, var(--gold) 18%, var(--s2));
  color: var(--gold);
  margin-right: 4px;
  min-width: 62px;
}
.btn-action--copied {
  background: color-mix(in oklch, var(--up) 20%, var(--s2));
  color: var(--up);
}
.btn-action--delete {
  background: color-mix(in oklch, var(--dn) 18%, var(--s2));
  color: var(--dn);
}
</style>
