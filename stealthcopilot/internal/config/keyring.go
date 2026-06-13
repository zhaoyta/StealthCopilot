// Package config 实现配置管理，包括 API Key 的安全存储。
package config

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keyringSvcName 是所有密钥在系统密钥链中的服务命名空间。
const keyringSvcName = "stealthcopilot"

// ErrNotFound 表示请求的 key 在密钥链中不存在。
var ErrNotFound = errors.New("key not found in keyring")

// KeyringStore 封装 go-keyring，提供跨平台统一接口。
// macOS → Keychain，Windows → Credential Manager，调用方无需感知平台差异。
type KeyringStore struct{}

// NewKeyringStore 创建一个 KeyringStore 实例。
func NewKeyringStore() *KeyringStore {
	return &KeyringStore{}
}

// Set 将 key-value 写入系统密钥链。
// value 为空时等效于删除该 key（保持幂等）。
func (k *KeyringStore) Set(key, value string) error {
	if value == "" {
		_ = keyring.Delete(keyringSvcName, key)
		return nil
	}
	return keyring.Set(keyringSvcName, key, value)
}

// Get 从系统密钥链读取 key 对应的值。
// key 不存在时返回 ("", ErrNotFound)，不 panic。
func (k *KeyringStore) Get(key string) (string, error) {
	v, err := keyring.Get(keyringSvcName, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return v, nil
}

// Delete 从系统密钥链删除指定 key。
// key 不存在时不报错（幂等）。
func (k *KeyringStore) Delete(key string) error {
	err := keyring.Delete(keyringSvcName, key)
	if err != nil && errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
