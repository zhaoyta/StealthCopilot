import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import i18n from '../../i18n'
import TabHistory from './TabHistory.vue'

const endedSession = {
  id: 'session-ended',
  started_at: Date.UTC(2026, 5, 21, 9, 30),
  ended_at: Date.UTC(2026, 5, 21, 10, 30),
  resume_id: 'resume-1',
  resume_name: 'Backend Resume',
  turn_count: 1,
}

const activeSession = {
  id: 'session-active',
  started_at: Date.UTC(2026, 5, 21, 11, 0),
  resume_id: 'resume-2',
  resume_name: 'Frontend Resume',
  turn_count: 0,
}

const turns = [
  {
    id: 1,
    session_id: 'session-ended',
    question: 'Tell me about the cache.',
    display_question: '请介绍缓存项目。',
    answer: 'I designed a cache layer with clear invalidation rules.',
    created_at: Date.UTC(2026, 5, 21, 9, 35),
  },
]

function installWailsMock(options?: { deleteError?: string }) {
  const app = {
    ListSessions: vi.fn().mockResolvedValue([endedSession, activeSession]),
    GetSessionTurns: vi.fn().mockResolvedValue(turns),
    DeleteSession: vi.fn().mockResolvedValue(options?.deleteError || ''),
  }

  Object.defineProperty(window, 'go', {
    configurable: true,
    value: { main: { App: app } },
  })

  return app
}

function mountHistory() {
  return mount(TabHistory, {
    props: { isActive: true },
    global: {
      plugins: [i18n],
      stubs: {
        ChevronDown: true,
        ChevronRight: true,
        Trash2: true,
      },
    },
  })
}

async function flushPromises() {
  await Promise.resolve()
  await Promise.resolve()
  await new Promise(resolve => setTimeout(resolve, 0))
  await nextTick()
}

describe('TabHistory', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    i18n.global.locale.value = 'zh-CN'
  })

  it('renders sessions loaded from Wails', async () => {
    const app = installWailsMock()
    const wrapper = mountHistory()
    await flushPromises()

    expect(app.ListSessions).toHaveBeenCalledWith(100)
    expect(wrapper.text()).toContain('Backend Resume')
    expect(wrapper.text()).toContain('Frontend Resume')
    expect(wrapper.text()).toContain('1 轮问答')
    expect(wrapper.text()).toContain('进行中')
  })

  it('expands a session and renders its turns', async () => {
    const app = installWailsMock()
    const wrapper = mountHistory()
    await flushPromises()

    await wrapper.findAll('button')[1].trigger('click')
    await flushPromises()

    expect(app.GetSessionTurns).toHaveBeenCalledWith('session-ended')
    expect(wrapper.text()).toContain('请介绍缓存项目。')
    expect(wrapper.text()).toContain('I designed a cache layer with clear invalidation rules.')
  })

  it('confirms and deletes an ended session', async () => {
    const app = installWailsMock()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mountHistory()
    await flushPromises()

    await wrapper.findAll('button')[2].trigger('click')
    await flushPromises()

    expect(window.confirm).toHaveBeenCalledWith('确定删除这场历史会话吗？')
    expect(app.DeleteSession).toHaveBeenCalledWith('session-ended')
    expect(wrapper.text()).not.toContain('Backend Resume')
    expect(wrapper.text()).toContain('Frontend Resume')
  })

  it('does not delete an active session', async () => {
    const app = installWailsMock()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mountHistory()
    await flushPromises()

    const activeDeleteButton = wrapper.findAll('button')[4]
    expect(activeDeleteButton.attributes('disabled')).toBeDefined()

    await activeDeleteButton.trigger('click')
    await flushPromises()

    expect(window.confirm).not.toHaveBeenCalled()
    expect(app.DeleteSession).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Frontend Resume')
  })
})
