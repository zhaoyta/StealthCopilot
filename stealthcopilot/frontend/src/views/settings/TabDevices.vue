<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { HelpCircle, RefreshCw, Video, X } from 'lucide-vue-next'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import MeetingSetupGuide from '../../components/MeetingSetupGuide.vue'
import ObsSetupGuide from '../../components/ObsSetupGuide.vue'

const { t } = useI18n()

interface DeviceOption { id: string; name: string }

const audioInputs  = ref<DeviceOption[]>([])
const audioOutputs = ref<DeviceOption[]>([])
const videoInputs  = ref<DeviceOption[]>([])
const refreshing   = ref(false)
const saving       = ref(false)
const msg          = ref('')
const showMeetingGuide = ref(false)
const showSimliHelp = ref(false)
const showZegoHelp = ref(false)
const showObsGuide = ref(false)
const obsSourceUrl = ref('')

const config = reactive({
  virtualMic:  '',
  physicalMic: '',
  virtualCam:  '',
  monitorOutput: '',
  hearingMonitorEnabled: false,
  hearingMonitorVolume: 80,
  hearingMonitorRate: 0,
  digitalHumanEnabled: false,
  digitalHumanProvider: 'simli',
  simliFaceId: '',
  zegoDigitalHumanId: '',
  zegoRtmpPullUrl: '',
})

type DeviceRole = 'meetingAudio' | 'virtualCam' | 'monitorOutput'

function isVirtualAudioDevice(name: string): boolean {
  const n = name.toLowerCase()
  return n.includes('blackhole') || n.includes('vb-cable') || n.includes('vb-audio') || n.includes('cable output')
}

function isVirtualCameraDevice(name: string): boolean {
  const n = name.toLowerCase()
  return n.includes('obs virtual')
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

function stepIndex(n: number): string {
  return `${n}.`
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
    config.virtualCam  = cfg.virtual_cam_name  || ''
    config.monitorOutput = cfg.monitor_output_name || ''
    config.hearingMonitorEnabled = Boolean(cfg.hearing_monitor_enabled)
    config.hearingMonitorVolume = cfg.hearing_monitor_volume || 80
    config.hearingMonitorRate = cfg.hearing_monitor_rate || 0
    config.digitalHumanEnabled = Boolean(cfg.digital_human_enabled)
    config.digitalHumanProvider = cfg.digital_human_provider || 'simli'
    config.simliFaceId = cfg.simli_face_id || ''
    config.zegoDigitalHumanId = cfg.zego_digital_human_id || ''
    config.zegoRtmpPullUrl = cfg.zego_rtmp_pull_url || ''
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    obsSourceUrl.value = await window.go.main.App.GetDigitalHumanOBSURL()
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
      virtual_cam_name:  config.virtualCam,
      monitor_output_name: config.monitorOutput,
      hearing_monitor_enabled: config.hearingMonitorEnabled,
      hearing_monitor_volume: Number(config.hearingMonitorVolume),
      hearing_monitor_rate: Number(config.hearingMonitorRate),
      digital_human_enabled: config.digitalHumanEnabled,
      digital_human_provider: config.digitalHumanProvider,
      simli_face_id: config.simliFaceId,
      zego_digital_human_id: config.zegoDigitalHumanId,
      zego_rtmp_pull_url: config.zegoRtmpPullUrl,
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

      <div class="border-t border-gray-700 pt-4 space-y-4">
        <!-- 数字人总开关 -->
        <label class="flex items-center gap-2">
          <input
            v-model="config.digitalHumanEnabled"
            type="checkbox"
            class="h-4 w-4 accent-cyan-500"
          >
          <span class="text-xs text-gray-400">{{ t('settings.devices.digitalHumanOutput') }}</span>
        </label>

        <div
          v-if="config.digitalHumanEnabled"
          class="space-y-3"
        >
          <!-- Provider 选择 -->
          <div>
            <label class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.digitalHumanProvider') }}</label>
            <select
              v-model="config.digitalHumanProvider"
              class="form-select"
            >
              <option value="simli">{{ t('settings.devices.digitalHumanProviderSimli') }}</option>
              <option value="zego">{{ t('settings.devices.digitalHumanProviderZego') }}</option>
            </select>
          </div>

          <!-- Simli 配置 -->
          <template v-if="config.digitalHumanProvider === 'simli'">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <span class="text-xs text-gray-500">{{ t('settings.devices.digitalHumanProviderSimli') }}</span>
              <div class="flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  class="flex items-center gap-1 text-cyan-500 hover:text-cyan-300 transition-colors text-xs"
                  @click="showZegoHelp = false; showSimliHelp = true"
                >
                  <HelpCircle class="w-3.5 h-3.5" />
                  <span>{{ t('settings.devices.simliHelp') }}</span>
                </button>
                <button
                  type="button"
                  class="flex items-center gap-1 text-cyan-500 hover:text-cyan-300 transition-colors text-xs"
                  @click="showZegoHelp = false; showSimliHelp = false; showObsGuide = true"
                >
                  <Video class="w-3.5 h-3.5" />
                  <span>{{ t('settings.devices.obsHelp') }}</span>
                </button>
              </div>
            </div>
            <label class="block">
              <span class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.simliFaceId') }}</span>
              <input
                v-model="config.simliFaceId"
                class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-400"
                type="text"
                :placeholder="t('settings.devices.simliFaceIdPlaceholder')"
              >
            </label>
            <label class="block">
              <span class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.obsSourceUrl') }}</span>
              <input
                :value="obsSourceUrl"
                class="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-xs text-gray-300 focus:outline-none"
                type="text"
                readonly
              >
            </label>
            <div class="rounded-lg border border-cyan-500/20 bg-cyan-500/10 px-3 py-2 text-xs leading-relaxed text-cyan-50/90">
              {{ t('settings.devices.obsInlineHint') }}
            </div>
          </template>

          <!-- ZEGO 配置 -->
          <template v-else-if="config.digitalHumanProvider === 'zego'">
            <div class="flex items-center justify-between">
              <span class="text-xs text-gray-500">{{ t('settings.devices.digitalHumanProviderZego') }}</span>
              <button
                type="button"
                class="flex items-center gap-1 text-cyan-500 hover:text-cyan-300 transition-colors text-xs"
                @click="showSimliHelp = false; showZegoHelp = true"
              >
                <HelpCircle class="w-3.5 h-3.5" />
                <span>{{ t('settings.devices.zegoHelp') }}</span>
              </button>
            </div>
            <label class="block">
              <span class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.zegoDigitalHumanId') }}</span>
              <input
                v-model="config.zegoDigitalHumanId"
                class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-400"
                type="text"
                :placeholder="t('settings.devices.zegoDigitalHumanIdPlaceholder')"
              >
            </label>
            <label class="block">
              <span class="block text-xs text-gray-400 mb-1">{{ t('settings.devices.zegoRtmpPullUrl') }}</span>
              <input
                v-model="config.zegoRtmpPullUrl"
                class="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-400"
                type="text"
                :placeholder="t('settings.devices.zegoRtmpPullUrlPlaceholder')"
              >
            </label>
          </template>
        </div>
      </div>
    </div>

    <!-- Simli AI 配置说明弹窗 -->
    <Teleport to="body">
      <div
        v-if="showSimliHelp"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/70"
        @click.self="showSimliHelp = false"
      >
        <div class="bg-gray-800 border border-gray-700 rounded-xl p-6 max-w-lg w-full mx-4 shadow-2xl">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-white font-semibold text-sm">{{ t('settings.devices.simliHelpTitle') }}</h3>
            <button class="text-gray-500 hover:text-white" @click="showSimliHelp = false">
              <X class="w-4 h-4" />
            </button>
          </div>
          <p class="text-xs text-gray-400 mb-4">{{ t('settings.devices.simliHelpDesc') }}</p>
          <ol class="space-y-3 text-xs text-gray-300">
            <li v-for="n in 4" :key="n" class="flex gap-2">
              <span class="text-cyan-400 font-bold shrink-0">{{ stepIndex(n) }}</span>
              <span>{{ t(`settings.devices.simliHelpStep${n}`) }}</span>
            </li>
          </ol>
          <p class="text-xs text-gray-500 mt-4 border-t border-gray-700 pt-3">{{ t('settings.devices.simliHelpNote') }}</p>
        </div>
      </div>
    </Teleport>

    <!-- 即构数字人配置说明弹窗 -->
    <Teleport to="body">
      <div
        v-if="showZegoHelp"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/70"
        @click.self="showZegoHelp = false"
      >
        <div class="bg-gray-800 border border-gray-700 rounded-xl p-6 max-w-lg w-full mx-4 shadow-2xl">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-white font-semibold text-sm">{{ t('settings.devices.zegoHelpTitle') }}</h3>
            <button class="text-gray-500 hover:text-white" @click="showZegoHelp = false">
              <X class="w-4 h-4" />
            </button>
          </div>
          <p class="text-xs text-gray-400 mb-4">{{ t('settings.devices.zegoHelpDesc') }}</p>
          <ol class="space-y-3 text-xs text-gray-300">
            <li class="flex gap-2">
              <span class="text-cyan-400 font-bold shrink-0">{{ stepIndex(1) }}</span>
              <span>{{ t('settings.devices.zegoHelpStep1') }}
                <button class="text-cyan-400 underline ml-1" @click="BrowserOpenURL('https://console.zego.im/')">{{ t('settings.devices.zegoHelpConsoleLink') }}</button>
                {{ t('settings.devices.zegoHelpStep1b') }}
              </span>
            </li>
            <li class="flex gap-2">
              <span class="text-cyan-400 font-bold shrink-0">{{ stepIndex(2) }}</span>
              <span>{{ t('settings.devices.zegoHelpStep2') }}</span>
            </li>
            <li class="flex gap-2">
              <span class="text-cyan-400 font-bold shrink-0">{{ stepIndex(3) }}</span>
              <span>{{ t('settings.devices.zegoHelpStep3') }}</span>
            </li>
            <li class="flex gap-2">
              <span class="text-cyan-400 font-bold shrink-0">{{ stepIndex(4) }}</span>
              <span>{{ t('settings.devices.zegoHelpStep4') }}</span>
            </li>
          </ol>
          <p class="text-xs text-gray-500 mt-4 border-t border-gray-700 pt-3">{{ t('settings.devices.zegoHelpNote') }}</p>
        </div>
      </div>
    </Teleport>

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
    <ObsSetupGuide
      :open="showObsGuide"
      :source-url="obsSourceUrl"
      @close="showObsGuide = false"
    />
  </div>
</template>
