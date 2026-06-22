<script lang="ts" setup>
import { useI18n } from 'vue-i18n'
import { ref } from 'vue'
import { Headphones, Mic, Video, X } from 'lucide-vue-next'
import ObsSetupGuide from './ObsSetupGuide.vue'

defineOptions({ name: 'MeetingSetupGuide' })

defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const showObsGuide = ref(false)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/55 px-4"
    @click.self="emit('close')"
  >
    <section class="w-full max-w-2xl rounded-2xl border border-gray-700 bg-gray-900 text-white shadow-2xl">
      <header class="flex items-center justify-between border-b border-gray-700 px-5 py-4">
        <div>
          <h2 class="text-base font-semibold text-white">
            {{ t('meetingGuide.title') }}
          </h2>
          <p class="mt-1 text-xs text-gray-400">
            {{ t('meetingGuide.subtitle') }}
          </p>
        </div>
        <button
          class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
          :title="t('common.close')"
          @click="emit('close')"
        >
          <X :size="16" />
        </button>
      </header>

      <div class="space-y-5 px-5 py-5">
        <div class="grid gap-3 sm:grid-cols-3">
          <div class="guide-item">
            <Headphones :size="17" class="text-indigo-300" />
            <div>
              <p class="guide-title">{{ t('meetingGuide.speaker.title') }}</p>
              <p class="guide-text">{{ t('meetingGuide.speaker.desc') }}</p>
            </div>
          </div>
          <div class="guide-item">
            <Mic :size="17" class="text-purple-300" />
            <div>
              <p class="guide-title">{{ t('meetingGuide.mic.title') }}</p>
              <p class="guide-text">{{ t('meetingGuide.mic.desc') }}</p>
            </div>
          </div>
          <div class="guide-item">
            <Video :size="17" class="text-cyan-300" />
            <div>
              <p class="guide-title">{{ t('meetingGuide.camera.title') }}</p>
              <p class="guide-text">{{ t('meetingGuide.camera.desc') }}</p>
            </div>
          </div>
        </div>

        <div class="rounded-xl border border-gray-700 bg-gray-950/60 p-4">
          <h3 class="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-400">
            {{ t('meetingGuide.routingTitle') }}
          </h3>
          <div class="space-y-2 text-sm text-gray-300">
            <p>{{ t('meetingGuide.routes.hearing') }}</p>
            <p>{{ t('meetingGuide.routes.speaking') }}</p>
            <p>{{ t('meetingGuide.routes.video') }}</p>
          </div>
          <button
            class="mt-4 inline-flex items-center gap-2 rounded-lg border border-cyan-500/40 px-3 py-2 text-xs font-medium text-cyan-200 transition-colors hover:border-cyan-300 hover:bg-cyan-500/10"
            type="button"
            @click="showObsGuide = true"
          >
            <Video :size="14" />
            {{ t('meetingGuide.obsHelp') }}
          </button>
        </div>

        <div class="rounded-xl border border-yellow-500/20 bg-yellow-500/10 p-4 text-xs leading-relaxed text-yellow-100">
          {{ t('meetingGuide.monitorNote') }}
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div class="rounded-xl border border-gray-700 bg-gray-800/70 p-4">
            <h3 class="guide-section-title">{{ t('meetingGuide.feishu.title') }}</h3>
            <p class="guide-text">{{ t('meetingGuide.feishu.desc') }}</p>
          </div>
          <div class="rounded-xl border border-gray-700 bg-gray-800/70 p-4">
            <h3 class="guide-section-title">{{ t('meetingGuide.tencent.title') }}</h3>
            <p class="guide-text">{{ t('meetingGuide.tencent.desc') }}</p>
          </div>
        </div>
      </div>
    </section>

    <ObsSetupGuide
      :open="showObsGuide"
      @close="showObsGuide = false"
    />
  </div>
</template>

<style scoped>
.guide-item {
  display: flex;
  gap: 0.625rem;
  border: 1px solid rgb(55 65 81);
  border-radius: 0.75rem;
  background: rgb(31 41 55 / 0.7);
  padding: 0.875rem;
}

.guide-title {
  color: rgb(243 244 246);
  font-size: 0.8125rem;
  font-weight: 600;
}

.guide-text {
  margin-top: 0.25rem;
  color: rgb(156 163 175);
  font-size: 0.75rem;
  line-height: 1.5;
}

.guide-section-title {
  color: rgb(229 231 235);
  font-size: 0.8125rem;
  font-weight: 600;
}
</style>
