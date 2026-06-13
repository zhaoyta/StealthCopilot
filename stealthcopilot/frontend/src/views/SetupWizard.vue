<script lang="ts" setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check } from 'lucide-vue-next'
import Step1Welcome from './setup/Step1Welcome.vue'
import Step2Deps from './setup/Step2Deps.vue'
import Step3ApiKeys from './setup/Step3ApiKeys.vue'
import Step4Voice from './setup/Step4Voice.vue'
import Step5Done from './setup/Step5Done.vue'

const emit = defineEmits<{ (e: 'complete'): void }>()

const { t } = useI18n()
const currentStep = ref(1)
const totalSteps = 5

const steps = [
  { label: t('setup.welcome.stepLabel') },
  { label: t('setup.deps.title') },
  { label: t('setup.apiKeys.title') },
  { label: t('setup.voice.title') },
  { label: t('common.finish') },
]

const canGoBack = computed(() => currentStep.value > 1)

function next() {
  if (currentStep.value < totalSteps) currentStep.value++
}
function back() {
  if (currentStep.value > 1) currentStep.value--
}
function finish() {
  emit('complete')
}
</script>

<template>
  <div class="setup-wizard flex flex-col items-center min-h-screen bg-gray-900 text-white p-8">
    <!-- 标题 -->
    <div class="mb-8 text-center">
      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
      <h1 class="text-3xl font-bold text-blue-400">
        StealthCopilot
      </h1>
      <p class="text-gray-400 mt-1">
        {{ t('setup.title') }}
      </p>
    </div>

    <!-- 步骤条 -->
    <div class="step-bar flex items-center mb-10 w-full max-w-2xl">
      <template
        v-for="(step, idx) in steps"
        :key="idx"
      >
        <div class="flex flex-col items-center">
          <div
            class="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-colors"
            :class="idx + 1 < currentStep
              ? 'bg-blue-500 text-white'
              : idx + 1 === currentStep
                ? 'bg-blue-400 text-white ring-2 ring-blue-300'
                : 'bg-gray-700 text-gray-400'"
          >
            <Check
              v-if="idx + 1 < currentStep"
              :size="14"
            />
            <span v-else>{{ idx + 1 }}</span>
          </div>
          <span class="text-xs mt-1 text-gray-400 hidden sm:block">{{ step.label }}</span>
        </div>
        <div
          v-if="idx < steps.length - 1"
          class="flex-1 h-0.5 mx-1 transition-colors"
          :class="idx + 1 < currentStep ? 'bg-blue-500' : 'bg-gray-700'"
        />
      </template>
    </div>

    <!-- 步骤内容区 -->
    <div class="step-content w-full max-w-2xl bg-gray-800 rounded-2xl p-8 shadow-xl min-h-80">
      <Step1Welcome v-if="currentStep === 1" />
      <Step2Deps v-else-if="currentStep === 2" />
      <Step3ApiKeys v-else-if="currentStep === 3" />
      <Step4Voice v-else-if="currentStep === 4" />
      <Step5Done v-else-if="currentStep === 5" />
    </div>

    <!-- 导航按钮 -->
    <div class="nav-buttons flex gap-4 mt-8">
      <button
        v-if="canGoBack"
        class="px-6 py-2 rounded-lg bg-gray-700 hover:bg-gray-600 transition-colors"
        @click="back"
      >
        {{ t('common.back') }}
      </button>
      <button
        v-if="currentStep < totalSteps"
        class="px-8 py-2 rounded-lg bg-blue-500 hover:bg-blue-600 transition-colors font-semibold"
        @click="next"
      >
        {{ t('common.next') }}
      </button>
      <button
        v-else
        class="px-8 py-2 rounded-lg bg-green-500 hover:bg-green-600 transition-colors font-semibold"
        @click="finish"
      >
        {{ t('common.finish') }}
      </button>
    </div>

    <!-- 步骤提示 -->
    <p class="mt-4 text-gray-500 text-sm">
      {{ t('setup.step', { current: currentStep, total: totalSteps }) }}
    </p>
  </div>
</template>
