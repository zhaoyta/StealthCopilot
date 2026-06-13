## 1. Keyring 存储

- [x] 1.1 添加 go-keyring 依赖（`go get github.com/zalando/go-keyring`）
- [x] 1.2 在 `internal/config/keyring.go` 实现 `KeyringStore` 结构体，封装 Set / Get / Delete 方法
- [x] 1.3 在应用启动时预加载所有 Key 到内存配置结构体 `internal/config/config.go`
- [x] 1.4 Wails 暴露 `GetConfig` / `SaveApiKey` 等 binding 供前端调用

## 2. 简历管理后端

- [x] 2.1 在 `internal/resume/` 实现文件存储（PDF/DOCX 解析、本地路径管理）
- [x] 2.2 集成 multilingual-e5-large 模型（调用本地推理或 Go-Python 桥接）
- [x] 2.3 实现本地向量库（sqlite-vss 或 hnswlib Go binding），存储 embedding
- [x] 2.4 实现激活简历管理接口（Set/Get active resume ID）
- [x] 2.5 Wails 暴露简历管理 binding（ListResumes / UploadResume / DeleteResume / SetActiveResume）

## 3. Setup 向导 Vue 组件

- [x] 3.1 创建 `src/views/SetupWizard.vue`，实现 5 步骤容器（步骤条 + 内容区 + 上一步/下一步）
- [x] 3.2 实现 Step 1 欢迎页（产品介绍 + 三管道图标）
- [x] 3.3 实现 Step 2 依赖检测（调用 Go 后端检测 BlackHole / 虚拟摄像头，显示安装进度）
- [x] 3.4 实现 Step 3 API Key 录入（讯飞/DeepSeek 必填，ElevenLabs/Simli 可选）
- [x] 3.5 实现 Step 4 声音克隆（录音波形动画、倒计时、上传进度条，可跳过）
- [x] 3.6 实现 Step 5 完成页（汇总已完成项，进入主界面按钮）
- [x] 3.7 在 `App.vue` 中根据初始化标记决定显示 SetupWizard 还是主界面

## 4. 设置面板 Vue 组件

- [x] 4.1 创建 `src/views/Settings.vue`，实现左侧 Tab 导航 + 右侧内容区布局
- [x] 4.2 实现 Tab API 凭证（4 个服务的密码输入框 + 显示/隐藏切换 + 连接测试按钮）
- [x] 4.3 实现 Tab 语言配置（听力链/说话链各一对独立语言下拉）
- [x] 4.4 实现 Tab 设备绑定（4 类设备下拉 + 重新枚举按钮，调用 Go 后端枚举接口）
- [x] 4.5 实现 Tab 简历管理（上传拖拽区 + 简历列表卡片 + 激活切换 + 删除）
- [x] 4.6 实现 Tab 提词窗外观（字号/透明度滑块 + 位置预设 + 实时预览）
- [x] 4.7 实现 Tab 高级（RAG Prompt + 润色 Prompt 折叠面板 + 恢复默认按钮）
- [x] 4.8 从主界面导航栏添加跳转设置面板的入口
