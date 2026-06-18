# Capability Spec: keyring-storage

## Purpose

提供跨平台的 API Key 安全存储能力，封装 go-keyring，统一 macOS Keychain 与 Windows Credential Manager 的读写接口，确保所有敏感凭证不以明文写入配置文件。

---

## Requirements

### Requirement: 统一 Keyring 接口
Go 后端 SHALL 提供统一的 `KeyringStore` 接口封装 go-keyring，调用方不感知平台差异（macOS Keychain / Windows Credential Manager）。所有 API Key、Task ID 和 Asset ID 通过此接口读写，禁止写入明文配置文件。

#### Scenario: 跨平台存储
- **WHEN** Go 后端调用 KeyringStore.Set(key, value)
- **THEN** macOS 上写入系统 Keychain，Windows 上写入 Credential Manager，无需调用方判断平台

#### Scenario: Key 不存在时的处理
- **WHEN** 调用 KeyringStore.Get(key) 且该 key 尚未设置
- **THEN** 返回空字符串和 ErrNotFound，不 panic

### Requirement: 应用启动时预加载配置
应用启动时 SHALL 从 Keychain 读取所有已存 API Key 和配置项，缓存在内存中供各管道使用，避免每次调用都读 Keychain（Keychain 读取有 I/O 开销）。

#### Scenario: 启动预加载
- **WHEN** 应用启动
- **THEN** 在 2s 内完成所有 Keychain 读取，缓存到内存配置结构体
