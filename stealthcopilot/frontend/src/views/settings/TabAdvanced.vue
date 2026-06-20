<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronUp, ChevronDown } from 'lucide-vue-next'

const { t } = useI18n()

const config = reactive({
  ragPrompt: '',
  speakPolishPrompt: '',
  polishEnabled: false,
  hearingAsrProvider: 'xunfei_simult',
  hearingTransProvider: 'xunfei_simult',
  hearingTtsProvider: 'system',
  speakingAsrProvider: 'xunfei_simult',
  speakingTransProvider: 'xunfei_simult',
  speakingTtsProvider: 'system',
  llmProvider: 'deepseek',
  llmBaseURL: 'https://api.deepseek.com/v1',
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
    config.hearingAsrProvider = cfg.hearing_asr_provider || 'xunfei_simult'
    config.hearingTransProvider = cfg.hearing_trans_provider || 'xunfei_simult'
    config.hearingTtsProvider = cfg.hearing_tts_provider || 'system'
    config.speakingAsrProvider = cfg.speaking_asr_provider || 'xunfei_simult'
    config.speakingTransProvider = cfg.speaking_trans_provider || 'xunfei_simult'
    config.speakingTtsProvider = cfg.speaking_tts_provider || 'system'
    config.llmProvider = cfg.llm_provider || 'deepseek'
    config.llmBaseURL = cfg.llm_base_url || 'https://api.deepseek.com/v1'
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
      hearing_asr_provider: config.hearingAsrProvider,
      hearing_trans_provider: config.hearingTransProvider,
      hearing_tts_provider: config.hearingTtsProvider,
      speaking_asr_provider: config.speakingAsrProvider,
      speaking_trans_provider: config.speakingTransProvider,
      speaking_tts_provider: config.speakingTtsProvider,
      llm_provider: config.llmProvider,
      llm_base_url: config.llmBaseURL,
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

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <h3 class="text-sm font-medium text-gray-200">
        {{ t('settings.advanced.hearingExtensions') }}
      </h3>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.hearingAsrProvider') }}</span>
          <select
            v-model="config.hearingAsrProvider"
            class="form-select"
          >
            <option value="xunfei_simult">{{ t('settings.advanced.providerNames.xunfeiRtasr') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.hearingTransProvider') }}</span>
          <select
            v-model="config.hearingTransProvider"
            class="form-select"
          >
            <option value="xunfei_simult">{{ t('settings.advanced.providerNames.xunfeiText') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.hearingTtsProvider') }}</span>
          <select
            v-model="config.hearingTtsProvider"
            class="form-select"
          >
            <option value="system">{{ t('settings.advanced.providerNames.systemMonitorTts') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.answerLlmProvider') }}</span>
          <select
            v-model="config.llmProvider"
            class="form-select"
          >
            <option value="deepseek">{{ t('settings.advanced.providerNames.deepseek') }}</option>
            <option value="openai_compatible">{{ t('settings.advanced.providerNames.openaiCompatible') }}</option>
          </select>
        </label>
      </div>
    </div>

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <h3 class="text-sm font-medium text-gray-200">
        {{ t('settings.advanced.speakingExtensions') }}
      </h3>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.speakingAsrProvider') }}</span>
          <select
            v-model="config.speakingAsrProvider"
            class="form-select"
          >
            <option value="xunfei_simult">{{ t('settings.advanced.providerNames.xunfeiSimult') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.speakingTransProvider') }}</span>
          <select
            v-model="config.speakingTransProvider"
            class="form-select"
          >
            <option value="xunfei_simult">{{ t('settings.advanced.providerNames.xunfeiSimult') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.sourceOnly') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.speakingTtsProvider') }}</span>
          <select
            v-model="config.speakingTtsProvider"
            class="form-select"
          >
            <option value="system">{{ t('settings.advanced.providerNames.systemTts') }}</option>
            <option value="xunfei_voiceclone">{{ t('settings.advanced.providerNames.xunfeiVoiceClone') }}</option>
            <option value="null">{{ t('settings.advanced.providerNames.null') }}</option>
          </select>
        </label>
        <div class="flex items-center justify-between bg-gray-900/40 rounded-lg px-4 py-3 border border-gray-700 md:col-span-2">
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
      </div>
    </div>

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <h3 class="text-sm font-medium text-gray-200">
        {{ t('settings.advanced.videoExtensions') }}
      </h3>
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
    </div>

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <h3 class="text-sm font-medium text-gray-200">
        {{ t('settings.advanced.resumeExtensions') }}
      </h3>
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

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <h3 class="text-sm font-medium text-gray-200">
        {{ t('settings.advanced.sharedLlmSettings') }}
      </h3>
      <label class="block">
        <span class="block text-xs text-gray-400 mb-1">{{ t('settings.advanced.llmBaseURL') }}</span>
        <input
          v-model="config.llmBaseURL"
          type="text"
          class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-blue-400"
        >
      </label>
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
