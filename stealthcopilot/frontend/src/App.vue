<script lang="ts" setup>
// App.vue — 根组件，根据初始化状态决定显示 SetupWizard 或主界面
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Settings as SettingsIcon } from 'lucide-vue-next'
import SetupWizard from './views/SetupWizard.vue'
import Dashboard from './views/Dashboard.vue'
import Settings from './views/Settings.vue'
import Teleprompter from './views/Teleprompter.vue'
import { GetConfig, MarkSetupComplete, ShowTeleprompter } from '../wailsjs/go/main/App'

const { t } = useI18n()
type View = 'loading' | 'setup' | 'dashboard' | 'settings' | 'teleprompter'

const currentView = ref<View>('loading')

onMounted(async () => {
  const previewView = new URLSearchParams(window.location.search).get('view')
  if (previewView === 'teleprompter') {
    currentView.value = 'teleprompter'
    return
  }

  try {
    const cfg = await GetConfig()
    currentView.value = cfg.setup_completed ? 'dashboard' : 'setup'
  } catch {
    // Wails 绑定尚未就绪时（如浏览器直接预览），默认进入 setup
    currentView.value = 'setup'
  }

  // @ts-expect-error — Wails 运行时注入
  window.runtime?.EventsOn?.('teleprompter:show', () => {
    currentView.value = 'teleprompter'
  })
  // @ts-expect-error — Wails 运行时注入
  window.runtime?.EventsOn?.('teleprompter:hide', () => {
    currentView.value = 'dashboard'
  })
})

async function onSetupComplete() {
  await MarkSetupComplete()
  currentView.value = 'dashboard'
}

function openSettings() {
  currentView.value = 'settings'
}

function closeSettings() {
  currentView.value = 'dashboard'
}

async function openTeleprompter() {
  try {
    await ShowTeleprompter()
  } catch {
    currentView.value = 'teleprompter'
  }
}
</script>

<template>
  <!-- 加载中：Keychain 预读时的过渡画面 -->
  <div
    v-if="currentView === 'loading'"
    class="flex items-center justify-center min-h-screen bg-gray-900"
  >
    <div class="text-center">
      <div class="w-10 h-10 border-4 border-blue-400 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
      <p class="text-gray-400 text-sm">
        {{ t('common.loading') }}
      </p>
    </div>
  </div>

  <!-- 首次启动：5 步 Setup 向导 -->
  <SetupWizard
    v-else-if="currentView === 'setup'"
    @complete="onSetupComplete"
  />

  <!-- 主界面 -->
  <div
    v-else-if="currentView === 'dashboard'"
    class="relative"
  >
    <Dashboard
      @open-settings="openSettings"
      @open-teleprompter="openTeleprompter"
    />
    <!-- 导航栏设置入口（叠加在主界面右上角） -->
    <button
      class="fixed top-4 right-4 z-50 px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm text-gray-300 transition-colors"
      @click="openSettings"
    >
      <SettingsIcon
        :size="14"
        class="inline-block mr-1"
      />{{ t('settings.title') }}
    </button>
  </div>

  <!-- 设置面板（全屏覆盖） -->
  <Settings
    v-else-if="currentView === 'settings'"
    @close="closeSettings"
  />

  <!-- 幽灵提词窗视图 -->
  <Teleprompter
    v-else-if="currentView === 'teleprompter'"
    @close="closeSettings"
  />
</template>
