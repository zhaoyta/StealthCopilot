<script lang="ts" setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronRight, Trash2 } from 'lucide-vue-next'

const props = defineProps<{ isActive?: boolean }>()
const { t, locale } = useI18n()

interface SessionSummary {
  id: string
  started_at: number
  ended_at?: number
  resume_id?: string
  resume_name?: string
  label?: string
  turn_count: number
}

interface SessionTurn {
  id: number
  session_id: string
  question: string
  display_question: string
  answer: string
  created_at: number
}

const sessions = ref<SessionSummary[]>([])
const turns = ref<Record<string, SessionTurn[]>>({})
const expanded = ref<Record<string, boolean>>({})
const loading = ref(false)
const errMsg = ref('')

const hasSessions = computed(() => sessions.value.length > 0)

watch(() => props.isActive, active => {
  if (active) loadSessions()
})

onMounted(() => {
  if (props.isActive) loadSessions()
})

async function loadSessions() {
  loading.value = true
  errMsg.value = ''
  try {
    // @ts-expect-error — Wails 运行时注入
    sessions.value = await window.go.main.App.ListSessions(100) || []
  } catch (e: unknown) {
    errMsg.value = String(e)
    sessions.value = []
  }
  loading.value = false
}

async function toggleSession(id: string) {
  expanded.value[id] = !expanded.value[id]
  if (!expanded.value[id] || turns.value[id]) return
  try {
    // @ts-expect-error — Wails 运行时注入
    turns.value[id] = await window.go.main.App.GetSessionTurns(id) || []
  } catch (e: unknown) {
    errMsg.value = String(e)
    turns.value[id] = []
  }
}

async function deleteSession(item: SessionSummary) {
  if (!item.ended_at) return
  if (!window.confirm(t('settings.history.confirmDelete'))) return
  // @ts-expect-error — Wails 运行时注入
  const err = await window.go.main.App.DeleteSession(item.id)
  if (err) {
    errMsg.value = err
    return
  }
  delete turns.value[item.id]
  delete expanded.value[item.id]
  sessions.value = sessions.value.filter(s => s.id !== item.id)
}

function formatTime(ms: number | undefined): string {
  if (!ms) return ''
  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(ms))
}

function statusLabel(item: SessionSummary): string {
  return item.ended_at ? t('settings.history.ended') : t('settings.history.active')
}
</script>

<template>
  <div class="tab-history">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-base font-semibold text-gray-200">
        {{ t('settings.tabs.history') }}
      </h2>
      <button
        class="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded-lg text-xs transition-colors"
        :disabled="loading"
        @click="loadSessions"
      >
        {{ t('common.refresh') }}
      </button>
    </div>

    <p
      v-if="errMsg"
      class="mb-4 text-sm text-red-400"
    >
      {{ errMsg }}
    </p>

    <div
      v-if="loading && !hasSessions"
      class="py-12 text-center text-sm text-gray-400"
    >
      {{ t('common.loading') }}
    </div>

    <div
      v-else-if="!hasSessions"
      class="py-12 text-center text-sm text-gray-500 border border-dashed border-gray-700 rounded-lg"
    >
      {{ t('settings.history.empty') }}
    </div>

    <div
      v-else
      class="space-y-3"
    >
      <div
        v-for="item in sessions"
        :key="item.id"
        class="rounded-lg border border-gray-700 bg-gray-800"
      >
        <div class="flex items-center gap-3 p-4">
          <button
            class="w-7 h-7 inline-flex items-center justify-center rounded-md hover:bg-gray-700 text-gray-300"
            @click="toggleSession(item.id)"
          >
            <component
              :is="expanded[item.id] ? ChevronDown : ChevronRight"
              :size="16"
            />
          </button>

          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium text-sm text-gray-100">{{ formatTime(item.started_at) }}</span>
              <span
                class="text-[11px] px-2 py-0.5 rounded-full"
                :class="item.ended_at ? 'bg-gray-700 text-gray-300' : 'bg-green-900/50 text-green-300'"
              >
                {{ statusLabel(item) }}
              </span>
            </div>
            <div class="mt-1 text-xs text-gray-400 truncate">
              {{ t('settings.history.sessionMeta', {
                resume: item.resume_name || t('settings.history.noResume'),
                count: item.turn_count,
              }) }}
            </div>
          </div>

          <button
            class="w-8 h-8 inline-flex items-center justify-center rounded-md text-gray-400 hover:text-red-300 hover:bg-red-950/40 disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-gray-400"
            :disabled="!item.ended_at"
            :title="item.ended_at ? t('common.delete') : t('settings.history.activeDeleteDisabled')"
            @click="deleteSession(item)"
          >
            <Trash2 :size="15" />
          </button>
        </div>

        <div
          v-if="expanded[item.id]"
          class="border-t border-gray-700 px-4 pb-4"
        >
          <div
            v-if="!turns[item.id]"
            class="py-4 text-sm text-gray-400"
          >
            {{ t('common.loading') }}
          </div>
          <div
            v-else-if="turns[item.id].length === 0"
            class="py-4 text-sm text-gray-500"
          >
            {{ t('settings.history.noTurns') }}
          </div>
          <div
            v-else
            class="space-y-4 pt-4"
          >
            <div
              v-for="turn in turns[item.id]"
              :key="turn.id"
              class="space-y-2"
            >
              <div class="text-xs text-gray-500">
                {{ formatTime(turn.created_at) }}
              </div>
              <div class="rounded-md bg-gray-900/60 border border-gray-700 p-3">
                <div class="text-xs text-gray-400 mb-1">
                  {{ t('settings.history.question') }}
                </div>
                <p class="text-sm text-gray-100 whitespace-pre-wrap">
                  {{ turn.display_question }}
                </p>
              </div>
              <div class="rounded-md bg-gray-900/40 border border-gray-700 p-3">
                <div class="text-xs text-gray-400 mb-1">
                  {{ t('settings.history.answer') }}
                </div>
                <p class="text-sm text-gray-200 whitespace-pre-wrap">
                  {{ turn.answer }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
