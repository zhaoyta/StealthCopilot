# Capability Spec: resume-manager

## Purpose

提供本地简历文件的上传、Embedding 生成与多份简历管理能力。所有简历数据和向量数据均在本地处理存储，不上传至云端，保护用户隐私。

---

## Requirements

### Requirement: 简历上传与本地 Embedding
用户 SHALL 可上传 PDF 或 DOCX 格式简历，上传后 Go 后端立即在后台线程使用 multilingual-e5-small 模型生成 embedding，存入本地向量库，不上传至任何云端。

#### Scenario: 上传简历触发 Embedding
- **WHEN** 用户上传一份简历文件
- **THEN** 文件保存至本地，后台异步生成 embedding，UI 显示"处理中"，完成后变为"已就绪"

#### Scenario: 支持的文件格式
- **WHEN** 用户尝试上传非 PDF/DOCX 文件
- **THEN** 前端拒绝选择，提示"仅支持 PDF / DOCX 格式"

### Requirement: 多份简历管理与激活切换
系统 SHALL 支持同时存储多份简历，每次 RAG 检索仅使用当前激活的简历。用户可随时切换激活简历，切换立即生效。

#### Scenario: 切换激活简历
- **WHEN** 用户点击某份简历的"设为激活"
- **THEN** 该简历标记为激活，之前激活的简历取消标记，后续 RAG 检索使用新激活的简历

#### Scenario: 删除简历
- **WHEN** 用户删除某份简历
- **THEN** 对应文件和 embedding 数据从本地删除；若删除的是当前激活简历，RAG 暂停直到用户重新激活一份
