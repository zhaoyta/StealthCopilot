<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Headphones, Mic, Video, Settings, Maximize2, Info, Play, Square, ChevronRight } from 'lucide-vue-next'
import {
  StartHearingChain,
  StopHearingChain,
  StartSpeakingChain,
  StopSpeakingChain,
  StartVideoChain,
  StopVideoChain,
  GetConfig,
  HideTeleprompter,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import i18n from '../i18n'

defineOptions({ name: 'AppDashboard' })
const { t } = useI18n()

const UI_LOCALES = [
  { code: 'zh-CN', label: '中文' },
  { code: 'en-US', label: 'English' },
]

const emit = defineEmits<{
  (e: 'openSettings'): void
  (e: 'openTeleprompter'): void
}>()

type ChainStatus = 'idle' | 'running' | 'error'

const hearingStatus = ref<ChainStatus>('idle')
const speakingStatus = ref<ChainStatus>('idle')
const videoStatus = ref<ChainStatus>('idle')
const circuitOpen = ref(false)
const errorMsg = ref('')
const hearingLangPair = ref('')
const speakingLangPair = ref('')
const uiLocale = ref<'zh-CN' | 'en-US'>('zh-CN')

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
  if (videoStatus.value !== 'running') await toggleVideo(true)
  if (speakingStatus.value !== 'running') await toggleSpeaking(true)
  if (hearingStatus.value !== 'running') await toggleHearing(true)
}

async function stopAll() {
  if (hearingStatus.value === 'running') await toggleHearing(false)
  if (speakingStatus.value === 'running') await toggleSpeaking(false)
  if (videoStatus.value === 'running') await toggleVideo(false)
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
  try {
    const cfg = await GetConfig()
    uiLocale.value         = (cfg.ui_locale || 'zh-CN') as 'zh-CN' | 'en-US'
    hearingLangPair.value  = `${langLabel(cfg.hearing_source_lang || 'en')} → ${langLabel(cfg.hearing_target_lang || 'zh')}`
    speakingLangPair.value = `${langLabel(cfg.speaking_input_lang || 'zh')} → ${langLabel(cfg.speaking_output_lang || 'en')}`
  } catch { /* 静默 */ }
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
        <span class="text-xs text-gray-500">{{ t('dashboard.circuitStatusLabel') }}</span>
        <span
          class="px-2 py-0.5 rounded text-xs font-medium"
          :class="circuitOpen
            ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/40'
            : 'bg-green-500/20 text-green-400 border border-green-500/40'"
        >
          {{ circuitOpen ? t('dashboard.circuitOpen') : t('dashboard.circuitNormal') }}
        </span>
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

      <!-- 管道区标题 -->
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

        <!-- 管道步骤流（讯飞输出三路） -->
        <div class="mt-3 space-y-1.5">
          <!-- 主路：字幕 + 中文语音 -->
          <div class="flex items-center gap-1.5 flex-wrap">
            <PipelineStep :label="t('pipeline.virtualMicBlackhole')" />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep :label="t('pipeline.xunfeiTranslation')" :active="hearingStatus === 'running'" />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep :label="t('pipeline.chineseSubtitle')" />
            <span class="text-gray-600 text-xs flex-shrink-0">{{ t('pipeline.plus') }}</span>
            <PipelineStep :label="t('pipeline.chineseAudio')" />
          </div>
          <!-- 支路：回答建议 -->
          <div class="flex items-center gap-1.5 flex-wrap pl-[5.5rem]">
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep :label="t('pipeline.ragRetrieval')" />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep :label="t('pipeline.deepseekAnswer')" />
            <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
            <PipelineStep :label="t('pipeline.answerSuggestion')" />
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
        <div class="flex items-center gap-1.5 mt-3 flex-wrap">
          <PipelineStep :label="t('pipeline.physicalMic')" />
          <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
          <PipelineStep :label="t('pipeline.xunfeiAsr')" />
          <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
          <PipelineStep :label="optionalLabel('pipeline.deepseekPolish')" optional />
          <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
          <PipelineStep label="ElevenLabs TTS" />
          <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
          <PipelineStep :label="t('pipeline.virtualMic')" />
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
        <div class="flex items-center gap-1.5 mt-3 flex-wrap">
          <PipelineStep :label="t('pipeline.physicalCam')" />
          <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
          <PipelineStep :label="t('pipeline.simliAvatar')" />
          <ChevronRight :size="12" class="text-gray-600 flex-shrink-0" />
          <PipelineStep :label="t('pipeline.virtualCam')" />
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

</template>

<!-- ===== 管道步骤 chip 子组件 ===== -->
<script lang="ts">
import { defineComponent, h } from 'vue'

const PipelineStep = defineComponent({
  name: 'PipelineStep',
  props: {
    label:    { type: String,  required: true },
    active:   { type: Boolean, default: false },
    optional: { type: Boolean, default: false },
  },
  setup(props) {
    return () => h('span', {
      class: [
        'inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-medium border',
        props.active
          ? 'bg-blue-500/20 text-blue-300 border-blue-500/40'
          : props.optional
            ? 'bg-transparent text-gray-500 border-dashed border-gray-600'
            : 'bg-white/5 text-gray-400 border-white/8',
      ],
    }, props.label)
  },
})

export { PipelineStep }
</script>
