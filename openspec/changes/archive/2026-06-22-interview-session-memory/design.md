## Context

当前 `AnswerGenerator` 已经支持按 `sessionID` 维护内存级对话历史（`map[string][]QAPair`，最多 3 轮）。`hearing.Chain.Start()` 每次调用生成新 `sessionID`，`WithHistory: intentType == intent.IntentFollowup` 控制是否注入历史。

**现有问题**：
- 历史仅存于内存，应用重启/会话结束即丢失
- 没有会话边界概念（无开始时间、结束时间、会话元数据）
- 用户无法回顾过往面试问答
- `historyMaxTurns = 3` 写死在代码中，无法配置

**利益相关方**：求职用户（回顾复盘）、面试辅助主流程（连续性回答）

## Goals / Non-Goals

**Goals:**
- 每场面试的 Q&A 历史自动持久化到本地 SQLite（与 vectors.db 同级）
- 后端预留同一会话续接能力（hearing chain 重启时可传入已有 session ID），v1 前端默认新建会话
- 提供「历史」Tab，展示历史会话列表及每场会话的问答详情
- 会话可手动删除；保留轮数上限可在设置面板配置
- 回答生成时，若当前 session 有持久化历史，自动恢复注入上下文

**Non-Goals:**
- 会话历史云同步（始终本地）
- 跨设备访问
- 历史数据导出（v1 不做）
- 对 RAG 检索逻辑本身做改动
- 重写意图识别/问题完整性判断（questionAccumulator、`ClassifyResult.Complete` 等另开 change）
- 提词窗窗口尺寸、拖拽调整等原生窗口能力（另开 change）

## Decisions

### D1 — 存储层：SQLite（sessions.db）

**选择**：新建 `sessions.db`（与 `vectors.db` 分离），`modernc.org/sqlite`（pure-Go，已有依赖）

**理由**：
- 关系型表结构（sessions + turns）天然支持外键约束、时间排序、批量删除
- 分离 DB 文件避免 vectors.db schema 污染，可独立清理
- 已有 SQLite 驱动，无新依赖

**替代方案**：追加写 JSONL 文件 → 无法高效按时间排序或删除单条；追加到 resumes.json → 文件职责混乱

### D2 — 表结构

```sql
CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  started_at INTEGER NOT NULL,  -- Unix ms
  ended_at   INTEGER,           -- NULL = 进行中
  resume_id  TEXT,              -- 快照关联的简历 ID
  label      TEXT               -- 预留：用户自定义标签（v1 不暴露 UI）
);

CREATE TABLE turns (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id   TEXT    NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  question     TEXT    NOT NULL,  -- 原始问题文本（src_text）
  display_question TEXT NOT NULL, -- 历史 Tab 展示文本；优先使用 dst_text，缺失时回退 src_text
  answer       TEXT    NOT NULL,  -- 英文回答建议
  created_at   INTEGER NOT NULL   -- Unix ms
);
```

**理由**：`display_question` 单独存储，历史 Tab 可直接展示用户看到的字幕文本；当翻译扩展禁用或失败时回退为原始问题，避免把“中文问题”写死到 schema。`label` 预留但 v1 不暴露 UI，schema 一次到位避免后续 migration。`ended_at NULL` 标识进行中会话，可安全重续。

### D3 — 会话隔离：一场面试一套历史，多场面试之间完全隔离

**核心约束**：每次 `hearing.Chain.Start()` 默认生成新 `sessionID`，新面试从空历史开始，绝不自动继承上一场会话的问答历史。

- `hearing.Chain.Start()` 开始时：生成新 UUID 作为 sessionID（默认），调用 `SessionStore.Begin(sessionID, resumeID)` 写入 sessions 表
- `hearing.Chain.Stop()` 时：调用 `SessionStore.End(sessionID)` 更新 `ended_at`
- **历史注入**：`AnswerGenerator.Generate` 对同一 session 内所有 question/followup 意图均注入该 session 的历史（不再区分意图类型），因为面试中的问题天然有连续性；`WithHistory` 字段改为"session 内有历史则注入"逻辑，而非依赖意图分类结果
- **跨 session 隔离**：`GetRecentTurns(sessionID, limit)` 按 session_id 精确过滤，不可能读到其他 session 的 turns；前端也不传递历史，历史完全由后端按 sessionID 管理
- **续接（v1 后端预留）**：`hearing.Chain.Start()` 可接受已有 `sessionID`，但 v1 前端不暴露「继续此次面试」。默认路径永远新建 session；后续如开放续接，需要补充 UI 入口和 ended_at 重开规则。

**替代方案**：`WithHistory` 继续由意图类型控制 → 同一面试中普通问题无法感知前面已说的内容，导致前后矛盾；保留内存 map + 异步写库 → 两套历史来源难以一致

### D4 — 历史上限从代码常量改为可配置

`historyMaxTurns` 移动到 `llm.Config` 中（带默认值 5），由设置面板「高级」区域控制，前后端均通过 binding 读写。

### D5 — `internal/session/` 新包职责边界

`session` 包只负责：SessionStore（SQLite CRUD）、Session 和 Turn 数据模型、SessionService（Wails binding 层）。不引入 hearing 或 llm 包，避免循环依赖。`hearing.ChainConfig` 和 `llm.AnswerGenerator` 接受 `session.Store` 接口。

### D6 — 听力链触发策略保持现状

**选择**：v1 不重写意图识别和问题累积缓冲。现有链路仍在 `result.IsFinal && result.SrcText != ""` 时调用 `startRAG`，由现有 `Classifier` 判断 question/followup/statement。

**理由**：
- 历史会话持久化的核心价值可以独立交付，不需要同时改 RAG 触发时机
- 当前听力链刚经过 ASR/翻译/TTS 拆分和句子缓冲修复，避免在同一 change 中再次扩大并发和取消语义
- 多句问题完整性判断是合理的后续优化，但需要独立设计、日志验证和回归测试

**Stop 行为**：用户主动停止听力链时，不额外触发新的 RAG；已经进入 `AnswerGenerator.Generate` 的回答若正常完成，仍按本 change 的规则写入 turns 表。

## Risks / Trade-offs

- **并发写入**：单个 session 可能同时有多轮回答完成并追加 turns → SQLite WAL 模式 + 短事务写入；若实际测试出现 locked，再引入单写 goroutine
- **会话孤儿**：应用崩溃时 `ended_at` 永远为 NULL → 启动时将超过 24h 仍未结束的会话自动标记为结束，避免历史 Tab 显示"进行中"乱象
- **DB 膨胀**：长期积累大量 turns → v1 提供单场删除；批量清理 N 天前会话留到 v2
- **内存 map 废弃过渡**：`AnswerGenerator.history` 改为从 `SessionStore` 读取后，旧内存 map 代码冗余 → 一并删除，不保留死代码（遵守重构规范）
- **进行中会话删除**：历史 Tab 删除当前进行中的 session 可能导致后续 AppendTurn 失败 → v1 禁止删除 ended_at 为 NULL 的 session，或后端返回明确错误

## Migration Plan

1. 新包 `internal/session/` 实现 Store + Service，无破坏性改动
2. `llm.Config` 新增 `HistoryMaxTurns int` 字段（默认值 5，向下兼容，旧配置读取到 0 时使用默认值）
3. `llm.AnswerGenerator` 接受可选 `session.Store` 依赖注入；`history map` 保留但不再写入（过渡期），`GetRecentTurns` 优先
4. `hearing.ChainConfig` 新增 `SessionStore session.Store` 和可选 `ResumeSessionID`；Start/Stop 调用 Begin/End
5. 前端新增「历史」Tab，绑定 `ListSessions` / `GetSessionTurns` / `DeleteSession`
6. 完成集成测试后删除 `AnswerGenerator.history map` 死代码
7. 回滚：session 包独立，删除即可回到原状态；DB 文件可直接删除重置

## Open Questions

- 前端「历史」Tab 是否需要搜索/过滤（按日期、按简历）？建议 v1 仅按时间倒序列表，v2 加筛选
- 是否支持给历史会话打标签（「阿里面试」「Google 一面」）？建议 v1 sessions 表预留 `label TEXT` 字段但不暴露 UI
- 是否开放「继续此次面试」？建议 v1 仅后端预留，前端等真实用户路径明确后再暴露
