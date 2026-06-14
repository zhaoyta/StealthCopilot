<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Eye, EyeOff } from 'lucide-vue-next'

const { t } = useI18n()

type TestStatus = 'untested' | 'testing' | 'ok' | 'fail'

interface ServiceConfig {
  name: string
  fields: {
    service: string
    field: string
    label: string
    value: string
    show: boolean
  }[]
  testStatus: TestStatus
  testMsg: string
}

const services = reactive<ServiceConfig[]>([
  {
    name: '讯飞 (iFlytek)',
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'xunfei', field: 'app_id',     label: t('settings.apiKeys.xunfei.appId'),     value: '', show: false },
      { service: 'xunfei', field: 'api_key',    label: t('settings.apiKeys.xunfei.apiKey'),    value: '', show: false },
      { service: 'xunfei', field: 'api_secret', label: t('settings.apiKeys.xunfei.apiSecret'), value: '', show: false },
    ],
  },
  {
    name: 'DeepSeek',
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'deepseek', field: 'key',   label: t('settings.apiKeys.deepseek.key'),   value: '', show: false },
      { service: 'deepseek', field: 'model', label: t('settings.apiKeys.deepseek.model'), value: '', show: false },
    ],
  },
  {
    name: 'ElevenLabs',
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'elevenlabs', field: 'key',      label: t('settings.apiKeys.elevenlabs.key'),    value: '', show: false },
      { service: 'elevenlabs', field: 'voice_id', label: t('settings.apiKeys.elevenlabs.voiceId'), value: '', show: false },
    ],
  },
  {
    name: 'Simli AI',
    testStatus: 'untested',
    testMsg: '',
    fields: [
      { service: 'simli', field: 'key',     label: t('settings.apiKeys.simli.key'),    value: '', show: false },
      { service: 'simli', field: 'face_id', label: t('settings.apiKeys.simli.faceId'), value: '', show: false },
    ],
  },
])

const saving = ref(false)

onMounted(async () => {
  // 加载已设置状态标记（API Key 原值不回显，仅显示是否已设置）
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cfg = await window.go.main.App.GetConfig()
    const setMap: Record<string, boolean> = {
      xunfei_app_id: cfg.xunfei_app_id_set,
      xunfei_api_key: cfg.xunfei_api_key_set,
      xunfei_api_secret: cfg.xunfei_api_secret_set,
      deepseek_key: cfg.deepseek_key_set,
      deepseek_model: !!cfg.deepseek_model,
      elevenlabs_key: cfg.elevenlabs_key_set,
      elevenlabs_voice_id: cfg.elevenlabs_voice_id_set,
      simli_key: cfg.simli_key_set,
      simli_face_id: cfg.simli_face_id_set,
    }
    for (const svc of services) {
      for (const f of svc.fields) {
        const k = `${f.service}_${f.field}`
        if (setMap[k]) f.value = '••••••••' // 已设置时显示掩码
      }
      // deepseek model 回显实际值
      if (svc.fields[0].service === 'deepseek') {
        svc.fields[1].value = cfg.deepseek_model || ''
      }
    }
  } catch { /* 加载失败静默处理 */ }
})

// 保存一张服务卡的全部字段（跳过未改动的掩码值）
async function saveService(svc: ServiceConfig) {
  saving.value = true
  svc.testStatus = 'untested'
  svc.testMsg = ''
  try {
    for (const f of svc.fields) {
      if (!f.value || f.value === '••••••••') continue
      let err = ''
      if (f.service === 'deepseek' && f.field === 'model') {
        // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
        const cur = await window.go.main.App.GetConfig()
        // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
        err = await window.go.main.App.SaveLocalConfig({ ...cur, deepseek_model: f.value })
      } else {
        // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
        err = await window.go.main.App.SaveAPIKey({ service: f.service, field: f.field, value: f.value })
        if (!err) f.value = '••••••••'
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
      <!-- 服务标题行 -->
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

      <!-- 字段列表 -->
      <div class="space-y-3">
        <div
          v-for="f in svc.fields"
          :key="f.field"
          class="flex items-center gap-3"
        >
          <label class="w-28 shrink-0 text-xs text-gray-400 text-left">{{ f.label }}</label>
          <div class="flex flex-1 gap-2">
            <input
              v-model="f.value"
              :type="f.show ? 'text' : 'password'"
              class="flex-1 bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-blue-400 transition-colors"
              @input="svc.testStatus = 'untested'"
            >
            <button
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

      <!-- 每张卡片底部统一保存按钮 -->
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
