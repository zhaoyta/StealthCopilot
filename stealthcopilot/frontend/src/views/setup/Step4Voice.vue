<script lang="ts" setup>
import { ref, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckCircle, SkipForward } from 'lucide-vue-next'

const { t } = useI18n()

type VoiceState = 'idle' | 'recording' | 'recorded' | 'uploading' | 'done' | 'error' | 'skipped'

const state = ref<VoiceState>('idle')
const countdown = ref(15)
const errorMsg = ref('')
const waveformActive = ref(false)

let mediaRecorder: MediaRecorder | null = null
let recordedChunks: Blob[] = []
let countdownTimer: ReturnType<typeof setInterval> | null = null

const canUpload = computed(() => state.value === 'recorded')
const isDone = computed(() => state.value === 'done' || state.value === 'skipped')

// 示例文本，用户需朗读约 15 秒
const sampleText = t('setup.voice.sampleText')

async function startRecording() {
  errorMsg.value = ''
  recordedChunks = []
  countdown.value = 15

  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    mediaRecorder = new MediaRecorder(stream)
    mediaRecorder.ondataavailable = (e) => {
      if (e.data.size > 0) recordedChunks.push(e.data)
    }
    mediaRecorder.onstop = () => {
      stream.getTracks().forEach(t => t.stop())
      state.value = 'recorded'
      waveformActive.value = false
    }
    mediaRecorder.start(100)
    state.value = 'recording'
    waveformActive.value = true

    // 倒计时
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
  mediaRecorder?.stop()
  waveformActive.value = false
}

async function uploadAndClone() {
  if (!recordedChunks.length) return
  state.value = 'uploading'
  errorMsg.value = ''

  try {
    const blob = new Blob(recordedChunks, { type: 'audio/webm' })
    const arrayBuf = await blob.arrayBuffer()
    const uint8 = Array.from(new Uint8Array(arrayBuf))

    // @ts-expect-error — Wails 运行时注入，window.go/window.runtime 无类型定义
    const errMsg = await window.go.main.App.CloneVoice(uint8)
    if (errMsg) {
      errorMsg.value = errMsg
      state.value = 'error'
    } else {
      state.value = 'done'
    }
  } catch (err: unknown) {
    errorMsg.value = String(err)
    state.value = 'error'
  }
}

function skip() {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  mediaRecorder?.stop()
  state.value = 'skipped'
}

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer)
  if (mediaRecorder?.state === 'recording') mediaRecorder.stop()
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

    <!-- 结果状态 -->
    <div
      v-if="isDone"
      class="result-banner flex items-center gap-3 bg-green-900/40 border border-green-500 rounded-xl px-5 py-4 mb-4"
    >
      <component
        :is="state === 'done' ? CheckCircle : SkipForward"
        :size="28"
        class="text-green-400"
      />
      <div>
        <p class="font-semibold text-green-300">
          {{ state === 'done' ? t('setup.voice.done') : t('setup.voice.skipped') }}
        </p>
        <p class="text-xs text-gray-400 mt-0.5">
          {{ state === 'done' ? t('setup.voice.doneDesc') : t('setup.voice.skippedDesc') }}
        </p>
      </div>
    </div>

    <template v-else>
      <!-- 示例文本 -->
      <div class="sample-text bg-gray-700 rounded-xl p-4 mb-5 text-sm text-gray-200 leading-relaxed">
        <p class="text-xs text-gray-500 mb-2">
          {{ t('setup.voice.readAloud') }}
        </p>
        {{ sampleText }}
      </div>

      <!-- 波形动画 -->
      <div class="waveform flex items-end gap-1 h-12 mb-5 justify-center">
        <div
          v-for="i in 20"
          :key="i"
          class="w-1.5 rounded-full bg-blue-400 transition-all duration-100"
          :class="waveformActive
            ? 'animate-[wave_0.8s_ease-in-out_infinite]'
            : 'h-2 opacity-30'"
          :style="waveformActive
            ? `animation-delay: ${(i % 5) * 0.1}s; height: ${Math.random() * 36 + 8}px`
            : ''"
        />
      </div>

      <!-- 倒计时 -->
      <div
        v-if="state === 'recording'"
        class="countdown text-center text-4xl font-mono text-blue-300 mb-4"
      >
        <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
        {{ countdown }}s
      </div>

      <!-- 错误提示 -->
      <p
        v-if="errorMsg"
        class="text-red-400 text-sm mb-3"
      >
        {{ errorMsg }}
      </p>

      <!-- 操作按钮 -->
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
