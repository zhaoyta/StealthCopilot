#!/usr/bin/env python3
"""
embed.py — 将单段文本编码为 multilingual-e5-large 向量。

用法：python3 embed.py "<text>"
输出：JSON 格式的 float32 列表，长度固定为 1024，写入 stdout。
stderr 仅在发生错误时有内容。
"""
import sys
import json

def main():
    if len(sys.argv) < 2:
        print("usage: embed.py <text>", file=sys.stderr)
        sys.exit(1)

    text = sys.argv[1]

    try:
        from sentence_transformers import SentenceTransformer
        import torch
    except ImportError as e:
        print(f"import error: {e}\nrun: pip install sentence-transformers", file=sys.stderr)
        sys.exit(2)

    # 模型首次使用时自动从 HuggingFace 下载（~1.1GB），后续从本地缓存加载
    model = SentenceTransformer("intfloat/multilingual-e5-large")

    # normalize_embeddings=True 使向量已归一化，余弦相似度 = 内积
    embedding = model.encode(text, normalize_embeddings=True)

    print(json.dumps(embedding.tolist()))


if __name__ == "__main__":
    main()
