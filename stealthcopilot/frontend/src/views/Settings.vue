<script lang="ts" setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Key, Globe, Sliders, FileText, Eye, Settings } from 'lucide-vue-next'
import TabApiKeys from './settings/TabApiKeys.vue'
import TabLanguage from './settings/TabLanguage.vue'
import TabDevices from './settings/TabDevices.vue'
import TabResume from './settings/TabResume.vue'
import TabGhost from './settings/TabGhost.vue'
import TabAdvanced from './settings/TabAdvanced.vue'

defineOptions({ name: 'AppSettings' })
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()

type TabId = 'apiKeys' | 'language' | 'devices' | 'resume' | 'ghost' | 'advanced'

const tabs: { id: TabId; label: string; icon: typeof Key }[] = [
  { id: 'apiKeys',   label: t('settings.tabs.apiKeys'),   icon: Key },
  { id: 'language',  label: t('settings.tabs.language'),  icon: Globe },
  { id: 'devices',   label: t('settings.tabs.devices'),   icon: Sliders },
  { id: 'resume',    label: t('settings.tabs.resume'),    icon: FileText },
  { id: 'ghost',     label: t('settings.tabs.ghost'),     icon: Eye },
  { id: 'advanced',  label: t('settings.tabs.advanced'),  icon: Settings },
]

const activeTab = ref<TabId>('apiKeys')
</script>

<template>
  <div class="settings-view flex flex-col min-h-screen bg-gray-900 text-white">
    <!-- 顶部栏 -->
    <div class="header flex items-center justify-between px-6 py-4 border-b border-gray-700">
      <h1 class="text-lg font-bold text-white">
        {{ t('settings.title') }}
      </h1>
      <button
        class="px-4 py-1.5 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm transition-colors"
        @click="emit('close')"
      >
        {{ t('common.close') }}
      </button>
    </div>

    <!-- 主体：左侧 Tab 导航 + 右侧内容区 -->
    <div class="body flex flex-1 overflow-hidden">
      <!-- 左侧 Tab 列表 -->
      <nav class="tab-nav w-44 flex-shrink-0 border-r border-gray-700 py-4">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          class="tab-btn w-full flex items-center gap-3 px-4 py-3 text-sm text-left transition-colors"
          :class="activeTab === tab.id
            ? 'bg-blue-600/20 text-blue-300 border-r-2 border-blue-400'
            : 'text-gray-400 hover:text-white hover:bg-gray-800'"
          @click="activeTab = tab.id"
        >
          <component
            :is="tab.icon"
            :size="15"
          />
          <span>{{ tab.label }}</span>
        </button>
      </nav>

      <!-- 右侧内容区（各 Tab 状态保留，避免切换重置） -->
      <main class="tab-content flex-1 overflow-y-auto p-6">
        <TabApiKeys v-show="activeTab === 'apiKeys'" />
        <TabLanguage v-show="activeTab === 'language'" />
        <TabDevices v-show="activeTab === 'devices'" />
        <TabResume v-show="activeTab === 'resume'" />
        <TabGhost v-show="activeTab === 'ghost'" />
        <TabAdvanced v-show="activeTab === 'advanced'" />
      </main>
    </div>
  </div>
</template>
