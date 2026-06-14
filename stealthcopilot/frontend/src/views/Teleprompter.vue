<script lang="ts" setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Minus, X } from 'lucide-vue-next'
import { GetConfig, HideTeleprompter, SaveLocalConfig, TripCircuit } from '../../wailsjs/go/main/App'

defineOptions({ name: 'AppTeleprompter' })
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()

// 听力链字幕事件：后端 hearing/chain.go EventSubtitle = "hearing:subtitle"
const eventHearingSubtitle = 'hearing:subtitle'
// 听力链错误事件：讯飞重连耗尽时触发
const eventHearingError = 'hearing:error'
const eventAnswerToken = 'answer:token'
const eventAnswerDone = 'answer:done'
const eventHide = 'teleprompter:hide'
const eventCircuitOpen = 'circuit:open'
const eventCircuitClosed = 'circuit:closed'
const minFontSize = 13
const maxFontSize = 28
const minOpacity = 0.3
const maxOpacity = 1

const subtitles = ref<string[]>([])
const hearingError = ref('')
const answer = ref('')
const fontSize = ref(16)
const opacity = ref(0.85)
const minimized = ref(false)
const answering = ref(false)
const circuitOpen = ref(false)
const subtitleEl = ref<HTMLElement | null>(null)
const answerEl = ref<HTMLElement | null>(null)
const unlisteners: Array<() => void> = []

const panelStyle = computed(() => ({
  '--ghost-font-size': `${fontSize.value}px`,
  backgroundColor: `rgba(17, 24, 39, ${opacity.value})`,
}))

onMounted(async () => {
  await loadConfig()
  listenRuntimeEvents()
})

onUnmounted(() => {
  unlisteners.forEach((off) => off())
})

async function loadConfig() {
  try {
    const cfg = await GetConfig()
    fontSize.value = clamp(Number(cfg.ghost_font_size || fontSize.value), minFontSize, maxFontSize)
    opacity.value = clamp(Number(cfg.ghost_opacity || opacity.value), minOpacity, maxOpacity)
  } catch {
    subtitles.value = [t('teleprompter.subtitlePlaceholder')]
    answer.value = t('teleprompter.answerPlaceholder')
  }
}

function listenRuntimeEvents() {
  const runtimeApi = getRuntime()
  if (!runtimeApi) return

  unlisteners.push(runtimeApi.EventsOn(eventHearingSubtitle, appendSubtitle))
  unlisteners.push(runtimeApi.EventsOn(eventHearingError, (msg: string) => { hearingError.value = msg }))
  unlisteners.push(runtimeApi.EventsOn(eventAnswerToken, appendAnswerToken))
  unlisteners.push(runtimeApi.EventsOn(eventAnswerDone, finishAnswer))
  unlisteners.push(runtimeApi.EventsOn(eventHide, () => emit('close')))
  unlisteners.push(runtimeApi.EventsOn(eventCircuitOpen, () => { circuitOpen.value = true }))
  unlisteners.push(runtimeApi.EventsOn(eventCircuitClosed, () => { circuitOpen.value = false }))
}

function appendSubtitle(payload: string | { text?: string }) {
  const text = typeof payload === 'string' ? payload : payload?.text
  if (!text) return
  subtitles.value.push(text)
  nextTick(() => {
    if (subtitleEl.value) subtitleEl.value.scrollTop = subtitleEl.value.scrollHeight
  })
}

function appendAnswerToken(token: string) {
  if (!token) return
  answering.value = true
  answer.value += token
  nextTick(() => {
    if (answerEl.value) answerEl.value.scrollTop = answerEl.value.scrollHeight
  })
}

function finishAnswer() {
  answering.value = false
}

async function adjustFontSize(delta: number) {
  fontSize.value = clamp(fontSize.value + delta, minFontSize, maxFontSize)
  await persistAppearance()
}

async function onOpacityInput() {
  opacity.value = clamp(opacity.value, minOpacity, maxOpacity)
  await persistAppearance()
}

async function persistAppearance() {
  try {
    const cfg = await GetConfig()
    await SaveLocalConfig({
      ...cfg,
      ghost_font_size: fontSize.value,
      ghost_opacity: opacity.value,
    })
  } catch {
    // 浏览器预览或 Wails binding 不可用时只保留本地即时状态。
  }
}

async function closeTeleprompter() {
  try {
    await HideTeleprompter()
  } catch {
    emit('close')
  }
}

function toggleMinimized() {
  minimized.value = !minimized.value
}

async function tripCircuit() {
  try {
    await TripCircuit()
  } catch {
    // Wails binding 不可用时忽略
  }
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function getRuntime(): { EventsOn: (eventName: string, callback: (...data: any[]) => void) => () => void } | null {
  // @ts-expect-error — Wails 运行时注入
  return window.runtime || null
}
</script>

<template>
  <main class="min-h-screen bg-transparent text-gray-100">
    <button
      v-if="minimized"
      class="fixed right-4 bottom-4 h-[38px] px-4 rounded-full bg-gray-900/90 border border-cyan-400/50 shadow-lg shadow-cyan-500/10 flex items-center gap-2 text-sm"
      @click="toggleMinimized"
    >
      <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
      <span>{{ t('teleprompter.pill') }}</span>
    </button>

    <section
      v-else
      class="fixed right-4 bottom-4 w-[400px] h-[300px] rounded-lg border border-cyan-400/30 shadow-2xl shadow-cyan-500/10 overflow-hidden backdrop-blur-md flex flex-col"
      :style="panelStyle"
    >
      <header class="h-9 px-3 border-b border-white/10 flex items-center justify-between select-none">
        <div class="flex items-center gap-2 text-xs text-cyan-100">
          <span class="w-2 h-2 rounded-full bg-emerald-400" />
          <span>{{ t('teleprompter.title') }}</span>
        </div>
        <div class="flex items-center gap-1">
          <button
            class="w-7 h-7 rounded-md hover:bg-white/10 text-gray-300 flex items-center justify-center"
            :title="t('teleprompter.minimize')"
            @click="toggleMinimized"
          >
            <Minus :size="14" />
          </button>
          <button
            class="w-7 h-7 rounded-md hover:bg-white/10 text-gray-300 flex items-center justify-center"
            :title="t('teleprompter.close')"
            @click="closeTeleprompter"
          >
            <X :size="14" />
          </button>
        </div>
      </header>

      <!-- 听力链错误条：hearing:error 事件触发时显示红色提示 -->
      <div
        v-if="hearingError"
        class="flex items-center justify-between gap-2 px-3 py-1.5 bg-red-600/90 text-white text-xs"
      >
        <span>{{ hearingError }}</span>
        <button
          class="shrink-0 px-2 py-0.5 rounded bg-white/20 hover:bg-white/30 font-medium"
          @click="hearingError = ''"
        >
          {{ t('common.close') }}
        </button>
      </div>

      <!-- 熔断警告条：circuit:open 时显示橙色提示，提供紧急降级按钮 -->
      <div
        v-if="circuitOpen"
        class="flex items-center justify-between gap-2 px-3 py-1.5 bg-orange-500/90 text-white text-xs"
      >
        <span>{{ t('teleprompter.circuitOpenWarning') }}</span>
        <button
          class="shrink-0 px-2 py-0.5 rounded bg-white/20 hover:bg-white/30 font-medium"
          @click="tripCircuit"
        >
          {{ t('teleprompter.emergencyBypass') }}
        </button>
      </div>

      <div class="min-h-0 flex-1 grid grid-rows-2">
        <div
          ref="subtitleEl"
          class="overflow-y-auto px-4 py-3 border-b border-white/10 leading-relaxed"
          :style="{ fontSize: 'var(--ghost-font-size)' }"
        >
          <p
            v-for="(line, index) in subtitles"
            :key="index"
            class="mb-2 text-cyan-50"
          >
            {{ line }}
          </p>
        </div>

        <div
          ref="answerEl"
          class="overflow-y-auto px-4 py-3 leading-relaxed text-white"
          :style="{ fontSize: 'var(--ghost-font-size)' }"
        >
          <span>{{ answer }}</span>
          <span
            v-if="answering"
            class="inline-block w-2 h-5 ml-1 align-[-3px] bg-cyan-300 animate-pulse"
          />
        </div>
      </div>

      <footer class="h-11 px-3 border-t border-white/10 flex items-center gap-3 bg-black/15">
        <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
        <button
          class="w-8 h-8 rounded-md bg-white/5 hover:bg-white/10 text-sm"
          :disabled="fontSize <= minFontSize"
          @click="adjustFontSize(-1)"
        >
          A-
        </button>
        <button
          class="w-8 h-8 rounded-md bg-white/5 hover:bg-white/10 text-sm"
          :disabled="fontSize >= maxFontSize"
          @click="adjustFontSize(1)"
        >
          A+
        </button>
        <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
        <input
          v-model.number="opacity"
          class="flex-1 accent-cyan-300"
          type="range"
          :min="minOpacity"
          :max="maxOpacity"
          step="0.05"
          @input="onOpacityInput"
        >
        <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
        <span class="w-10 text-right text-xs text-gray-300">{{ Math.round(opacity * 100) }}%</span>
      </footer>
    </section>
  </main>
</template>
