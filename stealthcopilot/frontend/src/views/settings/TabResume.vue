<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText } from 'lucide-vue-next'

const { t } = useI18n()

interface Resume {
  id: string
  name: string
  embedding_status: 'pending' | 'processing' | 'ready' | 'error'
  err_msg?: string
  created_at: string
  is_active: boolean
}

const resumes = ref<Resume[]>([])
const uploading = ref(false)
const errMsg = ref('')

async function loadResumes() {
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    resumes.value = await window.go.main.App.ListResumes() || []
  } catch { resumes.value = [] }
}

onMounted(() => {
  loadResumes()
  // 监听 embedding 状态变更事件（由 Go 后端 EventsEmit 推送）
  // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
  window.runtime?.EventsOn?.('resume:status_changed', (r: Resume) => {
    const idx = resumes.value.findIndex(x => x.id === r.id)
    if (idx >= 0) resumes.value[idx] = r
    else resumes.value.unshift(r)
  })
})

async function openFilePicker() {
  uploading.value = true
  errMsg.value = ''
  try {
    // 文件选择对话框必须由 Go 后端弹出（Wails v2 前端 runtime 不提供此 API）
    // @ts-expect-error — Wails 运行时注入
    const path: string = await window.go.main.App.PickResumeFile()
    if (!path) { uploading.value = false; return }
    // @ts-expect-error — Wails 运行时注入
    const err = await window.go.main.App.UploadResume(path)
    if (err) errMsg.value = err
    else await loadResumes()
  } catch (e: unknown) { errMsg.value = String(e) }
  uploading.value = false
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
    pending:    'settings.resume.statusPending',
    processing: 'settings.resume.statusProcessing',
    ready:      'settings.resume.statusReady',
    error:      'settings.resume.statusError',
  }
  return t(keyMap[s])
}
function statusColor(s: Resume['embedding_status']): string {
  return s === 'ready' ? 'text-green-400' : s === 'error' ? 'text-red-400' : 'text-yellow-400'
}
</script>

<template>
  <div class="tab-resume">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-base font-semibold text-gray-200">
        {{ t('settings.tabs.resume') }}
      </h2>
      <button
        class="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm font-semibold transition-colors"
        :disabled="uploading"
        @click="openFilePicker"
      >
        {{ uploading ? t('common.loading') : '+ ' + t('setup.resume.upload') }}
      </button>
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
            <p class="text-sm font-medium text-white truncate">
              {{ r.name }}
            </p>
            <p
              class="text-xs mt-0.5"
              :class="statusColor(r.embedding_status)"
            >
              {{ statusLabel(r.embedding_status) }}
              <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
              <span
                v-if="r.err_msg"
                class="ml-1 text-red-400"
              >— {{ r.err_msg }}</span>
              <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
            </p>
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
  </div>
</template>
