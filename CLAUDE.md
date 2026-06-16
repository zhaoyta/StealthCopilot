# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with this repository.

## Project Overview

**StealthCopilot** — 面向跨境求职者的面试辅助 SaaS。用户在后台用中文作答，面试官听到的是流利英文音色克隆语音、看到的是口型对齐的真人视频画面，同时有幽灵提词窗（对面试官完全不可见）展示实时翻译与回答建议。

This is a **greenfield project** — the `openspec/specs/prd.md` is the current spec, no implementation exists yet.

---

## Tech Stack Decisions (Finalized)

| Layer | Technology |
|---|---|
| Desktop framework | **Wails** (Go + WebView) — dual-platform: macOS + Windows |
| Audio routing | BlackHole (macOS) / VB-Cable (Windows) — user-installed, no driver bundling |
| Stealth UI (ghost window) | CGO → `NSWindow` (macOS) / `user32.dll` syscall (Windows) |
| Hearing chain STT+translate | **讯飞 RTASR 实时语音转写 + 机器翻译文本接口** (`src` from ASR, `dst` from text translation; MT v1 falls back to v2 on `403 not found`) |
| Speaking chain ASR | **讯飞 RTASR + 机器翻译文本接口/DeepSeek polish** (Chinese speech → text → target-language text) |
| LLM | **DeepSeek-V3** (text polish / answer generation) |
| TTS / voice clone | **讯飞声音复刻** streaming (English text → personal voice audio) |
| Lip sync | **Simli AI** official SaaS API — **not** self-hosted MuseTalk GPU cluster |
| Virtual camera output | OBS / CoreMediaIO (macOS) / DirectShow (Windows) |
| Resume embeddings | `multilingual-e5-large` + local vector store (never uploaded to cloud) |
| Frontend i18n | `i18next` — all UI strings via locale files, never hardcoded |
| CI/CD | GitHub Actions — separate macOS runner + Windows runner for CGO cross-compilation |

---

## Architecture: Three Core Pipelines

### Pipeline 1 — Hearing Chain (≤500ms target)
```
Meeting audio → BlackHole/VB-Cable → Go audio capture
  → 讯飞 RTASR
      ├─ src (English) → 机器翻译文本接口 → dst (Chinese) → Ghost subtitle window
      └─ src (English) → RAG (resume embeddings) → DeepSeek answer suggestion → Ghost window
```

### Pipeline 2 — Speaking Chain (≤1.2s target)
```
Physical mic (Chinese speech) → 讯飞 RTASR → Chinese text
  → 机器翻译文本接口 → English text → DeepSeek-V3 polish
  → 讯飞声音复刻 TTS (streaming)
  → Virtual microphone → Interviewer hears fluent English
```
While Xunfei VoiceClone generates audio, Go backend writes **zero-PCM chunks** to the virtual mic to suppress Chinese background audio leaking to the interviewer.

### Pipeline 3 — Video / Lip Sync (≥30fps, A/V delta ≤40ms)
```
Physical camera → Go capture (OpenCV) → eye-gaze correction
  → Simli AI real-time streaming API (lip sync)
  → Virtual camera (OBS/CoreMediaIO/DirectShow) → Interviewer sees lip-synced video
```
A **ring buffer** in Go aligns audio and video timestamps before sending to Simli AI — cloud processing latency (~1s) means naive ordering would desync A/V.

---

## Ghost UI (Stealth Window)

The teleprompter overlay must be **invisible to screen capture / screen share** and support **mouse click-through**.

- **macOS (CGO):** `[window setSharingType:0]` (`NSWindowSharingNone`) + `[window setIgnoresMouseEvents:YES]`
- **Windows (Syscall):** `SetWindowDisplayAffinity(hwnd, WDA_EXCLUDEFROMCAPTURE)` + inject `WS_EX_TRANSPARENT` via `GWL_EXSTYLE`

Wails exposes the native window handle — the CGO/syscall stealth hook attaches at app startup.

---

## Failsafe / Circuit Breaker

- 50ms UDP heartbeat between client and cloud
- If 3 consecutive heartbeats lost **or** cloud video stream latency > 300ms → hard bypass in ≤10ms: disconnect cloud pipelines, reconnect real microphone + real camera directly to Zoom/Teams
- No blackout, no audio drop, no freeze allowed during bypass

---

## Settings Panel Modules

1. **API credentials:** 讯飞 RTASR (AppID/APIKey), 讯飞机器翻译（AppID/APIKey/APISecret）, 讯飞声音复刻（AppID/APIKey/APISecret/Asset ID）, DeepSeek (key + model), Simli AI (key)
2. **Language config:** Hearing chain source→target language; Speaking chain input→output language (separate dropdowns, 讯飞-supported language pairs)
3. **Device binding:** Virtual sound card, physical mic, physical camera, virtual camera (dynamically enumerated)
4. **Resume management:** Local PDF/Word upload → local embedding (never leaves device); multiple resumes, switchable
5. **Ghost window appearance:** Position, font size, opacity
6. **Advanced (collapsed):** RAG answer-generation prompt, speaking-chain polish prompt — each with a default value + one-click reset

---

## Implementation Phases (from PRD)

| Phase | Goal |
|---|---|
| **Phase 1** | Wails shell MVP: Chinese → 讯飞 STT → DeepSeek → virtual sound card output. Validate ghost UI + mouse-through hooks. |
| **Phase 2** | Cloud pipeline: Simli AI lip sync integration, ring buffer A/V sync, virtual camera output. |
| **Phase 3** | SaaS billing, "full A/V mode" vs "text-only teleprompter mode" (zero cloud cost), internal beta. |

---

## Build Notes (anticipated)

- Wails builds require CGO — macOS runner needs Xcode command-line tools; Windows runner needs MSYS2/mingw64 for CGO
- GitHub Actions must use **separate jobs** for macOS and Windows artifacts (no cross-compilation for CGO-heavy code)
- Run `wails build` for production; `wails dev` for hot-reload development

---

## 开发规范（必须严格遵守）

> 以下规范对所有代码生成、重构、功能新增均强制生效，无例外。

### 语言与沟通
- **所有回复必须使用中文**，包括代码注释、文档说明、任务进度反馈。

### 代码质量
- **状态值必须用常量定义**，禁止在代码中直接使用魔法字符串或数字表示状态（如 `"running"`、`1`），一律定义为具名常量或枚举。
- **单个代码文件不得超过 500 行**，超出时必须拆分为更小的模块。
- **所有生成的代码文件必须包含详细注释**：每个函数/方法说明其职责、参数含义、返回值、关键副作用；复杂逻辑段落内联注释说明意图。
- **重复逻辑必须收敛**，三处以上相似代码必须提取为公共函数/方法，不允许复制粘贴式实现。
- **模块必须保持独立性**，非耦合逻辑强制解耦；跨模块调用只通过定义好的接口（Provider interface），不直接引用内部实现。

### 测试
- **所有生成的代码必须同时提供单元测试**，测试文件与源文件同目录（Go: `_test.go`，Vue: `.spec.ts`）。
- **每次功能升级或 Bug 修复完成后必须运行测试套件**（`make test`），确认无回归后再标记任务完成。

### 重构规范
- **重构时必须删除无效代码**，包括未使用的函数、变量、import、注释掉的代码块，不允许保留死代码。
- **数据库 schema 变更必须向下兼容**，新增字段须有默认值，不允许直接删除或重命名已有字段，需通过迁移脚本兼容旧数据。

### 前后端职责划分
- **页面逻辑尽量简单**：枚举映射、业务定义、Prompt 模板、数据转换逻辑一律放在 Go 后端，前端只负责展示和交互，通过 Wails binding 调用后端。

### 文档同步
- **Change 内更新的逻辑必须同步写回对应的 change 文档**（proposal/design/spec/tasks），保持文档与实现一致。
- **新增功能必须同步更新客户端帮助文档**（`docs/` 目录），说明按产品逻辑撰写，不体现技术名词和技术细节；需要截图的地方用占位符 `[截图：XXX]` 标注。
- **propose 阶段必须询问用户是否同时生成 UI 原型**，确认后再执行。
