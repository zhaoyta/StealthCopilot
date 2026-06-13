<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const POSITIONS = [
  { value: 'top-left',     label: t('settings.ghost.positions.topLeft') },
  { value: 'top-right',    label: t('settings.ghost.positions.topRight') },
  { value: 'bottom-left',  label: t('settings.ghost.positions.bottomLeft') },
  { value: 'bottom-right', label: t('settings.ghost.positions.bottomRight') },
  { value: 'center',       label: t('settings.ghost.positions.center') },
]

const config = reactive({ fontSize: 16, opacity: 0.85, position: 'bottom-right' })
const saving = ref(false)
const msg = ref('')

onMounted(async () => {
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cfg = await window.go.main.App.GetConfig()
    config.fontSize = cfg.ghost_font_size || 16
    config.opacity  = cfg.ghost_opacity   || 0.85
    config.position = cfg.ghost_position  || 'bottom-right'
  } catch { /* 静默处理 */ }
})

async function save() {
  saving.value = true
  msg.value = ''
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cur = await window.go.main.App.GetConfig()
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const err = await window.go.main.App.SaveLocalConfig({
      ...cur,
      ghost_font_size: config.fontSize,
      ghost_opacity:   config.opacity,
      ghost_position:  config.position,
    })
    msg.value = err || t('common.success')
  } catch (e: unknown) { msg.value = String(e) }
  saving.value = false
}
</script>

<template>
  <div class="tab-ghost space-y-6">
    <h2 class="text-base font-semibold text-gray-200 mb-4">
      {{ t('settings.tabs.ghost') }}
    </h2>

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-6">
      <!-- 字号 -->
      <div>
        <div class="flex items-center justify-between mb-2">
          <label class="text-sm text-gray-300">{{ t('settings.ghost.fontSize') }}</label>
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="text-sm text-blue-300 font-mono">{{ config.fontSize }}px</span>
        </div>
        <input
          v-model.number="config.fontSize"
          type="range"
          min="12"
          max="32"
          step="1"
          class="w-full h-2 bg-gray-600 rounded-lg appearance-none cursor-pointer accent-blue-400"
        >
        <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
        <div class="flex justify-between text-xs text-gray-500 mt-1">
          <span>12px</span><span>32px</span>
        </div>
        <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
      </div>

      <!-- 透明度 -->
      <div>
        <div class="flex items-center justify-between mb-2">
          <label class="text-sm text-gray-300">{{ t('settings.ghost.opacity') }}</label>
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="text-sm text-blue-300 font-mono">{{ Math.round(config.opacity * 100) }}%</span>
        </div>
        <input
          v-model.number="config.opacity"
          type="range"
          min="0.1"
          max="1.0"
          step="0.05"
          class="w-full h-2 bg-gray-600 rounded-lg appearance-none cursor-pointer accent-blue-400"
        >
        <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
        <div class="flex justify-between text-xs text-gray-500 mt-1">
          <span>10%</span><span>100%</span>
        </div>
        <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
      </div>

      <!-- 位置预设 -->
      <div>
        <label class="block text-sm text-gray-300 mb-2">{{ t('settings.ghost.position') }}</label>
        <div class="grid grid-cols-3 gap-2">
          <button
            v-for="pos in POSITIONS"
            :key="pos.value"
            class="py-2 text-xs rounded-lg border transition-colors"
            :class="config.position === pos.value
              ? 'bg-blue-600/30 border-blue-400 text-blue-200'
              : 'bg-gray-700 border-gray-600 text-gray-400 hover:border-gray-400'"
            @click="config.position = pos.value"
          >
            {{ pos.label }}
          </button>
        </div>
      </div>
    </div>

    <!-- 实时预览 -->
    <div class="preview-area relative bg-gray-900/60 border border-dashed border-gray-600 rounded-xl h-32 overflow-hidden">
      <p class="absolute top-1 left-2 text-xs text-gray-600">
        {{ t('settings.ghost.preview') }}
      </p>
      <div
        class="absolute bg-gray-800/80 rounded-lg px-4 py-2 max-w-48 text-left transition-all duration-200"
        :style="{
          fontSize: config.fontSize + 'px',
          opacity: config.opacity,
          ...(config.position.includes('top') ? { top: '8px' } : { bottom: '8px' }),
          ...(config.position.includes('left') ? { left: '8px' } : config.position.includes('right') ? { right: '8px' } : { left: '50%', transform: 'translateX(-50%)' }),
        }"
      >
        <p class="text-gray-400 text-xs">
          {{ t('settings.ghost.previewSubtitle') }}
        </p>
        <p class="text-white mt-0.5">
          {{ t('settings.ghost.previewAnswer') }}
        </p>
      </div>
    </div>

    <div class="flex items-center justify-between">
      <span
        v-if="msg"
        class="text-xs"
        :class="msg === t('common.success') ? 'text-green-400' : 'text-red-400'"
      >{{ msg }}</span>
      <span v-else />
      <button
        class="px-5 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm transition-colors"
        :disabled="saving"
        @click="save"
      >
        {{ t('common.save') }}
      </button>
    </div>
  </div>
</template>
