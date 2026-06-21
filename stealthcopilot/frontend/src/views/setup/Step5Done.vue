<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { PartyPopper, CheckCircle, Square } from 'lucide-vue-next'

const { t } = useI18n()

interface CheckItem {
  label: string
  done: boolean
}

const items = ref<CheckItem[]>([])

onMounted(async () => {
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cfg = await window.go.main.App.GetConfig()
    items.value = [
      { label: t('setup.done.xunfei'), done: cfg.xunfei_simult_app_id_set && cfg.xunfei_simult_api_key_set && cfg.xunfei_simult_api_secret_set },
      { label: t('setup.done.deepseek'), done: cfg.deepseek_key_set },
      { label: t('setup.done.defaultVoice'), done: true },
      { label: t('setup.done.personalVoice'), done: cfg.xunfei_tts_asset_id_set },
      { label: t('setup.done.simli'), done: cfg.simli_key_set },
    ]
  } catch {
    items.value = []
  }
})
</script>

<template>
  <div class="step5 text-center">
    <div class="flex justify-center mb-4 text-yellow-400">
      <PartyPopper :size="60" />
    </div>
    <h2 class="text-2xl font-bold mb-2 text-white">
      {{ t('setup.done.title') }}
    </h2>
    <p class="text-gray-400 mb-6 text-sm">
      {{ t('setup.done.desc') }}
    </p>

    <!-- 已完成项汇总 -->
    <div class="summary bg-gray-700 rounded-xl p-5 text-left space-y-2 mb-6">
      <h3 class="text-sm font-semibold text-gray-300 mb-3">
        {{ t('setup.done.summary') }}
      </h3>
      <div
        v-for="(item, idx) in items"
        :key="idx"
        class="flex items-center gap-3 text-sm"
      >
        <component
          :is="item.done ? CheckCircle : Square"
          :size="16"
          :class="item.done ? 'text-green-400' : 'text-gray-500'"
        />
        <span :class="item.done ? 'text-white' : 'text-gray-500'">{{ item.label }}</span>
      </div>
    </div>

    <p class="text-xs text-gray-500">
      {{ t('setup.done.hint') }}
    </p>
  </div>
</template>
