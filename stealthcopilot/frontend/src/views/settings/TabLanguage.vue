<script lang="ts" setup>
import { reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Headphones, Mic } from 'lucide-vue-next'

const { t } = useI18n()

// 讯飞支持的语言列表（常用子集）
const XUNFEI_LANGS = [
  { code: 'en', label: '英语 (English)' },
  { code: 'zh', label: '中文 (Chinese)' },
  { code: 'ja', label: '日语 (Japanese)' },
  { code: 'ko', label: '韩语 (Korean)' },
  { code: 'fr', label: '法语 (French)' },
  { code: 'de', label: '德语 (German)' },
  { code: 'es', label: '西班牙语 (Spanish)' },
  { code: 'ru', label: '俄语 (Russian)' },
  { code: 'ar', label: '阿拉伯语 (Arabic)' },
  { code: 'pt', label: '葡萄牙语 (Portuguese)' },
]

const config = reactive({
  hearingSource: 'en',
  hearingTarget: 'zh',
  speakingInput: 'zh',
  speakingOutput: 'en',
})
const saving = reactive({ hearing: false, speaking: false })
const msg = reactive({ hearing: '', speaking: '' })

onMounted(async () => {
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cfg = await window.go.main.App.GetConfig()
    config.hearingSource  = cfg.hearing_source_lang  || 'en'
    config.hearingTarget  = cfg.hearing_target_lang  || 'zh'
    config.speakingInput  = cfg.speaking_input_lang  || 'zh'
    config.speakingOutput = cfg.speaking_output_lang || 'en'
  } catch { /* 静默处理 */ }
})

async function saveHearing() {
  saving.hearing = true
  msg.hearing = ''
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cur = await window.go.main.App.GetConfig()
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const err = await window.go.main.App.SaveLocalConfig({
      ...cur,
      hearing_source_lang: config.hearingSource,
      hearing_target_lang: config.hearingTarget,
    })
    msg.hearing = err || t('common.success')
  } catch (e: unknown) { msg.hearing = String(e) }
  saving.hearing = false
}

async function saveSpeaking() {
  saving.speaking = true
  msg.speaking = ''
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cur = await window.go.main.App.GetConfig()
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const err = await window.go.main.App.SaveLocalConfig({
      ...cur,
      speaking_input_lang: config.speakingInput,
      speaking_output_lang: config.speakingOutput,
    })
    msg.speaking = err || t('common.success')
  } catch (e: unknown) { msg.speaking = String(e) }
  saving.speaking = false
}
</script>

<template>
  <div class="tab-language space-y-8">
    <h2 class="text-base font-semibold text-gray-200 mb-4">
      {{ t('settings.tabs.language') }}
    </h2>

    <!-- 听力链 -->
    <div class="chain-section bg-gray-800 rounded-xl p-5 border border-gray-700">
      <h3 class="font-semibold text-blue-300 mb-4 flex items-center gap-2">
        <Headphones :size="16" />{{ t('dashboard.hearingChain') }}
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs text-gray-400 mb-1">{{ t('setup.language.hearingSource') }}</label>
          <select
            v-model="config.hearingSource"
            class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-blue-400"
          >
            <option
              v-for="lang in XUNFEI_LANGS"
              :key="lang.code"
              :value="lang.code"
            >
              {{ lang.label }}
            </option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-gray-400 mb-1">{{ t('setup.language.hearingTarget') }}</label>
          <select
            v-model="config.hearingTarget"
            class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-blue-400"
          >
            <option
              v-for="lang in XUNFEI_LANGS"
              :key="lang.code"
              :value="lang.code"
            >
              {{ lang.label }}
            </option>
          </select>
        </div>
      </div>
      <div class="mt-3 flex items-center justify-between">
        <span
          v-if="msg.hearing"
          class="text-xs"
          :class="msg.hearing === t('common.success') ? 'text-green-400' : 'text-red-400'"
        >
          {{ msg.hearing }}
        </span>
        <span v-else />
        <button
          class="px-4 py-1.5 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm transition-colors"
          :disabled="saving.hearing"
          @click="saveHearing"
        >
          {{ t('common.save') }}
        </button>
      </div>
    </div>

    <!-- 说话链 -->
    <div class="chain-section bg-gray-800 rounded-xl p-5 border border-gray-700">
      <h3 class="font-semibold text-green-300 mb-4 flex items-center gap-2">
        <Mic :size="16" />{{ t('dashboard.speakingChain') }}
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs text-gray-400 mb-1">{{ t('setup.language.speakingInput') }}</label>
          <select
            v-model="config.speakingInput"
            class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-blue-400"
          >
            <option
              v-for="lang in XUNFEI_LANGS"
              :key="lang.code"
              :value="lang.code"
            >
              {{ lang.label }}
            </option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-gray-400 mb-1">{{ t('setup.language.speakingOutput') }}</label>
          <select
            v-model="config.speakingOutput"
            class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-blue-400"
          >
            <option
              v-for="lang in XUNFEI_LANGS"
              :key="lang.code"
              :value="lang.code"
            >
              {{ lang.label }}
            </option>
          </select>
        </div>
      </div>
      <div class="mt-3 flex items-center justify-between">
        <span
          v-if="msg.speaking"
          class="text-xs"
          :class="msg.speaking === t('common.success') ? 'text-green-400' : 'text-red-400'"
        >
          {{ msg.speaking }}
        </span>
        <span v-else />
        <button
          class="px-4 py-1.5 bg-green-500 hover:bg-green-600 rounded-lg text-sm transition-colors"
          :disabled="saving.speaking"
          @click="saveSpeaking"
        >
          {{ t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
