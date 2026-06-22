<script lang="ts" setup>
import { ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText } from 'lucide-vue-next'

const props = defineProps<{ isActive?: boolean }>()

const { t } = useI18n()

type ResumeLanguage = 'zh' | 'en' | 'ja' | 'ko' | 'fr' | 'de' | 'es' | 'other' | 'mixed'

interface Resume {
  id: string
  name: string
  resume_language: ResumeLanguage
  embedding_status: 'pending' | 'downloading' | 'processing' | 'ready' | 'error'
  err_msg?: string
  created_at: string
  is_active: boolean
}

const resumes = ref<Resume[]>([])
const uploading = ref(false)
const errMsg = ref('')

// 模型缓存状态（顶部状态栏）
interface ModelInfo { cached: boolean; cache_path: string }
const modelInfo = ref<ModelInfo | null>(null)

// 全局下载进度（下载时显示在状态栏中）
interface DownloadProgress { downloaded: number; total: number }
const globalDownload = ref<DownloadProgress | null>(null)
// 单卡片进度（保留供卡片级进度条使用）
const downloadProgress = ref<Record<string, DownloadProgress>>({})

// embedding chunk 进度：resumeID → { current, total }
interface EmbedProgress { current: number; total: number }
const embedProgress = ref<Record<string, EmbedProgress>>({})

// 上传语言确认 Modal
const showLangModal = ref(false)
const pendingFilePath = ref('')
const pendingLanguage = ref<ResumeLanguage>('mixed')

const languageOptions: { value: ResumeLanguage; label: string }[] = [
  { value: 'mixed', label: '多语言' },
  { value: 'zh',    label: '中文'   },
  { value: 'en',    label: '英文'   },
  { value: 'ja',    label: '日文'   },
  { value: 'ko',    label: '韩文'   },
  { value: 'fr',    label: '法文'   },
  { value: 'de',    label: '德文'   },
  { value: 'es',    label: '西文'   },
  { value: 'other', label: '其他'   },
]

async function loadResumes() {
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    resumes.value = await window.go.main.App.ListResumes() || []
  } catch { resumes.value = [] }
  // 还原正在处理中的简历的 chunk 进度（解决设置窗口重开后进度丢失的问题）
  for (const r of resumes.value) {
    if (r.embedding_status === 'processing') {
      try {
        // @ts-expect-error — Wails 运行时注入
        const p = await window.go.main.App.GetResumeEmbedProgress(r.id)
        if (p && p.total > 0) {
          embedProgress.value[r.id] = { current: p.current, total: p.total }
        }
      } catch { /* 忽略，等待下一个事件自动填充 */ }
    }
  }
}

// 切换回简历 tab 时重新拉取状态（进度条、模型状态等）
watch(() => props.isActive, (active) => {
  if (active) {
    loadResumes()
    loadModelInfo()
  }
})

async function loadModelInfo() {
  try {
    // @ts-expect-error — Wails 运行时注入
    modelInfo.value = await window.go.main.App.GetEmbeddingModelInfo()
  } catch { modelInfo.value = null }
}

onMounted(() => {
  loadResumes()
  loadModelInfo()
  // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
  window.runtime?.EventsOn?.('resume:status_changed', (r: Resume) => {
    const idx = resumes.value.findIndex(x => x.id === r.id)
    if (idx >= 0) resumes.value[idx] = r
    else resumes.value.unshift(r)
    if (r.embedding_status !== 'downloading') {
      delete downloadProgress.value[r.id]
    }
    if (r.embedding_status === 'ready' || r.embedding_status === 'error') {
      delete embedProgress.value[r.id]
    }
    // 下载完成后刷新状态栏
    if (r.embedding_status === 'processing') {
      globalDownload.value = null
      loadModelInfo()
    }
  })
  // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
  window.runtime?.EventsOn?.('resume:download_progress', (e: { id: string; downloaded: number; total: number }) => {
    downloadProgress.value[e.id] = { downloaded: e.downloaded, total: e.total }
    globalDownload.value = { downloaded: e.downloaded, total: e.total }
  })
  // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
  window.runtime?.EventsOn?.('resume:embed_progress', (e: { id: string; current: number; total: number }) => {
    embedProgress.value[e.id] = { current: e.current, total: e.total }
  })
})

async function openFilePicker() {
  errMsg.value = ''
  try {
    // 文件选择对话框必须由 Go 后端弹出（Wails v2 前端 runtime 不提供此 API）
    // @ts-expect-error — Wails 运行时注入
    const path: string = await window.go.main.App.PickResumeFile()
    if (!path) return
    // 文件选好后弹语言选择 Modal
    pendingFilePath.value = path
    pendingLanguage.value = 'mixed'
    showLangModal.value = true
  } catch (e: unknown) { errMsg.value = String(e) }
}

async function confirmUpload() {
  showLangModal.value = false
  uploading.value = true
  try {
    // @ts-expect-error — Wails 运行时注入
    const err = await window.go.main.App.UploadResumeWithLanguage(pendingFilePath.value, pendingLanguage.value)
    if (err) errMsg.value = err
    else await loadResumes()
  } catch (e: unknown) { errMsg.value = String(e) }
  uploading.value = false
}

function cancelUpload() {
  showLangModal.value = false
  pendingFilePath.value = ''
}

async function setActive(id: string) {
  // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
  const err = await window.go.main.App.SetActiveResume(id)
  if (!err) await loadResumes()
}

async function deleteResume(id: string) {
  // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
  const err = await window.go.main.App.DeleteResume(id)
  if (!err) resumes.value = resumes.value.filter(r => r.id !== id)
}

function statusLabel(s: Resume['embedding_status']): string {
  const keyMap: Record<Resume['embedding_status'], string> = {
    pending:     'settings.resume.statusPending',
    downloading: 'settings.resume.statusDownloading',
    processing:  'settings.resume.statusProcessing',
    ready:       'settings.resume.statusReady',
    error:       'settings.resume.statusError',
  }
  return t(keyMap[s] ?? 'settings.resume.statusPending')
}

function globalDownloadPercent(): number {
  const p = globalDownload.value
  if (!p || p.total <= 0) return 0
  return Math.min(100, Math.round(p.downloaded * 100 / p.total))
}

function globalDownloadLabel(): string {
  const p = globalDownload.value
  if (!p) return ''
  if (p.total <= 0) return `${(p.downloaded / 1024 / 1024).toFixed(0)} MB`
  return `${(p.downloaded / 1024 / 1024).toFixed(0)} / ${(p.total / 1024 / 1024).toFixed(0)} MB`
}

// 将完整路径中的 home 目录替换为 ~ 显示
function shortenPath(p: string): string {
  if (!p) return ''
  return p.replace(/^\/Users\/[^/]+/, '~').replace(/^C:\\Users\\[^\\]+/, '~')
}

function downloadPercent(id: string): number {
  const p = downloadProgress.value[id]
  if (!p || p.total <= 0) return 0
  return Math.min(100, Math.round(p.downloaded * 100 / p.total))
}

function downloadLabel(id: string): string {
  const p = downloadProgress.value[id]
  if (!p || p.total <= 0) return ''
  const dlMB = (p.downloaded / 1024 / 1024).toFixed(0)
  const totalMB = (p.total / 1024 / 1024).toFixed(0)
  return `${dlMB} / ${totalMB} MB`
}

function statusColor(s: Resume['embedding_status']): string {
  return s === 'ready' ? 'text-green-400' : s === 'error' ? 'text-red-400' : 'text-yellow-400'
}

function languageLabel(lang: ResumeLanguage | undefined): string {
  return languageOptions.find(o => o.value === (lang || 'mixed'))?.label ?? '多语言'
}

function langBadgeClass(lang: ResumeLanguage | undefined): string {
  const color: Record<ResumeLanguage, string> = {
    zh: 'bg-red-900/40 text-red-300',
    en: 'bg-blue-900/40 text-blue-300',
    ja: 'bg-purple-900/40 text-purple-300',
    ko: 'bg-pink-900/40 text-pink-300',
    fr: 'bg-indigo-900/40 text-indigo-300',
    de: 'bg-orange-900/40 text-orange-300',
    es: 'bg-yellow-900/40 text-yellow-300',
    other: 'bg-gray-700 text-gray-400',
    mixed: 'bg-gray-700 text-gray-400',
  }
  return color[lang || 'mixed'] ?? color.mixed
}

// 文件名从路径截取
function fileName(path: string): string {
  return path.split(/[\\/]/).pop() ?? path
}
</script>

<template>
  <div class="tab-resume">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-base font-semibold text-gray-200">
        {{ t('settings.tabs.resume') }}
      </h2>
      <button
        class="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm font-semibold transition-colors disabled:opacity-50"
        :disabled="uploading"
        @click="openFilePicker"
      >
        {{ uploading ? t('common.loading') : '+ ' + t('setup.resume.upload') }}
      </button>
    </div>

    <!-- 模型状态栏 -->
    <div
      v-if="modelInfo"
      class="flex items-center gap-3 rounded-xl px-4 py-3 mb-5 text-xs"
      :class="globalDownload
        ? 'bg-blue-950/60 border border-blue-800'
        : modelInfo.cached
          ? 'bg-gray-800 border border-gray-700'
          : 'bg-yellow-950/50 border border-yellow-800/60'"
    >
      <!-- 下载进度中 -->
      <template v-if="globalDownload">
        <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
        <span class="text-blue-400 shrink-0">↓</span>
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between mb-1.5">
            <span class="text-blue-300">{{ t('settings.resume.statusDownloading') }}</span>
            <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
            <span class="text-gray-400">{{ globalDownloadLabel() }}</span>
            <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
          </div>
          <div class="h-1 bg-gray-700 rounded-full overflow-hidden">
            <div
              class="h-full bg-blue-500 rounded-full transition-all duration-300"
              :style="{ width: (globalDownloadPercent() || 2) + '%' }"
            />
          </div>
        </div>
      </template>

      <!-- 已缓存 -->
      <template v-else-if="modelInfo.cached">
        <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
        <span class="text-green-400 shrink-0">✓</span>
        <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
        <span class="text-gray-400 truncate">模型已就绪 · <span class="text-gray-500 font-mono">{{ shortenPath(modelInfo.cache_path) }}</span></span>
        <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
      </template>

      <!-- 未下载 -->
      <template v-else>
        <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
        <span class="text-yellow-400 shrink-0">⚠</span>
        <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
        <span class="text-yellow-300/80">模型未下载，首次上传简历时将自动开始下载（约 470 MB）</span>
        <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
      </template>
    </div>

    <p
      v-if="errMsg"
      class="text-red-400 text-sm mb-4"
    >
      {{ errMsg }}
    </p>

    <!-- 空状态 -->
    <div
      v-if="resumes.length === 0"
      class="empty-state border-2 border-dashed border-gray-600 rounded-xl p-10 text-center cursor-pointer
             hover:border-blue-500 transition-colors"
      @click="openFilePicker"
    >
      <div class="flex justify-center mb-3 text-gray-500">
        <FileText :size="40" />
      </div>
      <p class="text-gray-400 text-sm">
        {{ t('settings.resume.empty') }}
      </p>
      <p class="text-gray-600 text-xs mt-1">
        {{ t('setup.resume.formats') }}
      </p>
    </div>

    <!-- 简历列表 -->
    <div
      v-else
      class="resume-list space-y-3"
    >
      <div
        v-for="r in resumes"
        :key="r.id"
        class="resume-card flex items-center justify-between bg-gray-800 rounded-xl px-5 py-4 border transition-colors"
        :class="r.is_active ? 'border-blue-500' : 'border-gray-700'"
      >
        <div class="flex items-center gap-3 min-w-0">
          <FileText
            :size="20"
            class="flex-shrink-0 text-gray-400"
          />
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <p class="text-sm font-medium text-white truncate">
                {{ r.name }}
              </p>
              <span
                class="px-1.5 py-0.5 text-xs rounded flex-shrink-0"
                :class="langBadgeClass(r.resume_language)"
              >{{ languageLabel(r.resume_language) }}</span>
            </div>
            <p
              class="text-xs mt-0.5 flex items-center gap-1.5"
              :class="statusColor(r.embedding_status)"
            >
              <!-- 活动状态的跳动小点 -->
              <span
                v-if="r.embedding_status === 'downloading' || r.embedding_status === 'processing'"
                class="inline-block w-1.5 h-1.5 rounded-full bg-current animate-pulse shrink-0"
              />
              {{ statusLabel(r.embedding_status) }}
              <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
              <span
                v-if="r.embedding_status === 'downloading' && downloadLabel(r.id)"
                class="text-gray-400"
              >{{ downloadLabel(r.id) }}</span>
              <span
                v-if="r.embedding_status === 'processing' && embedProgress[r.id]"
                class="text-gray-500"
              >{{ embedProgress[r.id].current }} / {{ embedProgress[r.id].total }} 段</span>
              <span
                v-if="r.err_msg"
                class="text-red-400"
              >— {{ r.err_msg }}</span>
              <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
            </p>
            <!-- 模型下载进度条 -->
            <div
              v-if="r.embedding_status === 'downloading'"
              class="mt-1.5 h-1 w-48 bg-gray-700 rounded-full overflow-hidden"
            >
              <div
                class="h-full bg-blue-500 rounded-full transition-all duration-300"
                :style="{ width: (downloadPercent(r.id) || 2) + '%' }"
              />
            </div>
            <!-- embedding chunk 进度条 -->
            <div
              v-if="r.embedding_status === 'processing' && embedProgress[r.id]"
              class="mt-1.5 h-1 w-48 bg-gray-700 rounded-full overflow-hidden"
            >
              <div
                class="h-full bg-yellow-500 rounded-full transition-all duration-300"
                :style="{ width: (embedProgress[r.id].current / embedProgress[r.id].total * 100) + '%' }"
              />
            </div>
          </div>
          <span
            v-if="r.is_active"
            class="ml-2 px-2 py-0.5 text-xs bg-blue-600/30 text-blue-300 rounded-full flex-shrink-0"
          >{{ t('settings.resume.activeLabel') }}</span>
        </div>

        <div class="flex items-center gap-2 flex-shrink-0 ml-3">
          <button
            v-if="!r.is_active"
            class="px-3 py-1.5 text-xs bg-gray-700 hover:bg-blue-600 rounded-lg transition-colors"
            @click="setActive(r.id)"
          >
            {{ t('settings.resume.activate') }}
          </button>
          <button
            class="px-3 py-1.5 text-xs bg-gray-700 hover:bg-red-600 rounded-lg transition-colors text-gray-400 hover:text-white"
            @click="deleteResume(r.id)"
          >
            {{ t('common.delete') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 语言选择 Modal（上传后、入库前弹出） -->
    <Teleport to="body">
      <div
        v-if="showLangModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
        @click.self="cancelUpload"
      >
        <div class="bg-gray-900 border border-gray-700 rounded-2xl p-6 w-[360px] shadow-2xl">
          <h3 class="text-sm font-semibold text-gray-200 mb-1">
            {{ t('settings.resume.langModal.title') }}
          </h3>
          <p class="text-xs text-gray-500 mb-4 truncate">
            {{ fileName(pendingFilePath) }}
          </p>

          <div class="grid grid-cols-3 gap-2 mb-6">
            <button
              v-for="opt in languageOptions"
              :key="opt.value"
              class="py-2 rounded-lg text-xs font-medium transition-colors border"
              :class="pendingLanguage === opt.value
                ? 'bg-blue-600 border-blue-500 text-white'
                : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-500 hover:text-gray-200'"
              @click="pendingLanguage = opt.value"
            >
              {{ opt.label }}
            </button>
          </div>

          <div class="flex gap-3">
            <button
              class="flex-1 py-2 rounded-lg text-sm bg-gray-800 hover:bg-gray-700 text-gray-400 transition-colors"
              @click="cancelUpload"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="flex-1 py-2 rounded-lg text-sm bg-blue-600 hover:bg-blue-500 text-white font-semibold transition-colors"
              @click="confirmUpload"
            >
              {{ t('settings.resume.langModal.confirm') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
