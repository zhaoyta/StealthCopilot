<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronUp, ChevronDown } from 'lucide-vue-next'

const { t } = useI18n()

const config = reactive({
  ragPrompt: '',
  speakPolishPrompt: '',
  polishEnabled: false,
  translationProvider: 'xunfei',
  llmProvider: 'deepseek',
  llmBaseURL: 'https://api.deepseek.com/v1',
  ttsProvider: 'xunfei_voiceclone',
  lipsyncProvider: 'simli',
  embeddingProvider: 'python_bridge',
})
const defaults = reactive({ ragPrompt: '', speakPolishPrompt: '' })
const expanded = reactive({ rag: false, speak: false })
const saving = ref(false)
const msg = ref('')

onMounted(async () => {
  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const w = window as any
    const [cfg, defs] = await Promise.all([
      w.go.main.App.GetConfig(),
      w.go.main.App.GetDefaultPrompts(),
    ])
    config.ragPrompt = cfg.rag_prompt || ''
    config.speakPolishPrompt = cfg.speak_polish_prompt || ''
    config.polishEnabled = cfg.polish_enabled || false
    config.translationProvider = cfg.translation_provider || 'xunfei'
    config.llmProvider = cfg.llm_provider || 'deepseek'
    config.llmBaseURL = cfg.llm_base_url || 'https://api.deepseek.com/v1'
    config.ttsProvider = cfg.tts_provider || 'xunfei_voiceclone'
    config.lipsyncProvider = cfg.lipsync_provider || 'simli'
    config.embeddingProvider = cfg.embedding_provider || 'python_bridge'
    defaults.ragPrompt = defs.rag_prompt
    defaults.speakPolishPrompt = defs.speak_polish_prompt
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
      rag_prompt: config.ragPrompt,
      speak_polish_prompt: config.speakPolishPrompt,
      polish_enabled: config.polishEnabled,
      translation_provider: config.translationProvider,
      llm_provider: config.llmProvider,
      llm_base_url: config.llmBaseURL,
      tts_provider: config.ttsProvider,
      lipsync_provider: config.lipsyncProvider,
      embedding_provider: config.embeddingProvider,
    })
    msg.value = err || t('common.success')
  } catch (e: unknown) { msg.value = String(e) }
  saving.value = false
}
</script>

<template>
  <div class="tab-advanced space-y-5">
    <h2 class="text-base font-semibold text-gray-200 mb-4">
      {{ t('settings.tabs.advanced') }}
    </h2>

    <!-- 说话润色开关 -->
    <div class="flex items-center justify-between bg-gray-800 rounded-xl px-5 py-4 border border-gray-700">
      <label class="text-sm text-gray-200">{{ t('settings.advanced.polishEnabled') }}</label>
      <button
        class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
        :class="config.polishEnabled ? 'bg-blue-500' : 'bg-gray-600'"
        @click="config.polishEnabled = !config.polishEnabled"
      >
        <span
          class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
          :class="config.polishEnabled ? 'translate-x-6' : 'translate-x-1'"
        />
      </button>
    </div>

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <h3 class="text-sm font-medium text-gray-200">
        {{ t('settings.advanced.providers') }}
      </h3>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.translationProvider') }}</span>
          <select
            v-model="config.translationProvider"
            class="form-select"
          >
            <option value="xunfei">{{ t('settings.advanced.providerNames.xunfei') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.llmProvider') }}</span>
          <select
            v-model="config.llmProvider"
            class="form-select"
          >
            <option value="deepseek">{{ t('settings.advanced.providerNames.deepseek') }}</option>
            <option value="openai_compatible">{{ t('settings.advanced.providerNames.openaiCompatible') }}</option>
          </select>
        </label>
        <label class="block md:col-span-2">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.llmBaseURL') }}</span>
          <input
            v-model="config.llmBaseURL"
            type="text"
            class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-blue-400"
          >
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.ttsProvider') }}</span>
          <select
            v-model="config.ttsProvider"
            class="form-select"
          >
            <option value="xunfei_voiceclone">{{ t('settings.advanced.providerNames.xunfeiVoiceClone') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.lipsyncProvider') }}</span>
          <select
            v-model="config.lipsyncProvider"
            class="form-select"
          >
            <option value="simli">{{ t('settings.advanced.providerNames.simli') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.embeddingProvider') }}</span>
          <select
            v-model="config.embeddingProvider"
            class="form-select"
          >
            <option value="python_bridge">{{ t('settings.advanced.providerNames.pythonBridge') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
      </div>
    </div>

    <!-- RAG 回答生成 Prompt -->
    <div class="bg-gray-800 rounded-xl border border-gray-700">
      <button
        class="w-full flex items-center justify-between px-5 py-4 text-sm font-medium text-gray-200"
        @click="expanded.rag = !expanded.rag"
      >
        <span>{{ t('settings.advanced.ragPrompt') }}</span>
        <component
          :is="expanded.rag ? ChevronUp : ChevronDown"
          :size="14"
          class="text-gray-400"
        />
      </button>
      <div
        v-if="expanded.rag"
        class="px-5 pb-4 space-y-2"
      >
        <textarea
          v-model="config.ragPrompt"
          rows="8"
          class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-blue-400 font-mono resize-y"
        />
        <button
          class="text-xs text-gray-400 hover:text-white underline transition-colors"
          @click="config.ragPrompt = defaults.ragPrompt"
        >
          {{ t('common.reset') }}
        </button>
      </div>
    </div>

    <!-- 说话润色 Prompt -->
    <div class="bg-gray-800 rounded-xl border border-gray-700">
      <button
        class="w-full flex items-center justify-between px-5 py-4 text-sm font-medium text-gray-200"
        @click="expanded.speak = !expanded.speak"
      >
        <span>{{ t('settings.advanced.speakPolishPrompt') }}</span>
        <component
          :is="expanded.speak ? ChevronUp : ChevronDown"
          :size="14"
          class="text-gray-400"
        />
      </button>
      <div
        v-if="expanded.speak"
        class="px-5 pb-4 space-y-2"
      >
        <textarea
          v-model="config.speakPolishPrompt"
          rows="8"
          class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-blue-400 font-mono resize-y"
        />
        <button
          class="text-xs text-gray-400 hover:text-white underline transition-colors"
          @click="config.speakPolishPrompt = defaults.speakPolishPrompt"
        >
          {{ t('common.reset') }}
        </button>
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
