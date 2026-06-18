<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Headphones, Mic, Video, Settings, Maximize2, Info, Play, Square, ChevronRight, SlidersHorizontal, AlertTriangle } from 'lucide-vue-next'
import {
  StartHearingChain,
  StopHearingChain,
  StartSpeakingChain,
  StopSpeakingChain,
  StartVideoChain,
  StopVideoChain,
  CheckDeps,
  EnumerateDevices,
  GetConfig,
  HideTeleprompter,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import i18n from '../i18n'
import MeetingSetupGuide from '../components/MeetingSetupGuide.vue'
import DeviceQuickConfig from '../components/DeviceQuickConfig.vue'

defineOptions({ name: 'AppDashboard' })
const { t } = useI18n()

const UI_LOCALES = [
  { code: 'zh-CN', label: '中文' },
  { code: 'en-US', label: 'English' },
]

const emit = defineEmits<{
  (e: 'openSettings', tab?: 'apiKeys' | 'language' | 'devices' | 'voice' | 'resume' | 'ghost' | 'advanced'): void
  (e: 'openTeleprompter'): void
}>()

type ChainStatus = 'idle' | 'running' | 'error'
type DeviceConfigTarget = 'meetingAudio' | 'physicalMic' | 'monitorOutput' | 'physicalCamera' | 'virtualCamera'

const hearingStatus = ref<ChainStatus>('idle')
const speakingStatus = ref<ChainStatus>('idle')
const videoStatus = ref<ChainStatus>('idle')
const circuitOpen = ref(false)
const errorMsg = ref('')
const startupIssues = ref<string[]>([])
const startupWarnings = ref<string[]>([])
const hearingLangPair = ref('')
const speakingLangPair = ref('')
const uiLocale = ref<'zh-CN' | 'en-US'>('zh-CN')
const channelNames = ref<Record<string, string>>({})
const showMeetingGuide = ref(false)
const deviceConfigTarget = ref<DeviceConfigTarget | null>(null)
const showPreflightDialog = ref(false)

const runningCount = computed(() =>
  [hearingStatus.value, speakingStatus.value, videoStatus.value].filter(s => s === 'running').length
)
const systemOk = computed(() =>
  !circuitOpen.value && hearingStatus.value !== 'error' && speakingStatus.value !== 'error' && videoStatus.value !== 'error'
)

// 将讯飞语言代码转为 locale 显示名（读 i18n langs map）
function langLabel(code: string): string {
  return t(`settings.language.langs.${code}`, code)
}

// ===== 听力链控制 =====

async function toggleHearing(on: boolean) {
  errorMsg.value = ''
  startupIssues.value = []
  startupWarnings.value = []
  if (on) {
    const err = await StartHearingChain()
    if (err) { hearingStatus.value = 'error'; errorMsg.value = err; return }
    hearingStatus.value = 'running'
    // 提词窗由用户手动点击右上角按钮打开，不在此处自动弹出
  } else {
    await StopHearingChain()
    hearingStatus.value = 'idle'
    await HideTeleprompter()
  }
}

async function toggleSpeaking(on: boolean) {
  errorMsg.value = ''
  startupIssues.value = []
  startupWarnings.value = []
  if (on) {
    const err = await StartSpeakingChain()
    if (err) { speakingStatus.value = 'error'; errorMsg.value = err; return }
    speakingStatus.value = 'running'
  } else {
    await StopSpeakingChain()
    speakingStatus.value = 'idle'
  }
}

async function toggleVideo(on: boolean) {
  errorMsg.value = ''
  startupIssues.value = []
  startupWarnings.value = []
  if (on) {
    const err = await StartVideoChain()
    if (err) { videoStatus.value = 'error'; errorMsg.value = err; return }
    videoStatus.value = 'running'
  } else {
    await StopVideoChain()
    videoStatus.value = 'idle'
    circuitOpen.value = false
  }
}

async function startAll() {
  const canStart = await validateStartAll()
  if (!canStart || startupWarnings.value.length) {
    showPreflightDialog.value = true
    return
  }
  await runStartAll()
}

async function runStartAll() {
  const warnings = [...startupWarnings.value]
  if (videoStatus.value !== 'running') await toggleVideo(true)
  if (speakingStatus.value !== 'running') await toggleSpeaking(true)
  if (hearingStatus.value !== 'running') await toggleHearing(true)
  startupWarnings.value = warnings
}

async function continueStartAll() {
  showPreflightDialog.value = false
  await runStartAll()
}

function closePreflightDialog() {
  showPreflightDialog.value = false
}

function openSettingsFromPreflight() {
  showPreflightDialog.value = false
  emit('openSettings')
}

async function stopAll() {
  if (hearingStatus.value === 'running') await toggleHearing(false)
  if (speakingStatus.value === 'running') await toggleSpeaking(false)
  if (videoStatus.value === 'running') await toggleVideo(false)
}

async function loadDashboardConfig() {
  const cfg = await GetConfig()
  uiLocale.value         = (cfg.ui_locale || 'zh-CN') as 'zh-CN' | 'en-US'
  hearingLangPair.value  = `${langLabel(cfg.hearing_source_lang || 'en')} → ${langLabel(cfg.hearing_target_lang || 'zh')}`
  speakingLangPair.value = `${langLabel(cfg.speaking_input_lang || 'zh')} → ${langLabel(cfg.speaking_output_lang || 'en')}`
  const translationProvider = providerLabel(cfg.translation_provider || 'xunfei')
  const llmProvider = providerLabel(cfg.llm_provider || 'deepseek')
  const ttsProvider = providerLabel(cfg.tts_provider || 'system')
  const lipsyncProvider = providerLabel(cfg.lipsync_provider || 'simli')
  const embeddingProvider = providerLabel(cfg.embedding_provider || 'python_bridge')

  channelNames.value = {
    meetingAudio: deviceLabel(cfg.virtual_mic_name),
    blackhole: deviceLabel(cfg.virtual_mic_name),
    xunfeiTranslation: translationProvider,
    xunfeiAsr: translationProvider,
    ragDeepseek: `${embeddingProvider} + ${llmProvider}`,
    localVectorStore: embeddingProvider,
    deepseek: llmProvider,
    systemTts: cfg.hearing_monitor_enabled
      ? deviceLabel(cfg.monitor_output_name || t('settings.devices.systemDefaultOutput'))
      : '',
    physicalMic: deviceLabel(cfg.physical_mic_name),
    ttsOutput: ttsProvider,
    xunfeiVoiceClone: ttsProvider,
    virtualMic: deviceLabel(cfg.virtual_mic_name),
    physicalCamera: deviceLabel(cfg.physical_cam_name),
    simli: lipsyncProvider,
    virtualCamera: deviceLabel(cfg.virtual_cam_name),
  }
}

async function validateStartAll(): Promise<boolean> {
  errorMsg.value = ''
  startupIssues.value = []
  startupWarnings.value = []

  try {
    const [cfg, deps, devices] = await Promise.all([GetConfig(), CheckDeps(), EnumerateDevices()])
    const issues: string[] = []
    const warnings: string[] = []
    const audioInputs = devices.audio_inputs || []
    const audioOutputs = devices.audio_outputs || []
    const videoInputs = devices.video_inputs || []

    if (deps.ffmpeg !== 'installed') issues.push(t('dashboard.preflight.ffmpegMissing'))
    if (deps.virtual_mic !== 'installed') issues.push(t('dashboard.preflight.virtualAudioMissing'))
    if (deps.virtual_cam !== 'installed') issues.push(t('dashboard.preflight.virtualCameraMissing'))

    if (!cfg.virtual_mic_name) issues.push(t('dashboard.preflight.meetingAudioMissing'))
    if (!cfg.physical_mic_name) issues.push(t('dashboard.preflight.speakingMicMissing'))
    if (!cfg.physical_cam_name) issues.push(t('dashboard.preflight.realCameraMissing'))
    if (!cfg.virtual_cam_name) issues.push(t('dashboard.preflight.meetingCameraMissing'))
    if (cfg.virtual_mic_name && !deviceExists(audioInputs, cfg.virtual_mic_name)) {
      issues.push(t('dashboard.preflight.meetingAudioUnavailable', { name: cfg.virtual_mic_name }))
    }
    if (cfg.physical_mic_name && !deviceExists(audioInputs, cfg.physical_mic_name)) {
      issues.push(t('dashboard.preflight.speakingMicUnavailable', { name: cfg.physical_mic_name }))
    }
    if (cfg.physical_cam_name && !deviceExists(videoInputs, cfg.physical_cam_name)) {
      issues.push(t('dashboard.preflight.realCameraUnavailable', { name: cfg.physical_cam_name }))
    }
    if (cfg.virtual_cam_name && !deviceExists(videoInputs, cfg.virtual_cam_name)) {
      issues.push(t('dashboard.preflight.meetingCameraUnavailable', { name: cfg.virtual_cam_name }))
    }
    if (cfg.hearing_monitor_enabled && cfg.monitor_output_name && !deviceExists(audioOutputs, cfg.monitor_output_name)) {
      issues.push(t('dashboard.preflight.monitorOutputUnavailable', { name: cfg.monitor_output_name }))
    }

    if (!cfg.xunfei_rtasr_app_id_set || !cfg.xunfei_rtasr_api_key_set) {
      issues.push(t('dashboard.preflight.rtasrMissing'))
    }
    const needsHearingTranslation = (cfg.hearing_source_lang || 'en') !== (cfg.hearing_target_lang || 'zh')
    const needsSpeakingTranslation = (cfg.speaking_input_lang || 'zh') !== (cfg.speaking_output_lang || 'en')
    if ((needsHearingTranslation || needsSpeakingTranslation) &&
      (!cfg.xunfei_mt_app_id_set || !cfg.xunfei_mt_api_key_set || !cfg.xunfei_mt_api_secret_set)) {
      issues.push(t('dashboard.preflight.mtMissing'))
    }
    if (!cfg.deepseek_key_set) warnings.push(t('dashboard.preflight.deepseekMissing'))
    if (cfg.tts_provider === 'null') issues.push(t('dashboard.preflight.ttsMissing'))
    if (cfg.tts_provider === 'xunfei_voiceclone' && !cfg.xunfei_tts_asset_id_set) {
      issues.push(t('dashboard.preflight.voiceCloneMissing'))
    }
    if (cfg.lipsync_provider === 'simli' && (!cfg.simli_key_set || !cfg.simli_face_id_set)) {
      warnings.push(t('dashboard.preflight.simliMissing'))
    }

    startupIssues.value = issues
    startupWarnings.value = warnings
    return issues.length === 0
  } catch (e: unknown) {
    startupIssues.value = [String(e)]
    return false
  }
}

function openDeviceConfig(target: DeviceConfigTarget) {
  deviceConfigTarget.value = target
}

function deviceExists(devices: Array<{ name?: string }>, name: string): boolean {
  const expected = normalizeDeviceName(name)
  return devices.some(device => normalizeDeviceName(device.name || '') === expected)
}

function normalizeDeviceName(name: string): string {
  return name.trim().toLowerCase()
}

// ===== 初始化 =====

onMounted(async () => {
  EventsOn('circuit:open',   () => { circuitOpen.value = true  })
  EventsOn('circuit:closed', () => { circuitOpen.value = false })
  // hearing:error：讯飞重连耗尽，链路中断，更新听力链状态并展示错误信息
  EventsOn('hearing:error', (msg: string) => {
    hearingStatus.value = 'error'
    errorMsg.value = msg
  })
  // speaking:* 事件：Phase 2 说话链状态，提前注册备用
  EventsOn('speaking:start', () => { speakingStatus.value = 'running' })
  EventsOn('speaking:done',  () => { speakingStatus.value = 'idle'    })
  EventsOn('speaking:error', (msg: string) => {
    speakingStatus.value = 'error'
    errorMsg.value = msg
  })
  try { await loadDashboardConfig() } catch { /* 静默 */ }
})
onUnmounted(() => {
  EventsOff('circuit:open')
  EventsOff('circuit:closed')
  EventsOff('hearing:error')
  EventsOff('speaking:start')
  EventsOff('speaking:done')
  EventsOff('speaking:error')
})

// ===== 工具函数 =====

function statusBadgeClass(s: ChainStatus) {
  if (s === 'running') return 'bg-green-500/20 text-green-400 border border-green-500/40'
  if (s === 'error')   return 'bg-red-500/20 text-red-400 border border-red-500/40'
  return 'bg-gray-600/40 text-gray-400 border border-gray-600/40'
}

// 为 optional 步骤拼装带括号注释的 label
function optionalLabel(key: string): string {
  return `${t(key)} (${t('pipeline.optional')})`
}

function channelLabel(key: string): string {
  return channelNames.value[key] || t('dashboard.channelUnset')
}

function providerLabel(provider: string): string {
  const keyMap: Record<string, string> = {
    xunfei: 'xunfei',
    deepseek: 'deepseek',
    openai_compatible: 'openaiCompatible',
    system: 'systemTts',
    xunfei_voiceclone: 'xunfeiVoiceClone',
    simli: 'simli',
    python_bridge: 'pythonBridge',
    null: 'null',
  }
  const key = keyMap[provider] || provider
  return t(`settings.advanced.providerNames.${key}`, provider || t('dashboard.channelUnset'))
}

function deviceLabel(name?: string): string {
  return name && name.trim() ? name : t('dashboard.channelUnset')
}

// 切换 UI 语言并持久化（fire-and-forget）
async function switchLocale(code: 'zh-CN' | 'en-US') {
  i18n.global.locale.value = code
  try {
    // @ts-expect-error — Wails 运行时注入
    const cur = await window.go.main.App.GetConfig()
    // @ts-expect-error — Wails 运行时注入
    await window.go.main.App.SaveLocalConfig({ ...cur, ui_locale: code })
  } catch { /* 静默 */ }
}
</script>

<template>
  <div class="dashboard flex flex-col h-screen bg-[#0f1117] text-white select-none overflow-hidden">

    <!-- ===== 顶部状态栏 ===== -->
    <div class="flex items-center justify-between px-5 py-3 border-b border-white/5">
      <div class="flex items-center gap-2">
        <span class="w-2 h-2 rounded-full" :class="systemOk ? 'bg-green-400' : 'bg-yellow-400'" />
        <span class="text-sm font-medium" :class="systemOk ? 'text-gray-200' : 'text-yellow-300'">
          {{ systemOk ? t('dashboard.systemNormal') : t('dashboard.systemWarning') }}
        </span>
        <span class="hidden sm:inline text-gray-500 text-xs ml-1">
          {{ systemOk ? t('dashboard.systemDesc') : '' }}
        </span>
      </div>
      <div class="flex items-center gap-3">
        <!-- 语言切换 -->
        <select
          v-model="uiLocale"
          class="form-select-xs"
          @change="switchLocale(uiLocale)"
        >
          <option v-for="loc in UI_LOCALES" :key="loc.code" :value="loc.code">
            {{ loc.label }}
          </option>
        </select>
      </div>
    </div>

    <!-- ===== 主体 ===== -->
    <div class="flex-1 overflow-y-auto px-5 py-5">

      <!-- 面试链路标题 -->
      <div class="flex items-center justify-between mb-4">
        <div>
          <h2 class="text-xl font-bold text-white">{{ t('dashboard.pipelines') }}</h2>
          <p class="text-xs text-gray-500 mt-0.5">
            {{ runningCount > 0
              ? t('dashboard.pipelinesRunning', { n: runningCount })
              : t('dashboard.pipelinesIdle') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="w-8 h-8 flex items-center justify-center rounded-lg text-gray-400 hover:text-gray-200 hover:bg-white/5 transition-colors"
            @click="emit('openSettings')"
          >
            <Settings :size="16" />
          </button>
          <button
            class="w-8 h-8 flex items-center justify-center rounded-lg text-gray-400 hover:text-gray-200 hover:bg-white/5 transition-colors"
            @click="emit('openTeleprompter')"
          >
            <Maximize2 :size="16" />
          </button>
          <button
            class="w-8 h-8 flex items-center justify-center rounded-lg text-gray-400 hover:text-gray-200 hover:bg-white/5 transition-colors"
            :title="t('meetingGuide.title')"
            @click="showMeetingGuide = true"
          >
            <Info :size="16" />
          </button>
        </div>
      </div>

      <!-- 错误提示 -->
      <p v-if="errorMsg" class="text-red-400 text-xs mb-3 px-1">{{ errorMsg }}</p>

      <!-- ===== 听力链卡片 ===== -->
      <div class="pipeline-card mb-3 bg-[#161b27] border border-white/8 rounded-2xl p-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-xl bg-indigo-500/20 flex items-center justify-center flex-shrink-0">
              <Headphones :size="18" class="text-indigo-400" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <span class="font-semibold text-sm">{{ t('dashboard.hearingChain') }}</span>
                <span
                  class="px-1.5 py-0.5 text-[10px] font-bold rounded"
                  :class="statusBadgeClass(hearingStatus)"
                >
                  {{ t(`dashboard.status.${hearingStatus}`) }}
                </span>
              </div>
              <p class="text-xs text-gray-500 mt-0.5">{{ t('dashboard.hearingDesc') }}</p>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-xs text-gray-400 hidden sm:block">{{ hearingLangPair }}</span>
            <!-- 切换开关 -->
            <button
              class="relative w-11 h-6 rounded-full transition-colors duration-200 focus:outline-none"
              :class="hearingStatus === 'running' ? 'bg-blue-500' : 'bg-gray-600'"
              @click="toggleHearing(hearingStatus !== 'running')"
            >
              <span
                class="absolute top-0.5 left-0 w-5 h-5 rounded-full bg-white shadow transition-transform duration-200"
                :class="hearingStatus === 'running' ? 'translate-x-[22px]' : 'translate-x-0.5'"
              />
            </button>
          </div>
        </div>

        <!-- 面试业务流：先表达用户结果，再保留诊断链路 -->
        <div class="mt-4 space-y-2">
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.businessInput') }}</span>
            <PipelineStep
              :label="t('pipeline.interviewerAudio')"
              :channel="channelLabel('meetingAudio')"
              :active="hearingStatus === 'running'"
              configurable
              @configure="openDeviceConfig('meetingAudio')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.understandQuestion')"
              :channel="channelLabel('xunfeiTranslation')"
              :active="hearingStatus === 'running'"
            />
          </div>
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.userVisible') }}</span>
            <PipelineStep
              :label="t('pipeline.liveSubtitle')"
              :active="hearingStatus === 'running'"
            />
            <span class="text-gray-600 text-xs flex-shrink-0">{{ t('pipeline.plus') }}</span>
            <PipelineStep
              :label="t('pipeline.resumeAnswer')"
              :channel="channelLabel('ragDeepseek')"
              :active="hearingStatus === 'running'"
            />
          </div>
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.localOutput') }}</span>
            <PipelineStep
              :label="t('pipeline.headphoneTranslation')"
              :channel="channelLabel('systemTts')"
              configurable
              @configure="openDeviceConfig('monitorOutput')"
            />
          </div>
          <div class="business-lane opacity-75">
            <span class="lane-label">{{ t('dashboard.diagnostics') }}</span>
            <PipelineStep
              :label="t('pipeline.virtualMicBlackhole')"
              :channel="channelLabel('blackhole')"
              configurable
              @configure="openDeviceConfig('meetingAudio')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.xunfeiTranslation')"
              :channel="channelLabel('xunfeiTranslation')"
              :active="hearingStatus === 'running'"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.ragRetrieval')"
              :channel="channelLabel('localVectorStore')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.deepseekAnswer')"
              :channel="channelLabel('deepseek')"
            />
          </div>
        </div>
      </div>

      <!-- ===== 说话链卡片 ===== -->
      <div class="pipeline-card mb-3 bg-[#161b27] border border-white/8 rounded-2xl p-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-xl bg-purple-500/20 flex items-center justify-center flex-shrink-0">
              <Mic :size="18" class="text-purple-400" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <span class="font-semibold text-sm">{{ t('dashboard.speakingChain') }}</span>
                <span
                  class="px-1.5 py-0.5 text-[10px] font-bold rounded"
                  :class="statusBadgeClass(speakingStatus)"
                >
                  {{ t(`dashboard.status.${speakingStatus}`) }}
                </span>
              </div>
              <p class="text-xs text-gray-500 mt-0.5">{{ t('dashboard.speakingDesc') }}</p>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-xs text-gray-500 hidden sm:block">{{ speakingLangPair }}</span>
            <button
              class="relative w-11 h-6 rounded-full transition-colors duration-200 focus:outline-none"
              :class="speakingStatus === 'running' ? 'bg-purple-500' : 'bg-gray-600'"
              @click="toggleSpeaking(speakingStatus !== 'running')"
            >
              <span
                class="absolute top-0.5 left-0 w-5 h-5 rounded-full bg-white shadow transition-transform duration-200"
                :class="speakingStatus === 'running' ? 'translate-x-[22px]' : 'translate-x-0.5'"
              />
            </button>
          </div>
        </div>
        <div class="mt-4 space-y-2">
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.businessInput') }}</span>
            <PipelineStep
              :label="t('pipeline.yourNativeAnswer')"
              :channel="channelLabel('physicalMic')"
              :active="speakingStatus === 'running'"
              configurable
              @configure="openDeviceConfig('physicalMic')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.targetLanguageSpeech')"
              :channel="channelLabel('xunfeiTranslation')"
              :active="speakingStatus === 'running'"
            />
          </div>
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.userVisible') }}</span>
            <PipelineStep
              :label="t('pipeline.speechTransformReady')"
              :active="speakingStatus === 'running'"
            />
          </div>
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.meetingOutput') }}</span>
            <PipelineStep
              :label="t('pipeline.clonedVoice')"
              :channel="channelLabel('ttsOutput')"
              :active="speakingStatus === 'running'"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.virtualMic')"
              :channel="channelLabel('virtualMic')"
              :active="speakingStatus === 'running'"
              configurable
              @configure="openDeviceConfig('meetingAudio')"
            />
          </div>
          <div class="business-lane opacity-75">
            <span class="lane-label">{{ t('dashboard.diagnostics') }}</span>
            <PipelineStep
              :label="t('pipeline.physicalMic')"
              :channel="channelLabel('physicalMic')"
              configurable
              @configure="openDeviceConfig('physicalMic')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.xunfeiAsr')"
              :channel="channelLabel('xunfeiAsr')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="optionalLabel('pipeline.deepseekPolish')"
              :channel="channelLabel('deepseek')"
              optional
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.xunfeiVoiceClone')"
              :channel="channelLabel('xunfeiVoiceClone')"
            />
          </div>
        </div>
      </div>

      <!-- ===== 视频链卡片 ===== -->
      <div class="pipeline-card bg-[#161b27] border border-white/8 rounded-2xl p-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-xl bg-cyan-500/20 flex items-center justify-center flex-shrink-0">
              <Video :size="18" class="text-cyan-400" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <span class="font-semibold text-sm">{{ t('dashboard.videoChain') }}</span>
                <span
                  class="px-1.5 py-0.5 text-[10px] font-bold rounded"
                  :class="statusBadgeClass(videoStatus)"
                >
                  {{ t(`dashboard.status.${videoStatus}`) }}
                </span>
                <span
                  class="px-1.5 py-0.5 text-[10px] font-bold rounded border"
                  :class="circuitOpen
                    ? 'bg-yellow-500/20 text-yellow-300 border-yellow-500/40'
                    : 'bg-cyan-500/10 text-cyan-300 border-cyan-500/30'"
                >
                  {{ circuitOpen ? t('dashboard.videoDirectOpen') : t('dashboard.videoDirectNormal') }}
                </span>
              </div>
              <p class="text-xs text-gray-500 mt-0.5">{{ t('dashboard.videoDesc') }}</p>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <button
              class="relative w-11 h-6 rounded-full transition-colors duration-200 focus:outline-none"
              :class="videoStatus === 'running' ? 'bg-cyan-500' : 'bg-gray-600'"
              @click="toggleVideo(videoStatus !== 'running')"
            >
              <span
                class="absolute top-0.5 left-0 w-5 h-5 rounded-full bg-white shadow transition-transform duration-200"
                :class="videoStatus === 'running' ? 'translate-x-[22px]' : 'translate-x-0.5'"
              />
            </button>
          </div>
        </div>
        <div
          v-if="circuitOpen"
          class="mt-3 rounded-xl border border-yellow-500/25 bg-yellow-500/10 px-4 py-2 text-xs leading-relaxed text-yellow-100"
        >
          {{ t('dashboard.videoDirectDesc') }}
        </div>
        <div class="mt-4 space-y-2">
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.businessInput') }}</span>
            <PipelineStep
              :label="t('pipeline.cameraImage')"
              :channel="channelLabel('physicalCamera')"
              :active="videoStatus === 'running'"
              configurable
              @configure="openDeviceConfig('physicalCamera')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.lipSync')"
              :channel="channelLabel('simli')"
              :active="videoStatus === 'running'"
            />
          </div>
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.userVisible') }}</span>
            <PipelineStep
              :label="t('pipeline.videoSyncStatus')"
              :active="videoStatus === 'running'"
            />
          </div>
          <div class="business-lane">
            <span class="lane-label">{{ t('dashboard.meetingOutput') }}</span>
            <PipelineStep
              :label="t('pipeline.virtualCam')"
              :channel="channelLabel('virtualCamera')"
              :active="videoStatus === 'running'"
              configurable
              @configure="openDeviceConfig('virtualCamera')"
            />
          </div>
          <div class="business-lane opacity-75">
            <span class="lane-label">{{ t('dashboard.diagnostics') }}</span>
            <PipelineStep
              :label="t('pipeline.physicalCam')"
              :channel="channelLabel('physicalCamera')"
              configurable
              @configure="openDeviceConfig('physicalCamera')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.simliAvatar')"
              :channel="channelLabel('simli')"
            />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep
              :label="t('pipeline.virtualCam')"
              :channel="channelLabel('virtualCamera')"
              configurable
              @configure="openDeviceConfig('virtualCamera')"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- ===== 底部操作栏 ===== -->
    <div class="flex items-center justify-between px-5 py-3 border-t border-white/5 bg-[#0d1018]">
      <div class="flex items-center gap-2">
        <button
          class="flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-semibold transition-colors"
          :class="runningCount === 3
            ? 'bg-gray-700 text-gray-400 cursor-not-allowed'
            : 'bg-blue-600 hover:bg-blue-500 text-white'"
          :disabled="runningCount === 3"
          @click="startAll"
        >
          <Play :size="14" />
          {{ t('dashboard.startAll') }}
        </button>
        <button
          class="flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-semibold transition-colors"
          :class="runningCount === 0
            ? 'bg-gray-700/50 text-gray-600 cursor-not-allowed'
            : 'bg-gray-700 hover:bg-gray-600 text-gray-200'"
          :disabled="runningCount === 0"
          @click="stopAll"
        >
          <Square :size="14" />
          {{ t('dashboard.stopAll') }}
        </button>
      </div>
      <span class="text-xs text-gray-600">{{ t('dashboard.footerBrand', { version: t('dashboard.version') }) }}</span>
    </div>
  </div>

  <MeetingSetupGuide
    :open="showMeetingGuide"
    @close="showMeetingGuide = false"
  />

  <DeviceQuickConfig
    :open="deviceConfigTarget !== null"
    :target="deviceConfigTarget"
    @close="deviceConfigTarget = null"
    @saved="loadDashboardConfig"
  />

  <div
    v-if="showPreflightDialog"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/65 px-4 backdrop-blur-sm"
    @click.self="closePreflightDialog"
  >
    <div class="w-full max-w-lg rounded-2xl border border-white/10 bg-[#111827] shadow-2xl">
      <div class="flex items-start gap-3 border-b border-white/10 px-5 py-4">
        <div
          class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl"
          :class="startupIssues.length ? 'bg-red-500/15 text-red-300' : 'bg-yellow-500/15 text-yellow-200'"
        >
          <AlertTriangle :size="18" />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-white">
            {{ startupIssues.length ? t('dashboard.preflight.modalBlockedTitle') : t('dashboard.preflight.modalWarningTitle') }}
          </h3>
          <p class="mt-1 text-xs leading-relaxed text-gray-400">
            {{ startupIssues.length ? t('dashboard.preflight.modalBlockedDesc') : t('dashboard.preflight.modalWarningDesc') }}
          </p>
        </div>
      </div>

      <div class="space-y-4 px-5 py-4 text-xs leading-relaxed">
        <div v-if="startupIssues.length" class="rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-red-100">
          <p class="mb-2 font-semibold text-red-100">{{ t('dashboard.preflight.blockedTitle') }}</p>
          <ul class="list-disc space-y-1 pl-4">
            <li v-for="issue in startupIssues" :key="issue">{{ issue }}</li>
          </ul>
        </div>
        <div v-if="startupWarnings.length" class="rounded-xl border border-yellow-500/20 bg-yellow-500/10 px-4 py-3 text-yellow-100">
          <p class="mb-2 font-semibold">{{ t('dashboard.preflight.warningTitle') }}</p>
          <ul class="list-disc space-y-1 pl-4">
            <li v-for="warning in startupWarnings" :key="warning">{{ warning }}</li>
          </ul>
        </div>
      </div>

      <div class="flex items-center justify-end gap-2 border-t border-white/10 px-5 py-4">
        <button
          class="rounded-lg bg-white/5 px-4 py-2 text-xs font-semibold text-gray-200 transition-colors hover:bg-white/10"
          @click="closePreflightDialog"
        >
          {{ startupIssues.length ? t('common.close') : t('common.cancel') }}
        </button>
        <button
          v-if="startupIssues.length"
          class="rounded-lg bg-blue-600 px-4 py-2 text-xs font-semibold text-white transition-colors hover:bg-blue-500"
          @click="openSettingsFromPreflight"
        >
          {{ t('dashboard.preflight.openSettings') }}
        </button>
        <button
          v-else
          class="rounded-lg bg-blue-600 px-4 py-2 text-xs font-semibold text-white transition-colors hover:bg-blue-500"
          @click="continueStartAll"
        >
          {{ t('dashboard.preflight.continueStart') }}
        </button>
      </div>
    </div>
  </div>

</template>

<!-- ===== 管道步骤 chip 子组件 ===== -->
<script lang="ts">
import { defineComponent, h } from 'vue'

const PipelineStep = defineComponent({
  name: 'PipelineStep',
  props: {
    label:        { type: String,  required: true },
    channel:      { type: String,  default: '' },
    active:       { type: Boolean, default: false },
    optional:     { type: Boolean, default: false },
    configurable: { type: Boolean, default: false },
  },
  emits: ['configure'],
  setup(props, { emit }) {
    return () => h(props.configurable ? 'button' : 'span', {
      type: props.configurable ? 'button' : undefined,
      class: [
        'inline-flex flex-col justify-center px-2 rounded-md text-[11px] font-medium border leading-tight',
        props.channel ? 'py-1' : 'py-0.5',
        props.active
          ? 'bg-blue-500/20 text-blue-300 border-blue-500/40'
          : props.optional
            ? 'bg-transparent text-gray-500 border-dashed border-gray-600'
            : 'bg-white/5 text-gray-400 border-white/8',
        props.configurable
          ? 'cursor-pointer text-left transition-colors bg-blue-500/10 border-blue-400/35 hover:bg-blue-500/18 hover:border-blue-300/70 focus:outline-none focus:ring-1 focus:ring-blue-400/70'
          : '',
      ],
      onClick: props.configurable ? () => emit('configure') : undefined,
    }, [
      h('span', { class: 'inline-flex items-center gap-1 whitespace-nowrap' }, [
        props.configurable
          ? h(SlidersHorizontal, { size: 10, class: props.active ? 'text-blue-200' : 'text-blue-300' })
          : null,
        h('span', props.label),
      ]),
      props.channel
        ? h('span', {
          class: [
            'mt-0.5 text-[9px] font-normal whitespace-nowrap',
            props.active ? 'text-blue-200/70' : 'text-gray-500',
          ],
        }, props.channel)
        : null,
    ])
  },
})

export { PipelineStep }
</script>

<style scoped>
.business-lane {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.lane-label {
  width: 4.5rem;
  flex: none;
  color: #6b7280;
  font-size: 11px;
  font-weight: 600;
}
</style>
