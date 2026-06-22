<script lang="ts" setup>
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Save, X } from 'lucide-vue-next'
import { EnumerateDevices, GetConfig, SaveLocalConfig } from '../../wailsjs/go/main/App'
import type { config as WailsConfig } from '../../wailsjs/go/models'

defineOptions({ name: 'DeviceQuickConfig' })

type Target = 'meetingAudio' | 'physicalMic' | 'monitorOutput' | 'virtualCamera'

interface DeviceOption { id: string; name: string }

const props = defineProps<{
  open: boolean
  target: Target | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const { t } = useI18n()

const audioInputs = ref<DeviceOption[]>([])
const audioOutputs = ref<DeviceOption[]>([])
const videoInputs = ref<DeviceOption[]>([])
const loading = ref(false)
const saving = ref(false)
const message = ref('')
const currentConfig = ref<Record<string, unknown> | null>(null)

const form = reactive({
  virtualMic: '',
  physicalMic: '',
  monitorOutput: '',
  hearingMonitorEnabled: false,
  virtualCam: '',
})

const title = computed(() => props.target ? t(`deviceQuickConfig.${props.target}.title`) : '')
const desc = computed(() => props.target ? t(`deviceQuickConfig.${props.target}.desc`) : '')

function isVirtualAudioDevice(name: string): boolean {
  const n = name.toLowerCase()
  return n.includes('blackhole') || n.includes('vb-cable') || n.includes('vb-audio') || n.includes('cable output')
}

function isVirtualCameraDevice(name: string): boolean {
  const n = name.toLowerCase()
  return n.includes('obs virtual')
}

function optionLabel(device: DeviceOption, role: Target): string {
  if (role === 'meetingAudio' && isVirtualAudioDevice(device.name)) {
    return t('settings.devices.recommendedVirtualMic', { name: device.name })
  }
  if (role === 'virtualCamera' && isVirtualCameraDevice(device.name)) {
    return t('settings.devices.recommendedVirtualCam', { name: device.name })
  }
  if (role === 'monitorOutput' && device.name.toLowerCase().includes('headphone')) {
    return t('settings.devices.recommendedMonitorOutput', { name: device.name })
  }
  return device.name
}

async function load() {
  if (!props.open) return
  loading.value = true
  message.value = ''
  try {
    const [devices, cfg] = await Promise.all([EnumerateDevices(), GetConfig()])
    audioInputs.value = devices.audio_inputs || []
    audioOutputs.value = devices.audio_outputs || [{ id: 'default', name: t('settings.devices.systemDefaultOutput') }]
    videoInputs.value = devices.video_inputs || []
    currentConfig.value = cfg as unknown as Record<string, unknown>
    form.virtualMic = cfg.virtual_mic_name || ''
    form.physicalMic = cfg.physical_mic_name || ''
    form.monitorOutput = cfg.monitor_output_name || ''
    form.hearingMonitorEnabled = Boolean(cfg.hearing_monitor_enabled)
    form.virtualCam = cfg.virtual_cam_name || ''
  } catch (e: unknown) {
    message.value = String(e)
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!currentConfig.value || !props.target) return
  saving.value = true
  message.value = ''
  const patch: Record<string, unknown> = {}
  if (props.target === 'meetingAudio') patch.virtual_mic_name = form.virtualMic
  if (props.target === 'physicalMic') patch.physical_mic_name = form.physicalMic
  if (props.target === 'monitorOutput') {
    patch.monitor_output_name = form.monitorOutput
    patch.hearing_monitor_enabled = form.hearingMonitorEnabled
  }
  if (props.target === 'virtualCamera') patch.virtual_cam_name = form.virtualCam

  try {
    const payload = { ...currentConfig.value, ...patch } as unknown as WailsConfig.SaveLocalConfigRequest
    const err = await SaveLocalConfig(payload)
    if (err) {
      message.value = err
      return
    }
    emit('saved')
    emit('close')
  } catch (e: unknown) {
    message.value = String(e)
  } finally {
    saving.value = false
  }
}

watch(() => [props.open, props.target] as const, () => {
  void load()
})
</script>

<template>
  <div
    v-if="open && target"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/55 px-4"
    @click.self="emit('close')"
  >
    <section class="w-full max-w-lg rounded-2xl border border-gray-700 bg-gray-900 text-white shadow-2xl">
      <header class="flex items-start justify-between border-b border-gray-700 px-5 py-4">
        <div>
          <h2 class="text-base font-semibold text-white">{{ title }}</h2>
          <p class="mt-1 text-xs leading-relaxed text-gray-400">{{ desc }}</p>
        </div>
        <button
          class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
          @click="emit('close')"
        >
          <X :size="16" />
        </button>
      </header>

      <div class="space-y-4 px-5 py-5">
        <p v-if="loading" class="text-sm text-gray-400">{{ t('common.loading') }}</p>

        <template v-else>
          <label v-if="target === 'meetingAudio'" class="block">
            <span class="mb-1 block text-xs text-gray-400">{{ t('settings.devices.virtualMic') }}</span>
            <select v-model="form.virtualMic" class="form-select">
              <option value="">{{ t('settings.devices.select') }}</option>
              <option v-for="d in audioInputs" :key="d.id" :value="d.name">
                {{ optionLabel(d, 'meetingAudio') }}
              </option>
            </select>
          </label>

          <label v-if="target === 'physicalMic'" class="block">
            <span class="mb-1 block text-xs text-gray-400">{{ t('settings.devices.physicalMic') }}</span>
            <select v-model="form.physicalMic" class="form-select">
              <option value="">{{ t('settings.devices.select') }}</option>
              <option v-for="d in audioInputs" :key="d.id" :value="d.name">{{ d.name }}</option>
            </select>
          </label>

          <div v-if="target === 'monitorOutput'" class="space-y-4">
            <label class="flex items-center justify-between gap-4 rounded-xl border border-gray-700 bg-gray-800/70 px-4 py-3">
              <span class="text-xs text-gray-300">{{ t('settings.devices.hearingMonitor') }}</span>
              <input v-model="form.hearingMonitorEnabled" type="checkbox" class="h-4 w-4 accent-blue-500">
            </label>
            <label class="block">
              <span class="mb-1 block text-xs text-gray-400">{{ t('settings.devices.monitorOutput') }}</span>
              <select v-model="form.monitorOutput" class="form-select">
                <option value="">{{ t('settings.devices.systemDefaultOutput') }}</option>
                <option v-for="d in audioOutputs" :key="d.id" :value="d.name">
                  {{ optionLabel(d, 'monitorOutput') }}
                </option>
              </select>
            </label>
          </div>

          <label v-if="target === 'virtualCamera'" class="block">
            <span class="mb-1 block text-xs text-gray-400">{{ t('settings.devices.virtualCam') }}</span>
            <select v-model="form.virtualCam" class="form-select">
              <option value="">{{ t('settings.devices.select') }}</option>
              <option v-for="d in videoInputs" :key="d.id" :value="d.name">
                {{ optionLabel(d, 'virtualCamera') }}
              </option>
            </select>
          </label>
        </template>

        <p v-if="message" class="text-xs text-red-400">{{ message }}</p>
      </div>

      <footer class="flex items-center justify-end gap-2 border-t border-gray-700 px-5 py-4">
        <button
          class="px-4 py-2 rounded-lg bg-gray-700 text-sm text-gray-200 transition-colors hover:bg-gray-600"
          @click="emit('close')"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          class="flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-600 text-sm font-semibold text-white transition-colors hover:bg-blue-500 disabled:opacity-50"
          :disabled="loading || saving"
          @click="save"
        >
          <Save :size="14" />
          {{ t('common.save') }}
        </button>
      </footer>
    </section>
  </div>
</template>
