package resume

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const embeddingDim = 1024 // multilingual-e5-large 向量维度

// ErrModelNotReady 表示 Python 环境或模型尚未就绪。
var ErrModelNotReady = errors.New("embedding model not ready: run setup to install dependencies")

// EmbeddingProvider 是 embedding 功能的抽象接口，支持多种实现替换。
type EmbeddingProvider interface {
	// Embed 对给定文本生成归一化 embedding 向量。
	// 返回的 float32 切片长度固定为 embeddingDim。
	Embed(text string) ([]float32, error)
	// Ready 检查当前提供者是否可用（Python 环境、模型文件等）。
	Ready() bool
}

// PythonBridgeProvider 通过调用 Python 子进程实现 embedding，
// 使用 sentence-transformers 库加载 multilingual-e5-large 模型。
// 需要 Python3 + `pip install sentence-transformers` 已就绪。
type PythonBridgeProvider struct {
	scriptPath string // embed.py 脚本的绝对路径
	python     string // python 可执行文件名（默认 python3）
}

// NewPythonBridgeProvider 创建 Python 桥接 embedding 提供者。
// scriptPath 是随应用分发的 embed.py 脚本路径。
func NewPythonBridgeProvider(scriptPath string) *PythonBridgeProvider {
	return &PythonBridgeProvider{
		scriptPath: scriptPath,
		python:     "python3",
	}
}

// Ready 检测 Python3 和 sentence-transformers 是否可用。
func (p *PythonBridgeProvider) Ready() bool {
	cmd := exec.Command(p.python, "-c", "import sentence_transformers")
	return cmd.Run() == nil
}

// Embed 调用 Python 脚本对文本生成 embedding 向量（blocking，约 500ms~2s）。
// 文本会自动添加 "query: " 前缀（e5 模型推荐格式）。
func (p *PythonBridgeProvider) Embed(text string) ([]float32, error) {
	if !p.Ready() {
		return nil, ErrModelNotReady
	}
	// e5 模型推荐对查询文本加 "query: " 前缀
	input := "query: " + strings.TrimSpace(text)
	cmd := exec.Command(p.python, p.scriptPath, input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("embedding script error: %w\nstderr: %s", err, stderr.String())
	}

	var vec []float32
	if err := json.Unmarshal(stdout.Bytes(), &vec); err != nil {
		return nil, fmt.Errorf("embedding: parse output: %w", err)
	}
	if len(vec) != embeddingDim {
		return nil, fmt.Errorf("embedding: expected %d dims, got %d", embeddingDim, len(vec))
	}
	return vec, nil
}

// EmbedPassage 对段落文本生成 embedding，加 "passage: " 前缀（e5 模型对文档段落的推荐格式）。
func (p *PythonBridgeProvider) EmbedPassage(text string) ([]float32, error) {
	if !p.Ready() {
		return nil, ErrModelNotReady
	}
	input := "passage: " + strings.TrimSpace(text)
	cmd := exec.Command(p.python, p.scriptPath, input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("embedding script error: %w\nstderr: %s", err, stderr.String())
	}

	var vec []float32
	if err := json.Unmarshal(stdout.Bytes(), &vec); err != nil {
		return nil, fmt.Errorf("embedding: parse output: %w", err)
	}
	return vec, nil
}

// NullProvider 是 embedding 不可用时的空实现，始终返回 ErrModelNotReady。
// 用于 RAG 系统的优雅降级。
type NullProvider struct{}

// Ready 始终返回 false。
func (n *NullProvider) Ready() bool { return false }

// Embed 始终返回 ErrModelNotReady。
func (n *NullProvider) Embed(_ string) ([]float32, error) { return nil, ErrModelNotReady }
