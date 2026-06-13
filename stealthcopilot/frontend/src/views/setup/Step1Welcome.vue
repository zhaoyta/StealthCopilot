<script lang="ts" setup>
import { useI18n } from 'vue-i18n'
import { Headphones, Mic, Video } from 'lucide-vue-next'
const { t } = useI18n()
</script>

<template>
  <div class="step1 text-center">
    <h2 class="text-2xl font-bold mb-4 text-white">
      {{ t('setup.welcome.title') }}
    </h2>
    <p class="text-gray-400 mb-10 text-base leading-relaxed">
      {{ t('setup.welcome.desc') }}
    </p>

    <!-- 三管道图标说明 -->
    <div class="pipeline-icons grid grid-cols-3 gap-6">
      <div class="pipeline-card bg-gray-700 rounded-xl p-5 flex flex-col items-center gap-3">
        <Headphones
          :size="32"
          class="text-blue-300"
        />
        <h3 class="font-semibold text-blue-300">
          {{ t('dashboard.hearingChain') }}
        </h3>
        <p class="text-xs text-gray-400 text-center">
          {{ t('setup.welcome.hearingDesc') }}
        </p>
      </div>
      <div class="pipeline-card bg-gray-700 rounded-xl p-5 flex flex-col items-center gap-3">
        <Mic
          :size="32"
          class="text-green-300"
        />
        <h3 class="font-semibold text-green-300">
          {{ t('dashboard.speakingChain') }}
        </h3>
        <p class="text-xs text-gray-400 text-center">
          {{ t('setup.welcome.speakingDesc') }}
        </p>
      </div>
      <div class="pipeline-card bg-gray-700 rounded-xl p-5 flex flex-col items-center gap-3">
        <Video
          :size="32"
          class="text-purple-300"
        />
        <h3 class="font-semibold text-purple-300">
          {{ t('dashboard.videoChain') }}
        </h3>
        <p class="text-xs text-gray-400 text-center">
          {{ t('setup.welcome.videoDesc') }}
        </p>
      </div>
    </div>

    <p class="mt-8 text-gray-500 text-sm">
      {{ t('setup.welcome.hint') }}
    </p>
  </div>
</template>
