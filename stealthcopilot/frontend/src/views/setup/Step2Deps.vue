<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

type DepStatus = 'installed' | 'missing' | 'unknown' | 'checking' | 'installing' | 'failed'

interface DepItem {
  key: string
  label: string
  status: DepStatus
  installMsg: string // 安装操作后的提示消息（空字符串表示无提示）
}

const deps = ref<DepItem[]>([
  { key: 'ffmpeg',      label: t('setup.deps.ffmpeg'),      status: 'checking', installMsg: '' },
  { key: 'virtual_mic', label: t('setup.deps.virtualMic'), status: 'checking', installMsg: '' },
  { key: 'virtual_cam', label: t('setup.deps.virtualCam'), status: 'checking', installMsg: '' },
])

async function checkDeps() {
  deps.value.forEach(d => { d.status = 'checking' })
  try {
    // @ts-expect-error — Wails 运行时注入
    const report = await window.go.main.App.CheckDeps()
    deps.value[0].status = report.ffmpeg      as DepStatus
    deps.value[1].status = report.virtual_mic as DepStatus
    deps.value[2].status = report.virtual_cam as DepStatus
  } catch {
    deps.value.forEach(d => { d.status = 'unknown' })
  }
}

async function install(dep: DepItem) {
  dep.installMsg = ''
  try {
    // @ts-expect-error — Wails 运行时注入
    const result = await window.go.main.App.InstallDep(dep.key) as { auto_installed: boolean; message: string }
    dep.installMsg = result?.message ?? ''
  } catch {
    dep.status = 'failed'
  }
}

function statusIcon(status: DepStatus): string {
  const icons: Record<DepStatus, string> = {
    installed: '✅',
    missing: '❌',
    unknown: '❓',
    checking: '⏳',
    installing: '⏳',
    failed: '❌',
  }
  return icons[status] ?? '❓'
}

function statusClass(status: DepStatus): string {
  if (status === 'installed') return 'text-green-400'
  if (status === 'checking' || status === 'installing') return 'text-yellow-400'
  return 'text-red-400'
}

onMounted(checkDeps)
</script>

<template>
  <div class="step2">
    <h2 class="text-xl font-bold mb-2 text-white">
      {{ t('setup.deps.title') }}
    </h2>
    <p class="text-gray-400 mb-6 text-sm">
      {{ t('setup.deps.desc') }}
    </p>

    <div class="dep-list space-y-4">
      <div
        v-for="dep in deps"
        :key="dep.key"
        class="dep-item bg-gray-700 rounded-xl px-5 py-4"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <span class="text-xl">{{ statusIcon(dep.status) }}</span>
            <span class="font-medium">{{ dep.label }}</span>
          </div>

          <div class="flex items-center gap-3">
            <span
              class="text-sm"
              :class="statusClass(dep.status)"
            >
              <template v-if="dep.status === 'installed'">{{ t('setup.deps.installed') }}</template>
              <template v-else-if="dep.status === 'checking'">
                {{ t('common.loading') }}
              </template>
              <template v-else-if="dep.status === 'failed'">{{ t('setup.deps.failed') }}</template>
              <template v-else>{{ t('setup.deps.missing') }}</template>
            </span>
            <button
              v-if="dep.status === 'missing' || dep.status === 'failed'"
              class="px-4 py-1.5 text-sm bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors"
              @click="install(dep)"
            >
              {{ t('setup.deps.installGuide') }}
            </button>
          </div>
        </div>

        <!-- 安装操作完成后显示的引导提示 -->
        <p
          v-if="dep.installMsg"
          class="mt-2 text-xs text-yellow-300 leading-relaxed"
        >
          {{ dep.installMsg }}
        </p>
      </div>
    </div>

    <div class="mt-6 flex justify-end">
      <button
        class="text-sm text-gray-500 hover:text-gray-300 underline transition-colors"
        @click="checkDeps"
      >
        {{ t('setup.deps.recheck') }}
      </button>
    </div>
  </div>
</template>
