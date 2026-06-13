package resume

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"sort"

	// SQLite CGO 驱动，编译时需要 CGO_ENABLED=1
	_ "github.com/mattn/go-sqlite3"
)

const dbFileName = "vectors.db"

// SearchResult 表示向量相似度搜索的单条结果。
type SearchResult struct {
	ResumeID  string  // 所属简历 ID
	ChunkID   int64   // 文本分块序号
	ChunkText string  // 分块原文
	Score     float64 // 余弦相似度（0~1，越高越相关）
}

// vectorStore 使用 SQLite 存储文本分块和 embedding 向量，
// 并在 Go 层计算余弦相似度（适合小数据集，无需 sqlite-vss 扩展）。
type vectorStore struct {
	db *sql.DB
}

// newVectorStore 初始化向量库，dbPath 对应的 SQLite 文件不存在时自动创建。
func newVectorStore(dataDir string) (*vectorStore, error) {
	dbPath := filepath.Join(dataDir, dbFileName)
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("vectorStore: open db: %w", err)
	}
	vs := &vectorStore{db: db}
	if err := vs.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return vs, nil
}

// migrate 创建必要的表结构（幂等）。
func (vs *vectorStore) migrate() error {
	_, err := vs.db.Exec(`
		CREATE TABLE IF NOT EXISTS chunks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			resume_id  TEXT    NOT NULL,
			chunk_idx  INTEGER NOT NULL,
			text       TEXT    NOT NULL,
			embedding  BLOB    NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_chunks_resume ON chunks(resume_id);
	`)
	return err
}

// UpsertChunks 将简历的所有文本分块及其向量写入（先删后插，保证幂等）。
func (vs *vectorStore) UpsertChunks(resumeID string, chunks []string, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("vectorStore: chunks/embeddings count mismatch")
	}
	tx, err := vs.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM chunks WHERE resume_id = ?", resumeID); err != nil {
		return err
	}
	for i, chunk := range chunks {
		blob := float32SliceToBlob(embeddings[i])
		if _, err := tx.Exec(
			"INSERT INTO chunks(resume_id, chunk_idx, text, embedding) VALUES(?,?,?,?)",
			resumeID, i, chunk, blob,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteResume 删除指定简历的所有分块数据。
func (vs *vectorStore) DeleteResume(resumeID string) error {
	_, err := vs.db.Exec("DELETE FROM chunks WHERE resume_id = ?", resumeID)
	return err
}

// Search 在指定简历中搜索与 query 向量最相关的 topK 分块。
// 若 resumeID 为空则搜索所有简历。
func (vs *vectorStore) Search(query []float32, resumeID string, topK int) ([]SearchResult, error) {
	var rows *sql.Rows
	var err error
	if resumeID != "" {
		rows, err = vs.db.Query(
			"SELECT id, resume_id, text, embedding FROM chunks WHERE resume_id = ?", resumeID,
		)
	} else {
		rows, err = vs.db.Query("SELECT id, resume_id, text, embedding FROM chunks")
	}
	if err != nil {
		return nil, fmt.Errorf("vectorStore.Search: query: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		chunkID   int64
		resumeID  string
		text      string
		score     float64
	}
	var cands []candidate

	for rows.Next() {
		var (
			chunkID  int64
			rID      string
			text     string
			blob     []byte
		)
		if err := rows.Scan(&chunkID, &rID, &text, &blob); err != nil {
			return nil, err
		}
		vec := blobToFloat32Slice(blob)
		score := cosineSimilarity(query, vec)
		cands = append(cands, candidate{chunkID: chunkID, resumeID: rID, text: text, score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	if topK > len(cands) {
		topK = len(cands)
	}
	results := make([]SearchResult, 0, topK)
	for _, c := range cands[:topK] {
		results = append(results, SearchResult{
			ResumeID:  c.resumeID,
			ChunkID:   c.chunkID,
			ChunkText: c.text,
			Score:     c.score,
		})
	}
	return results, nil
}

// Close 关闭数据库连接。
func (vs *vectorStore) Close() error {
	return vs.db.Close()
}

// --- 工具函数 ---

// cosineSimilarity 计算两个归一化向量的余弦相似度（内积）。
// 若 a 已归一化（L2 norm = 1），则余弦相似度 = 内积。
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	// 防止浮点误差超过 [-1, 1]
	return math.Max(-1, math.Min(1, dot))
}

// float32SliceToBlob 将 float32 切片序列化为 little-endian 字节数组。
func float32SliceToBlob(vec []float32) []byte {
	b := make([]byte, len(vec)*4)
	for i, v := range vec {
		bits := math.Float32bits(v)
		binary.LittleEndian.PutUint32(b[i*4:], bits)
	}
	return b
}

// blobToFloat32Slice 将 little-endian 字节数组反序列化为 float32 切片。
func blobToFloat32Slice(b []byte) []float32 {
	vec := make([]float32, len(b)/4)
	for i := range vec {
		bits := binary.LittleEndian.Uint32(b[i*4:])
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}
