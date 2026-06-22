package resume

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const embeddingDim = 384 // multilingual-e5-small 向量维度

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

// PassageEmbeddingProvider 支持对文档段落生成带 "passage: " 前缀的 embedding。
type PassageEmbeddingProvider interface {
	EmbedPassage(text string) ([]float32, error)
}

// ModelCacheChecker 可检测本地模型权重文件是否已缓存。
// PythonBridgeProvider 实现此接口，用于在 embedding 前判断是否需要先下载。
type ModelCacheChecker interface {
	IsModelCached() bool
}

// ModelDownloader 支持带进度回调的模型文件下载。
// onProgress 的参数为 (已下载字节数, 总字节数)；total 为 0 表示无法确定总量。
type ModelDownloader interface {
	DownloadModel(ctx context.Context, onProgress func(downloaded, total int64)) error
}

// downloadProgressEvent 是 embed.py --download 模式输出的单行 JSON 结构。
type downloadProgressEvent struct {
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Done       bool   `json:"done"`
	Error      string `json:"error"`
}

// PythonBridgeProvider 通过调用 Python 子进程实现 embedding，
// 使用 sentence-transformers 库加载 multilingual-e5-small 模型。
// 需要 Python3 + `pip install sentence-transformers torch` 已就绪。
type PythonBridgeProvider struct {
	scriptPath string // embed.py 脚本的绝对路径
	python     string // 缓存已检测到的 Python 可执行路径（空则每次自动检测）
}

// NewPythonBridgeProvider 创建 Python 桥接 embedding 提供者。
// scriptPath 是随应用分发的 embed.py 脚本路径。
func NewPythonBridgeProvider(scriptPath string) *PythonBridgeProvider {
	return &PythonBridgeProvider{scriptPath: scriptPath}
}

// Ready 检测 Python3 和 sentence-transformers 是否可用。
func (p *PythonBridgeProvider) Ready() bool {
	_, ok := p.pythonExecutable()
	return ok
}

// IsModelCached 检测 HuggingFace 本地缓存中是否已存在 multilingual-e5-small 权重文件。
func (p *PythonBridgeProvider) IsModelCached() bool {
	_, err := os.Stat(p.ModelCachePath())
	return err == nil
}

// ModelCachePath 返回 multilingual-e5-small 在 HuggingFace 本地缓存中的预期目录路径。
func (p *PythonBridgeProvider) ModelCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "huggingface", "hub", "models--intfloat--multilingual-e5-small")
}

// DownloadModel 运行 embed.py --download，流式读取 JSON 进度事件并通过 onProgress 回调上报。
// 若模型已缓存，Python 侧无任何 tqdm 进度，仅输出 {"done":true}，onProgress 不会被调用。
func (p *PythonBridgeProvider) DownloadModel(ctx context.Context, onProgress func(downloaded, total int64)) error {
	python, ok := p.pythonExecutable()
	if !ok {
		return ErrModelNotReady
	}

	cmd := exec.CommandContext(ctx, python, p.scriptPath, "--download")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("download model: stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("download model: start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var event downloadProgressEvent
		if jsonErr := json.Unmarshal(scanner.Bytes(), &event); jsonErr != nil {
			continue // 跳过非 JSON 行（如 tqdm 残留输出）
		}
		if event.Error != "" {
			_ = cmd.Wait()
			return fmt.Errorf("download model: python error: %s", event.Error)
		}
		if event.Done {
			break
		}
		if onProgress != nil {
			onProgress(event.Downloaded, event.Total)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("download model: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

func (p *PythonBridgeProvider) pythonExecutable() (string, bool) {
	if _, err := os.Stat(p.scriptPath); err != nil {
		return "", false
	}
	if p.python != "" && pythonHasSentenceTransformers(p.python) {
		return p.python, true
	}
	for _, candidate := range pythonCandidates() {
		if pythonHasSentenceTransformers(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func pythonCandidates() []string {
	return []string{
		"python3",
		"python3.13",
		"python3.12",
		"python3.11",
		"python3.10",
		"/opt/homebrew/bin/python3.13",
		"/opt/homebrew/bin/python3.12",
		"/opt/homebrew/bin/python3.11",
		"/opt/homebrew/bin/python3.10",
		"/usr/local/bin/python3.13",
		"/usr/local/bin/python3.12",
		"/usr/local/bin/python3.11",
		"/usr/local/bin/python3.10",
	}
}

func pythonHasSentenceTransformers(python string) bool {
	cmd := exec.Command(python, "-c", `import importlib.util as u; raise SystemExit(0 if u.find_spec("sentence_transformers") and u.find_spec("torch") else 1)`)
	return cmd.Run() == nil
}

// Embed 调用 Python 脚本对文本生成 embedding 向量（blocking，约 500ms~2s）。
// 文本会自动添加 "query: " 前缀（e5 模型推荐格式）。
func (p *PythonBridgeProvider) Embed(text string) ([]float32, error) {
	python, ok := p.pythonExecutable()
	if !ok {
		return nil, ErrModelNotReady
	}
	input := "query: " + strings.TrimSpace(text)
	return p.runScript(python, input, embeddingDim)
}

// EmbedPassage 对段落文本生成 embedding，加 "passage: " 前缀（e5 模型对文档段落的推荐格式）。
func (p *PythonBridgeProvider) EmbedPassage(text string) ([]float32, error) {
	python, ok := p.pythonExecutable()
	if !ok {
		return nil, ErrModelNotReady
	}
	input := "passage: " + strings.TrimSpace(text)
	return p.runScript(python, input, 0) // passage 不强制校验维度（允许未来换模型）
}

// runScript 调用 embed.py 并解析输出向量；expectedDim=0 时跳过维度校验。
func (p *PythonBridgeProvider) runScript(python, input string, expectedDim int) ([]float32, error) {
	cmd := exec.Command(python, p.scriptPath, input)
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
	if expectedDim > 0 && len(vec) != expectedDim {
		return nil, fmt.Errorf("embedding: expected %d dims, got %d", expectedDim, len(vec))
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
