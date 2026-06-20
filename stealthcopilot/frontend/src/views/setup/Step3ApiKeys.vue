<script lang="ts" setup>
import { reactive, ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Eye, EyeOff } from 'lucide-vue-next'

const { t } = useI18n()

interface KeyField {
  service: string
  field: string
  label: string
  required: boolean
  value: string
  show: boolean
}

// 必填：讯飞同声传译、DeepSeek；可选：讯飞声音复刻、Simli
const fields = reactive<KeyField[]>([
  { service: 'xunfei_simult', field: 'app_id', label: t('settings.apiKeys.xunfei.simultAppId'), required: true, value: '', show: false },
  { service: 'xunfei_simult', field: 'api_key', label: t('settings.apiKeys.xunfei.simultApiKey'), required: true, value: '', show: false },
  { service: 'xunfei_simult', field: 'api_secret', label: t('settings.apiKeys.xunfei.simultApiSecret'), required: true, value: '', show: false },
  { service: 'xunfei_tts', field: 'app_id', label: t('settings.apiKeys.xunfei.ttsAppId'), required: false, value: '', show: false },
  { service: 'xunfei_tts', field: 'api_key', label: t('settings.apiKeys.xunfei.ttsApiKey'), required: false, value: '', show: false },
  { service: 'xunfei_tts', field: 'api_secret', label: t('settings.apiKeys.xunfei.ttsApiSecret'), required: false, value: '', show: false },
  { service: 'deepseek', field: 'key', label: t('settings.apiKeys.deepseek.key'), required: true, value: '', show: false },
  { service: 'simli', field: 'key', label: t('settings.apiKeys.simli.key'), required: false, value: '', show: false },
])

const saving = ref(false)
const saveMsg = ref('')

// 必填项全部有值才能继续
const canProceed = computed(() =>
  fields.filter(f => f.required).every(f => f.value.trim() !== '')
)

async function save(field: KeyField) {
  if (!field.value.trim()) return
  saving.value = true
  saveMsg.value = ''
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const err = await window.go.main.App.SaveAPIKey({
      service: field.service,
      field: field.field,
      value: field.value.trim(),
    })
    saveMsg.value = err ? err : ''
  } catch (e: unknown) {
    saveMsg.value = String(e)
  }
  saving.value = false
}

async function saveAll() {
  for (const f of fields) {
    if (f.value.trim()) await save(f)
  }
}
</script>

<template>
  <div class="step3">
    <h2 class="text-xl font-bold mb-2 text-white">
      {{ t('setup.apiKeys.title') }}
    </h2>
    <p class="text-gray-400 mb-6 text-sm">
      {{ t('setup.apiKeys.desc') }}
    </p>

    <div class="fields space-y-3">
      <div
        v-for="f in fields"
        :key="f.service + '_' + f.field"
        class="field-row flex items-center gap-3"
      >
        <label class="w-32 shrink-0 text-sm text-gray-300 text-left">
          {{ f.label }}
          <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
          <span
            v-if="f.required"
            class="text-red-400 ml-1"
          >*</span>
          <span
            v-else
            class="text-gray-500 text-xs ml-1"
          >（{{ t('setup.apiKeys.optional') }}）</span>
          <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
        </label>
        <div class="flex flex-1 gap-2">
          <input
            v-model="f.value"
            :type="f.show ? 'text' : 'password'"
            class="flex-1 bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-blue-400 transition-colors"
            :placeholder="t('setup.apiKeys.placeholder')"
          >
          <button
            class="px-3 py-2 bg-gray-600 hover:bg-gray-500 rounded-lg transition-colors flex items-center"
            @click="f.show = !f.show"
          >
            <component
              :is="f.show ? EyeOff : Eye"
              :size="14"
            />
          </button>
        </div>
      </div>
    </div>

    <p
      v-if="saveMsg"
      class="mt-3 text-red-400 text-sm"
    >
      {{ saveMsg }}
    </p>

    <div class="mt-6 flex items-center justify-between">
      <span class="text-xs text-gray-500">{{ t('setup.apiKeys.secureNote') }}</span>
      <button
        class="px-5 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm font-semibold transition-colors
               disabled:opacity-40 disabled:cursor-not-allowed"
        :disabled="!canProceed || saving"
        @click="saveAll"
      >
        {{ saving ? t('common.loading') : t('common.save') }}
      </button>
    </div>
  </div>
</template>
