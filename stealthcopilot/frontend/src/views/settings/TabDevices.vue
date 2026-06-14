<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RefreshCw } from 'lucide-vue-next'

const { t } = useI18n()

interface DeviceOption { id: string; name: string }

const audioInputs  = ref<DeviceOption[]>([])
const videoInputs  = ref<DeviceOption[]>([])
const refreshing   = ref(false)
const saving       = ref(false)
const msg          = ref('')

const config = reactive({
  virtualMic:  '',
  physicalMic: '',
  physicalCam: '',
  virtualCam:  '',
})

async function loadDevices() {
  refreshing.value = true
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const dl = await window.go.main.App.EnumerateDevices()
    audioInputs.value = dl.audio_inputs  || []
    videoInputs.value = dl.video_inputs  || []
  } catch { /* 静默处理 */ }
  refreshing.value = false
}

onMounted(async () => {
  await loadDevices()
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cfg = await window.go.main.App.GetConfig()
    config.virtualMic  = cfg.virtual_mic_name  || ''
    config.physicalMic = cfg.physical_mic_name || ''
    config.physicalCam = cfg.physical_cam_name || ''
    config.virtualCam  = cfg.virtual_cam_name  || ''
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
      virtual_mic_name:  config.virtualMic,
      physical_mic_name: config.physicalMic,
      physical_cam_name: config.physicalCam,
      virtual_cam_name:  config.virtualCam,
    })
    msg.value = err || t('common.success')
  } catch (e: unknown) { msg.value = String(e) }
  saving.value = false
}
</script>

<template>
  <div class="tab-devices space-y-6">
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-base font-semibold text-gray-200">
        {{ t('settings.tabs.devices') }}
      </h2>
      <button
        class="flex items-center gap-2 px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm transition-colors"
        :disabled="refreshing"
        @click="loadDevices"
      >
        <RefreshCw
          :size="14"
          :class="refreshing ? 'animate-spin' : ''"
        />
        {{ t('settings.devices.refresh') }}
      </button>
    </div>

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <!-- 虚拟声卡 -->
      <div>
        <label class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.virtualMic') }}</label>
        <select
          v-model="config.virtualMic"
          class="form-select"
        >
          <option value="">
            {{ t('settings.devices.select') }}
          </option>
          <option
            v-for="d in audioInputs"
            :key="d.id"
            :value="d.name"
          >
            {{ d.name }}
          </option>
        </select>
      </div>

      <!-- 物理麦克风 -->
      <div>
        <label class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.physicalMic') }}</label>
        <select
          v-model="config.physicalMic"
          class="form-select"
        >
          <option value="">
            {{ t('settings.devices.select') }}
          </option>
          <option
            v-for="d in audioInputs"
            :key="d.id"
            :value="d.name"
          >
            {{ d.name }}
          </option>
        </select>
      </div>

      <!-- 物理摄像头 -->
      <div>
        <label class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.physicalCam') }}</label>
        <select
          v-model="config.physicalCam"
          class="form-select"
        >
          <option value="">
            {{ t('settings.devices.select') }}
          </option>
          <option
            v-for="d in videoInputs"
            :key="d.id"
            :value="d.name"
          >
            {{ d.name }}
          </option>
        </select>
      </div>

      <!-- 虚拟摄像头 -->
      <div>
        <label class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.virtualCam') }}</label>
        <select
          v-model="config.virtualCam"
          class="form-select"
        >
          <option value="">
            {{ t('settings.devices.select') }}
          </option>
          <option
            v-for="d in videoInputs"
            :key="d.id"
            :value="d.name"
          >
            {{ d.name }}
          </option>
        </select>
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
