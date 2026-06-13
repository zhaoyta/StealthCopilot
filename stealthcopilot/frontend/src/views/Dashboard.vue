<script lang="ts" setup>
import { useI18n } from 'vue-i18n'

// 主界面占位组件，将在 hearing-chain / speaking-chain / video-chain change 中完整实现
defineOptions({ name: 'AppDashboard' })
const { t } = useI18n()
defineEmits<{
  (e: 'openSettings'): void
  (e: 'openTeleprompter'): void
}>()
</script>

<template>
  <div class="dashboard flex flex-col items-center justify-center min-h-screen bg-gray-900 text-white">
    <h1 class="text-4xl font-bold text-blue-400 mb-4">
      {{ t('dashboard.title') }}
    </h1>
    <p class="text-gray-400 text-sm">
      {{ t('dashboard.placeholder') }}
    </p>
    <div class="flex items-center gap-3 mt-8">
      <button
        class="px-4 py-2 rounded-lg bg-blue-500 hover:bg-blue-600 text-sm transition-colors"
        @click="$emit('openTeleprompter')"
      >
        {{ t('dashboard.openTeleprompter') }}
      </button>
      <button
        class="px-4 py-2 rounded-lg bg-gray-700 hover:bg-gray-600 text-sm transition-colors"
        @click="$emit('openSettings')"
      >
        {{ t('dashboard.openSettings') }}
      </button>
    </div>
  </div>
</template>
