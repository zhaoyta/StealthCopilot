<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Eye, EyeOff } from 'lucide-vue-next'

const { t } = useI18n()
const MASKED_VALUE = '••••••••'

type TestStatus = 'untested' | 'testing' | 'ok' | 'fail'

interface ServiceConfig {
  name: string
  docs?: {
    label: string
    url: string
  }[]
  fields: {
    service: string
    field: string
    label: string
    secret: boolean
    value: string
    show: boolean
  }[]
  testStatus: TestStatus
  testMsg: string
}

const services = reactive<ServiceConfig[]>([
  {
    name: '讯飞实时转写、同声传译与机器翻译',
    docs: [
      { label: t('settings.apiKeys.docs.xunfeiRtasr'), url: 'https://www.xfyun.cn/doc/spark/asr_llm/rtasr_llm.html' },
      { label: t('settings.apiKeys.docs.xunfeiSimult'), url: 'https://www.xfyun.cn/doc/nlp/simultaneous-interpretation/API.html#%E6%8E%A5%E5%8F%A3%E8%AF%B4%E6%98%8E' },
      { label: t('settings.apiKeys.docs.xunfeiText'), url: 'https://www.xfyun.cn/doc/nlp/xftrans_new/API.html' },
    ],
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'xunfei_simult', field: 'app_id',     label: t('settings.apiKeys.xunfei.simultAppId'),     secret: true, value: '', show: false },
      { service: 'xunfei_simult', field: 'api_key',    label: t('settings.apiKeys.xunfei.simultApiKey'),    secret: true, value: '', show: false },
      { service: 'xunfei_simult', field: 'api_secret', label: t('settings.apiKeys.xunfei.simultApiSecret'), secret: true, value: '', show: false },
    ],
  },
  {
    name: '讯飞声音复刻',
    docs: [
      { label: t('settings.apiKeys.docs.xunfeiVoiceClone'), url: 'https://www.xfyun.cn/doc/spark/voiceclone.html#%E6%9C%8D%E5%8A%A1%E4%BB%8B%E7%BB%8D' },
    ],
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'xunfei_tts', field: 'app_id',     label: t('settings.apiKeys.xunfei.ttsAppId'),     secret: true,  value: '', show: false },
      { service: 'xunfei_tts', field: 'api_key',    label: t('settings.apiKeys.xunfei.ttsApiKey'),    secret: true,  value: '', show: false },
      { service: 'xunfei_tts', field: 'api_secret', label: t('settings.apiKeys.xunfei.ttsApiSecret'), secret: true,  value: '', show: false },
    ],
  },
  {
    name: 'DeepSeek',
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'deepseek', field: 'key',   label: t('settings.apiKeys.deepseek.key'),   secret: true,  value: '', show: false },
      { service: 'deepseek', field: 'model', label: t('settings.apiKeys.deepseek.model'), secret: false, value: '', show: false },
    ],
  },
  {
    name: 'Simli AI',
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'simli', field: 'key',     label: t('settings.apiKeys.simli.key'),    secret: true,  value: '', show: false },
      { service: 'simli', field: 'face_id', label: t('settings.apiKeys.simli.faceId'), secret: false, value: '', show: false },
    ],
  },
])

const saving = ref(false)

onMounted(async () => {
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cfg = await window.go.main.App.GetConfig()
    const setMap: Record<string, boolean> = {
      xunfei_simult_app_id: cfg.xunfei_simult_app_id_set,
      xunfei_simult_api_key: cfg.xunfei_simult_api_key_set,
      xunfei_simult_api_secret: cfg.xunfei_simult_api_secret_set,
      xunfei_tts_app_id: cfg.xunfei_tts_app_id_set,
      xunfei_tts_api_key: cfg.xunfei_tts_api_key_set,
      xunfei_tts_api_secret: cfg.xunfei_tts_api_secret_set,
      deepseek_key: cfg.deepseek_key_set,
      deepseek_model: !!cfg.deepseek_model,
      simli_key: cfg.simli_key_set,
      simli_face_id: cfg.simli_face_id_set,
    }
    for (const svc of services) {
      for (const f of svc.fields) {
        const k = `${f.service}_${f.field}`
        if (setMap[k]) f.value = MASKED_VALUE
      }
      if (svc.fields[0].service === 'deepseek') {
        svc.fields[1].value = cfg.deepseek_model || ''
      }
    }
  } catch { /* 加载失败静默处理 */ }
})

async function saveService(svc: ServiceConfig) {
  saving.value = true
  svc.testStatus = 'untested'
  svc.testMsg = ''
  try {
    for (const f of svc.fields) {
      if (!f.value || f.value === MASKED_VALUE) continue
      let err = ''
      if (f.service === 'deepseek' && f.field === 'model') {
        // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
        const cur = await window.go.main.App.GetConfig()
        // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
        err = await window.go.main.App.SaveLocalConfig({ ...cur, deepseek_model: f.value })
      } else {
        // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
        err = await window.go.main.App.SaveAPIKey({ service: f.service, field: f.field, value: f.value })
        if (!err) f.value = MASKED_VALUE
      }
      if (err) { svc.testMsg = err; break }
    }
  } catch { /* 静默处理 */ }
  saving.value = false
}

async function testConnection(svc: ServiceConfig) {
  svc.testStatus = 'testing'
  svc.testMsg = ''
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const result = await window.go.main.App.TestAPIConnection(svc.fields[0].service)
    svc.testStatus = result.ok ? 'ok' : 'fail'
    svc.testMsg = result.message || ''
  } catch (e: unknown) {
    svc.testStatus = 'fail'
    svc.testMsg = String(e)
  }
}

function testStatusIcon(s: TestStatus): string {
  return { untested: '○', testing: '⏳', ok: '✅', fail: '❌' }[s]
}

function testStatusClass(s: TestStatus): string {
  return s === 'ok' ? 'text-green-400' : s === 'fail' ? 'text-red-400' : 'text-gray-500'
}

function clearMaskedValue(field: ServiceConfig['fields'][number]) {
  if (field.value === MASKED_VALUE) field.value = ''
}
</script>

<template>
  <div class="tab-api-keys space-y-6">
    <h2 class="text-base font-semibold text-gray-200 mb-4">
      {{ t('settings.tabs.apiKeys') }}
    </h2>

    <div
      v-for="svc in services"
      :key="svc.fields[0].service"
      class="service-card bg-gray-800 rounded-xl p-5 border border-gray-700"
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="font-semibold text-white">
          {{ t('settings.apiKeys.serviceNames.' + svc.fields[0].service) }}
        </h3>
        <div class="flex items-center gap-2">
          <span
            class="text-xs"
            :class="testStatusClass(svc.testStatus)"
          >
            {{ testStatusIcon(svc.testStatus) }}
            <span v-if="svc.testStatus === 'ok'">{{ t('settings.apiKeys.statusOk') }}</span>
            <span v-else-if="svc.testStatus === 'fail'">{{ t('settings.apiKeys.statusFail') }}</span>
            <span v-else-if="svc.testStatus === 'testing'">{{ t('settings.apiKeys.statusTesting') }}</span>
          </span>
          <button
            class="px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 rounded-lg transition-colors"
            :disabled="svc.testStatus === 'testing'"
            @click="testConnection(svc)"
          >
            {{ t('settings.apiKeys.test') }}
          </button>
        </div>
      </div>

      <div class="space-y-3">
        <div
          v-for="f in svc.fields"
          :key="f.field"
          class="flex items-start gap-3"
        >
          <label class="w-56 shrink-0 pt-2 text-xs text-gray-400 text-left">{{ f.label }}</label>
          <div class="flex flex-1 gap-2">
            <input
              v-model="f.value"
              :type="!f.secret || f.show ? 'text' : 'password'"
              class="flex-1 bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-blue-400 transition-colors"
              @focus="clearMaskedValue(f)"
              @input="svc.testStatus = 'untested'"
            >
            <button
              v-if="f.secret && f.value !== MASKED_VALUE"
              class="px-2 py-2 bg-gray-600 hover:bg-gray-500 rounded-lg transition-colors flex items-center"
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

      <div
        v-if="svc.docs?.length"
        class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs"
      >
        <a
          v-for="doc in svc.docs"
          :key="doc.url"
          :href="doc.url"
          target="_blank"
          rel="noreferrer"
          class="text-blue-300 hover:text-blue-200 underline underline-offset-2"
        >
          {{ doc.label }}
        </a>
      </div>

      <div class="mt-3 flex justify-center">
        <button
          class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 rounded-lg text-xs transition-colors"
          :disabled="saving"
          @click="saveService(svc)"
        >
          {{ t('common.save') }}
        </button>
      </div>

      <p
        v-if="svc.testMsg"
        class="mt-2 text-xs text-red-400"
      >
        {{ svc.testMsg }}
      </p>
    </div>
  </div>
</template>
