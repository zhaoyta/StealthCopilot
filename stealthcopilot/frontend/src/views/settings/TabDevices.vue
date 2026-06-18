<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { HelpCircle, RefreshCw } from 'lucide-vue-next'
import MeetingSetupGuide from '../../components/MeetingSetupGuide.vue'

const { t } = useI18n()

interface DeviceOption { id: string; name: string }

const audioInputs  = ref<DeviceOption[]>([])
const audioOutputs = ref<DeviceOption[]>([])
const videoInputs  = ref<DeviceOption[]>([])
const refreshing   = ref(false)
const saving       = ref(false)
const msg          = ref('')
const showMeetingGuide = ref(false)

const config = reactive({
  virtualMic:  '',
  physicalMic: '',
  physicalCam: '',
  virtualCam:  '',
  monitorOutput: '',
  hearingMonitorEnabled: false,
  hearingMonitorVolume: 80,
  hearingMonitorRate: 0,
})

type DeviceRole = 'meetingAudio' | 'virtualCam' | 'monitorOutput'

function isVirtualAudioDevice(name: string): boolean {
  const n = name.toLowerCase()
  return n.includes('blackhole') || n.includes('vb-cable') || n.includes('vb-audio') || n.includes('cable output')
}

function isVirtualCameraDevice(name: string): boolean {
  const n = name.toLowerCase()
  return n.includes('stealthvirtualcam') || n.includes('stealth virtual') || n.includes('obs virtual')
}

function deviceOptionLabel(device: DeviceOption, role: DeviceRole): string {
  if (role === 'meetingAudio' && isVirtualAudioDevice(device.name)) {
    return t('settings.devices.recommendedVirtualMic', { name: device.name })
  }
  if (role === 'virtualCam' && isVirtualCameraDevice(device.name)) {
    return t('settings.devices.recommendedVirtualCam', { name: device.name })
  }
  if (role === 'monitorOutput' && device.name.toLowerCase().includes('headphone')) {
    return t('settings.devices.recommendedMonitorOutput', { name: device.name })
  }
  return device.name
}

async function loadDevices() {
  refreshing.value = true
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const dl = await window.go.main.App.EnumerateDevices()
    audioInputs.value = dl.audio_inputs  || []
    audioOutputs.value = dl.audio_outputs || [{ id: 'default', name: 'System Default Output' }]
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
    config.monitorOutput = cfg.monitor_output_name || ''
    config.hearingMonitorEnabled = Boolean(cfg.hearing_monitor_enabled)
    config.hearingMonitorVolume = cfg.hearing_monitor_volume || 80
    config.hearingMonitorRate = cfg.hearing_monitor_rate || 0
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
      monitor_output_name: config.monitorOutput,
      hearing_monitor_enabled: config.hearingMonitorEnabled,
      hearing_monitor_volume: Number(config.hearingMonitorVolume),
      hearing_monitor_rate: Number(config.hearingMonitorRate),
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
      <div class="flex items-center gap-2">
        <button
          class="flex items-center gap-2 px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm transition-colors"
          @click="showMeetingGuide = true"
        >
          <HelpCircle :size="14" />
          {{ t('meetingGuide.entry') }}
        </button>
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
    </div>

    <div class="bg-gray-800 rounded-xl p-5 border border-gray-700 space-y-4">
      <!-- 会议音频虚拟通道 -->
      <div>
        <label class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.virtualMic') }}</label>
        <p class="mb-2 text-[11px] leading-relaxed text-gray-500">
          {{ t('settings.devices.virtualMicHint') }}
        </p>
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
            {{ deviceOptionLabel(d, 'meetingAudio') }}
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

      <!-- 听力链译文播报 -->
      <div class="border-t border-gray-700 pt-4 space-y-4">
        <label class="flex items-center justify-between gap-4">
          <span class="text-xs text-gray-400">{{ t('settings.devices.hearingMonitor') }}</span>
          <input
            v-model="config.hearingMonitorEnabled"
            type="checkbox"
            class="h-4 w-4 accent-blue-500"
          >
        </label>

        <div>
          <label class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.monitorOutput') }}</label>
          <select
            v-model="config.monitorOutput"
            class="form-select"
          >
            <option value="">
              {{ t('settings.devices.systemDefaultOutput') }}
            </option>
            <option
              v-for="d in audioOutputs"
              :key="d.id"
              :value="d.name"
            >
              {{ deviceOptionLabel(d, 'monitorOutput') }}
            </option>
          </select>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <label class="block">
            <span class="block text-xs text-gray-400 mb-1">
              {{ `${t('settings.devices.monitorVolume')} ${config.hearingMonitorVolume}` }}
            </span>
            <input
              v-model.number="config.hearingMonitorVolume"
              type="range"
              min="0"
              max="100"
              class="w-full accent-blue-500"
            >
          </label>
          <label class="block">
            <span class="block text-xs text-gray-400 mb-1">
              {{ `${t('settings.devices.monitorRate')} ${config.hearingMonitorRate}` }}
            </span>
            <input
              v-model.number="config.hearingMonitorRate"
              type="range"
              min="-5"
              max="5"
              class="w-full accent-blue-500"
            >
          </label>
        </div>
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

    <MeetingSetupGuide
      :open="showMeetingGuide"
      @close="showMeetingGuide = false"
    />
  </div>
</template>
