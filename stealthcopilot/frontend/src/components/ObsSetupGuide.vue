<script lang="ts" setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckCircle2, Copy, ExternalLink, MonitorUp, RefreshCw, Video, X } from 'lucide-vue-next'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'

defineOptions({ name: 'ObsSetupGuide' })

const props = withDefaults(defineProps<{
  open: boolean
  sourceUrl?: string
}>(), {
  sourceUrl: 'http://127.0.0.1:18765/',
})

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const copied = ref(false)
const obsUrl = computed(() => props.sourceUrl || 'http://127.0.0.1:18765/')

function stepIndex(n: number): string {
  return `${n}.`
}

async function copyObsUrl() {
  try {
    await navigator.clipboard.writeText(obsUrl.value)
    copied.value = true
    window.setTimeout(() => { copied.value = false }, 1400)
  } catch {
    copied.value = false
  }
}

function openObsDownload() {
  BrowserOpenURL('https://obsproject.com/download')
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4"
    @click.self="emit('close')"
  >
    <section class="max-h-[88vh] w-full max-w-3xl overflow-hidden rounded-2xl border border-gray-700 bg-gray-900 text-white shadow-2xl">
      <header class="flex items-start justify-between gap-4 border-b border-gray-700 px-5 py-4">
        <div>
          <h2 class="text-base font-semibold text-white">
            {{ t('obsGuide.title') }}
          </h2>
          <p class="mt-1 text-xs leading-relaxed text-gray-400">
            {{ t('obsGuide.subtitle') }}
          </p>
        </div>
        <button
          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
          :title="t('common.close')"
          @click="emit('close')"
        >
          <X :size="16" />
        </button>
      </header>

      <div class="max-h-[calc(88vh-73px)] overflow-y-auto px-5 py-5 text-left">
        <div class="grid gap-3 sm:grid-cols-3">
          <div class="obs-card">
            <Video :size="17" class="text-cyan-300" />
            <div>
              <p class="obs-card-title">{{ t('obsGuide.cards.source.title') }}</p>
              <p class="obs-card-text">{{ t('obsGuide.cards.source.desc') }}</p>
            </div>
          </div>
          <div class="obs-card">
            <MonitorUp :size="17" class="text-indigo-300" />
            <div>
              <p class="obs-card-title">{{ t('obsGuide.cards.camera.title') }}</p>
              <p class="obs-card-text">{{ t('obsGuide.cards.camera.desc') }}</p>
            </div>
          </div>
          <div class="obs-card">
            <RefreshCw :size="17" class="text-emerald-300" />
            <div>
              <p class="obs-card-title">{{ t('obsGuide.cards.refresh.title') }}</p>
              <p class="obs-card-text">{{ t('obsGuide.cards.refresh.desc') }}</p>
            </div>
          </div>
        </div>

        <div class="mt-5 rounded-xl border border-cyan-500/20 bg-cyan-500/10 p-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p class="text-xs font-semibold uppercase tracking-wide text-cyan-200">
                {{ t('obsGuide.sourceUrlLabel') }}
              </p>
              <p class="mt-1 font-mono text-sm text-cyan-50">
                {{ obsUrl }}
              </p>
            </div>
            <button
              class="inline-flex items-center justify-center gap-2 rounded-lg bg-cyan-500 px-3 py-2 text-xs font-semibold text-gray-950 transition-colors hover:bg-cyan-400"
              type="button"
              @click="copyObsUrl"
            >
              <CheckCircle2 v-if="copied" :size="14" />
              <Copy v-else :size="14" />
              {{ copied ? t('obsGuide.copied') : t('obsGuide.copyUrl') }}
            </button>
          </div>
        </div>

        <div class="mt-5 grid gap-4 lg:grid-cols-[1.05fr_0.95fr]">
          <div class="rounded-xl border border-gray-700 bg-gray-950/60 p-4">
            <h3 class="obs-section-title">{{ t('obsGuide.stepsTitle') }}</h3>
            <ol class="mt-3 space-y-3 text-xs text-gray-300">
              <li
                v-for="n in 7"
                :key="n"
                class="flex gap-2"
              >
                <span class="obs-step-index">{{ stepIndex(n) }}</span>
                <span>{{ t(`obsGuide.steps.${n}`) }}</span>
              </li>
            </ol>
            <button
              class="mt-4 inline-flex items-center gap-2 rounded-lg border border-gray-600 px-3 py-2 text-xs text-gray-200 transition-colors hover:border-cyan-400 hover:text-cyan-200"
              type="button"
              @click="openObsDownload"
            >
              <ExternalLink :size="14" />
              {{ t('obsGuide.downloadObs') }}
            </button>
          </div>

          <div class="space-y-3">
            <div class="rounded-xl border border-yellow-500/20 bg-yellow-500/10 p-4">
              <h3 class="obs-section-title text-yellow-100">{{ t('obsGuide.notesTitle') }}</h3>
              <ul class="mt-3 space-y-2 text-xs leading-relaxed text-yellow-50/90">
                <li>{{ t('obsGuide.notes.useRoot') }}</li>
                <li>{{ t('obsGuide.notes.keepRunning') }}</li>
                <li>{{ t('obsGuide.notes.fitToScreen') }}</li>
              </ul>
            </div>

            <div class="rounded-xl border border-gray-700 bg-gray-800/70 p-4">
              <h3 class="obs-section-title">{{ t('obsGuide.troubleshootingTitle') }}</h3>
              <div class="mt-3 space-y-3 text-xs leading-relaxed text-gray-300">
                <p><span class="font-semibold text-gray-100">{{ t('obsGuide.troubleshooting.unavailable.title') }}</span>{{ t('obsGuide.troubleshooting.unavailable.desc') }}</p>
                <p><span class="font-semibold text-gray-100">{{ t('obsGuide.troubleshooting.black.title') }}</span>{{ t('obsGuide.troubleshooting.black.desc') }}</p>
                <p><span class="font-semibold text-gray-100">{{ t('obsGuide.troubleshooting.flicker.title') }}</span>{{ t('obsGuide.troubleshooting.flicker.desc') }}</p>
                <p><span class="font-semibold text-gray-100">{{ t('obsGuide.troubleshooting.sync.title') }}</span>{{ t('obsGuide.troubleshooting.sync.desc') }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.obs-card {
  display: flex;
  gap: 0.625rem;
  border: 1px solid rgb(55 65 81);
  border-radius: 0.75rem;
  background: rgb(31 41 55 / 0.7);
  padding: 0.875rem;
}

.obs-card-title {
  color: rgb(243 244 246);
  font-size: 0.8125rem;
  font-weight: 600;
}

.obs-card-text {
  margin-top: 0.25rem;
  color: rgb(156 163 175);
  font-size: 0.75rem;
  line-height: 1.5;
}

.obs-section-title {
  color: rgb(229 231 235);
  font-size: 0.8125rem;
  font-weight: 600;
}

.obs-step-index {
  color: rgb(34 211 238);
  flex-shrink: 0;
  font-weight: 700;
}
</style>
