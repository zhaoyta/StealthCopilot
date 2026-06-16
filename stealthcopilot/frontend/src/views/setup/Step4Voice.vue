<script lang="ts" setup>
import { ref, computed, onUnmounted, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckCircle, SkipForward } from 'lucide-vue-next'

const { t } = useI18n()

type VoiceState = 'idle' | 'recording' | 'recorded' | 'uploading' | 'submitted' | 'done' | 'error' | 'skipped'

const state = ref<VoiceState>('idle')
const countdown = ref(30)
const errorMsg = ref('')
const statusMsg = ref('')
const waveformActive = ref(false)
const sampleText = ref(t('setup.voice.sampleText'))

let audioContext: AudioContext | null = null
let sourceNode: MediaStreamAudioSourceNode | null = null
let processorNode: ScriptProcessorNode | null = null
let mediaStream: MediaStream | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null
let recordedBuffers: Float32Array[] = []
let recordedLength = 0
let recordedSampleRate = 44100
let wavBytes: Uint8Array | null = null

const canUpload = computed(() => state.value === 'recorded')
const isDone = computed(() => state.value === 'done' || state.value === 'skipped' || state.value === 'submitted')

onMounted(async () => {
  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const trainText = await window.go.main.App.GetXunfeiVoiceTrainText()
    if (trainText?.text) sampleText.value = trainText.text
  } catch {
    // 使用内置文案兜底，后端提交时仍会获取讯飞训练文本并校验。
  }
})

async function startRecording() {
  errorMsg.value = ''
  statusMsg.value = ''
  recordedBuffers = []
  recordedLength = 0
  wavBytes = null
  countdown.value = 30

  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true })
    audioContext = new AudioContext()
    recordedSampleRate = audioContext.sampleRate
    sourceNode = audioContext.createMediaStreamSource(mediaStream)
    processorNode = audioContext.createScriptProcessor(4096, 1, 1)
    processorNode.onaudioprocess = (event) => {
      const input = event.inputBuffer.getChannelData(0)
      const copy = new Float32Array(input.length)
      copy.set(input)
      recordedBuffers.push(copy)
      recordedLength += copy.length
    }
    sourceNode.connect(processorNode)
    processorNode.connect(audioContext.destination)
    state.value = 'recording'
    waveformActive.value = true

    countdownTimer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0) stopRecording()
    }, 1000)
  } catch {
    errorMsg.value = t('setup.voice.micError')
    state.value = 'error'
  }
}

function stopRecording() {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  processorNode?.disconnect()
  sourceNode?.disconnect()
  mediaStream?.getTracks().forEach(track => track.stop())
  audioContext?.close()
  processorNode = null
  sourceNode = null
  mediaStream = null
  audioContext = null
  waveformActive.value = false
  wavBytes = encodeWav(recordedBuffers, recordedLength, recordedSampleRate)
  state.value = 'recorded'
}

async function uploadAndClone() {
  if (!wavBytes?.length) return
  state.value = 'uploading'
  errorMsg.value = ''
  statusMsg.value = ''

  try {
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const errMsg = await window.go.main.App.CloneVoice(Array.from(wavBytes))
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
    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const result = await window.go.main.App.QueryXunfeiVoiceCloneStatus()
    if (result.ok) {
      statusMsg.value = result.message || t('setup.voice.doneDesc')
      state.value = 'done'
    } else {
      statusMsg.value = result.message || ''
      state.value = 'submitted'
    }
  } catch (err: unknown) {
    errorMsg.value = String(err)
    state.value = 'error'
  }
}

function skip() {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  if (state.value === 'recording') stopRecording()
  state.value = 'skipped'
}

function encodeWav(buffers: Float32Array[], length: number, sampleRate: number): Uint8Array {
  const samples = new Float32Array(length)
  let offset = 0
  for (const buffer of buffers) {
    samples.set(buffer, offset)
    offset += buffer.length
  }
  const dataSize = samples.length * 2
  const wav = new ArrayBuffer(44 + dataSize)
  const view = new DataView(wav)
  writeString(view, 0, 'RIFF')
  view.setUint32(4, 36 + dataSize, true)
  writeString(view, 8, 'WAVE')
  writeString(view, 12, 'fmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true)
  view.setUint16(22, 1, true)
  view.setUint32(24, sampleRate, true)
  view.setUint32(28, sampleRate * 2, true)
  view.setUint16(32, 2, true)
  view.setUint16(34, 16, true)
  writeString(view, 36, 'data')
  view.setUint32(40, dataSize, true)
  let pos = 44
  for (let i = 0; i < samples.length; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]))
    view.setInt16(pos, s < 0 ? s * 0x8000 : s * 0x7fff, true)
    pos += 2
  }
  return new Uint8Array(wav)
}

function writeString(view: DataView, offset: number, value: string) {
  for (let i = 0; i < value.length; i++) view.setUint8(offset + i, value.charCodeAt(i))
}

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer)
  mediaStream?.getTracks().forEach(track => track.stop())
  processorNode?.disconnect()
  sourceNode?.disconnect()
  audioContext?.close()
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
      </div>
    </div>

    <template v-if="state !== 'done' && state !== 'skipped'">
      <div class="sample-text bg-gray-700 rounded-xl p-4 mb-5 text-sm text-gray-200 leading-relaxed">
        <p class="text-xs text-gray-500 mb-2">
          {{ t('setup.voice.readAloud') }}
        </p>
        {{ sampleText }}
      </div>

      <div class="waveform flex items-end gap-1 h-12 mb-5 justify-center">
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
          v-if="state === 'idle' || state === 'error'"
          class="px-6 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg font-semibold transition-colors"
          @click="startRecording"
        >
          {{ t('setup.voice.record') }}
        </button>
        <button
          v-if="state === 'recording'"
          class="px-6 py-2 bg-red-500 hover:bg-red-600 rounded-lg font-semibold transition-colors"
          @click="stopRecording"
        >
          {{ t('setup.voice.stop') }}
        </button>
        <button
          v-if="canUpload"
          class="px-6 py-2 bg-green-500 hover:bg-green-600 rounded-lg font-semibold transition-colors"
          @click="uploadAndClone"
        >
          {{ t('setup.voice.upload') }}
        </button>
        <button
          v-if="state === 'submitted'"
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
