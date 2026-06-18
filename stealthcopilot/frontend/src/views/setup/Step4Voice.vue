<script lang="ts" setup>
import { ref, computed, onUnmounted, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckCircle, SkipForward } from 'lucide-vue-next'

const { t } = useI18n()

type VoiceState = 'loading' | 'idle' | 'blocked' | 'recording' | 'recorded' | 'uploading' | 'submitted' | 'done' | 'failed' | 'error' | 'skipped'

const state = ref<VoiceState>('loading')
const countdown = ref(30)
const errorMsg = ref('')
const statusMsg = ref('')
const waveformActive = ref(false)
const sampleText = ref(t('setup.voice.sampleText'))

// 录音产出的 WAV 字节，由 Go binding 返回
let wavBytes: number[] | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null

const canUpload = computed(() => state.value === 'recorded' && wavBytes != null && wavBytes.length > 0)
const isDone = computed(() => state.value === 'done' || state.value === 'skipped' || state.value === 'submitted')
const canRecord = computed(() => state.value === 'idle' || state.value === 'error' || state.value === 'failed')
const canShowTrainingControls = computed(() => state.value !== 'blocked' && state.value !== 'loading')

onMounted(loadVoiceState)

async function loadVoiceState() {
  state.value = 'loading'
  errorMsg.value = ''
  statusMsg.value = ''
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const cfg = await window.go.main.App.GetConfig()
    if (cfg?.xunfei_tts_asset_id_set) {
      state.value = 'done'
      statusMsg.value = t('setup.voice.doneDesc')
    } else if (cfg?.xunfei_tts_task_id_set) {
      state.value = 'submitted'
      statusMsg.value = t('setup.voice.submittedDesc')
    }
    if (!cfg?.xunfei_tts_app_id_set || !cfg?.xunfei_tts_api_key_set || !cfg?.xunfei_tts_api_secret_set) {
      state.value = 'blocked'
      errorMsg.value = t('setup.voice.credentialsRequired')
      return
    }
    if (state.value !== 'done') {
      // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
      const trainText = await window.go.main.App.GetXunfeiVoiceTrainText()
      if (trainText?.text) sampleText.value = trainText.text
    }
  } catch (err: unknown) {
    state.value = 'blocked'
    errorMsg.value = t('setup.voice.trainTextError', { message: String(err) })
  } finally {
    if (state.value === 'loading') state.value = 'idle'
  }
}

async function startRecording() {
  errorMsg.value = ''
  statusMsg.value = ''
  wavBytes = null
  countdown.value = 30

  // @ts-expect-error — Wails 运行时注入
  const cfg = await window.go.main.App.GetConfig()
  const deviceName: string = cfg?.physical_mic_name ?? ''

  // @ts-expect-error — Wails 运行时注入
  const errMsg: string = await window.go.main.App.StartVoiceTrainingRecording(deviceName)
  if (errMsg) {
    errorMsg.value = errMsg
    state.value = 'error'
    return
  }

  state.value = 'recording'
  waveformActive.value = true

  countdownTimer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) void stopRecording()
  }, 1000)
}

async function stopRecording() {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  waveformActive.value = false

  // @ts-expect-error — Wails 运行时注入
  const res: { wav: number[] | null; err_msg: string } = await window.go.main.App.StopVoiceTrainingRecording()
  if (res.err_msg) {
    errorMsg.value = res.err_msg
    state.value = 'error'
    return
  }
  if (!res.wav || res.wav.length === 0) {
    errorMsg.value = t('setup.voice.micError')
    state.value = 'error'
    return
  }
  wavBytes = res.wav
  state.value = 'recorded'
}

function reRecord() {
  wavBytes = null
  countdown.value = 30
  errorMsg.value = ''
  statusMsg.value = ''
  state.value = 'idle'
}

async function uploadAndClone() {
  if (!wavBytes?.length) return
  state.value = 'uploading'
  errorMsg.value = ''
  statusMsg.value = ''

  try {
    // @ts-expect-error — Wails 运行时注入
    const errMsg = await window.go.main.App.CloneVoice(wavBytes)
    if (errMsg) {
      errorMsg.value = errMsg
      state.value = 'error'
    } else {
      state.value = 'submitted'
      statusMsg.value = t('setup.voice.submittedDesc')
    }
  } catch (err: unknown) {
    errorMsg.value = String(err)
    state.value = 'error'
  }
}

async function queryStatus() {
  state.value = 'uploading'
  errorMsg.value = ''
  try {
    // @ts-expect-error — Wails 运行时注入
    const result = await window.go.main.App.QueryXunfeiVoiceCloneStatus()
    statusMsg.value = result.message || ''
    switch (result.state) {
      case 'done':
        state.value = 'done'
        break
      case 'failed':
        state.value = 'failed'
        errorMsg.value = result.message || ''
        break
      case 'submitted':
      default:
        state.value = 'submitted'
        break
    }
  } catch (err: unknown) {
    errorMsg.value = String(err)
    state.value = 'error'
  }
}

function skip() {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  if (state.value === 'recording') {
    // 异步停止，跳过时不关心返回值
    // @ts-expect-error — Wails 运行时注入
    void window.go.main.App.StopVoiceTrainingRecording()
  }
  wavBytes = null
  state.value = 'skipped'
}

function restartClone() {
  wavBytes = null
  void loadVoiceState()
}

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer)
  if (state.value === 'recording') {
    // @ts-expect-error — Wails 运行时注入
    void window.go.main.App.StopVoiceTrainingRecording() // 组件卸载时静默停止
  }
})
</script>

<template>
  <div class="step4">
    <h2 class="text-xl font-bold mb-2 text-white">
      {{ t('setup.voice.title') }}
    </h2>
    <p class="text-gray-400 mb-4 text-sm">
      {{ t('setup.voice.desc') }}
    </p>

    <div
      v-if="isDone"
      class="result-banner flex items-center gap-3 bg-green-900/40 border border-green-500 rounded-xl px-5 py-4 mb-4"
    >
      <component
        :is="state === 'skipped' ? SkipForward : CheckCircle"
        :size="28"
        class="text-green-400"
      />
      <div>
        <p class="font-semibold text-green-300">
          {{ state === 'done' ? t('setup.voice.done') : state === 'submitted' ? t('setup.voice.submitted') : t('setup.voice.skipped') }}
        </p>
        <p class="text-xs text-gray-400 mt-0.5">
          {{ statusMsg || (state === 'skipped' ? t('setup.voice.skippedDesc') : t('setup.voice.doneDesc')) }}
        </p>
        <button
          v-if="state === 'skipped'"
          class="mt-3 px-4 py-1.5 bg-blue-500 hover:bg-blue-600 rounded-lg text-xs font-semibold text-white transition-colors"
          @click="restartClone"
        >
          {{ t('setup.voice.restart') }}
        </button>
      </div>
    </div>

    <template v-if="state !== 'done' && state !== 'skipped'">
      <p
        v-if="state === 'loading'"
        class="text-gray-400 text-sm mb-3"
      >
        {{ t('setup.voice.loading') }}
      </p>

      <div
        v-if="canShowTrainingControls"
        class="sample-text bg-gray-700 rounded-xl p-4 mb-5 text-sm text-gray-200 leading-relaxed"
      >
        <p class="text-xs text-gray-500 mb-2">
          {{ t('setup.voice.readAloud') }}
        </p>
        {{ sampleText }}
      </div>

      <div
        v-if="canShowTrainingControls"
        class="waveform flex items-end gap-1 h-12 mb-5 justify-center"
      >
        <div
          v-for="i in 20"
          :key="i"
          class="w-1.5 rounded-full bg-blue-400 transition-all duration-100"
          :class="waveformActive ? 'animate-[wave_0.8s_ease-in-out_infinite]' : 'h-2 opacity-30'"
          :style="waveformActive ? `animation-delay: ${(i % 5) * 0.1}s; height: ${(i * 17) % 36 + 8}px` : ''"
        />
      </div>

      <div
        v-if="state === 'recording'"
        class="countdown text-center text-4xl font-mono text-blue-300 mb-4"
      >
        {{ t('setup.voice.countdown', { seconds: countdown }) }}
      </div>

      <p
        v-if="errorMsg"
        class="text-red-400 text-sm mb-3"
      >
        {{ errorMsg }}
      </p>

      <div class="actions flex gap-3 flex-wrap">
        <button
          v-if="canRecord && canShowTrainingControls"
          class="px-6 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg font-semibold transition-colors"
          @click="startRecording"
        >
          {{ t('setup.voice.record') }}
        </button>
        <button
          v-if="state === 'recording' && canShowTrainingControls"
          class="px-6 py-2 bg-red-500 hover:bg-red-600 rounded-lg font-semibold transition-colors"
          @click="stopRecording"
        >
          {{ t('setup.voice.stop') }}
        </button>
        <button
          v-if="state === 'recorded' && canShowTrainingControls"
          class="px-4 py-2 bg-gray-600 hover:bg-gray-500 rounded-lg font-semibold transition-colors"
          @click="reRecord"
        >
          {{ t('setup.voice.reRecord') }}
        </button>
        <button
          v-if="canUpload && canShowTrainingControls"
          class="px-6 py-2 bg-green-500 hover:bg-green-600 rounded-lg font-semibold transition-colors"
          @click="uploadAndClone"
        >
          {{ t('setup.voice.upload') }}
        </button>
        <button
          v-if="state === 'submitted' && canShowTrainingControls"
          class="px-6 py-2 bg-green-500 hover:bg-green-600 rounded-lg font-semibold transition-colors"
          @click="queryStatus"
        >
          {{ t('setup.voice.query') }}
        </button>
        <button
          v-if="state === 'uploading'"
          class="px-6 py-2 bg-gray-600 rounded-lg font-semibold cursor-not-allowed opacity-60"
          disabled
        >
          {{ t('setup.voice.uploading') }}
        </button>
        <button
          v-if="state !== 'uploading'"
          class="px-4 py-2 text-gray-400 hover:text-gray-200 underline text-sm transition-colors"
          @click="skip"
        >
          {{ t('common.skip') }}
        </button>
      </div>
    </template>
  </div>
</template>
