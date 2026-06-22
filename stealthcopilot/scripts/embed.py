#!/usr/bin/env python3
"""
embed.py — 将单段文本编码为 multilingual-e5-small 向量。

用法（普通模式）：python3 embed.py "<text>"
  输出：JSON 格式的 float32 列表，长度固定为 384，写入 stdout。

用法（下载模式）：python3 embed.py --download
  输出：JSON 进度行（每行一个事件）直到 {"done": true}，写入 stdout。
  使用 huggingface_hub.snapshot_download(tqdm_class=...) 以确保进度事件可靠触发。

stderr 仅在发生错误时有内容。
"""
import sys
import json


def _emit(data: dict):
    """向 stdout 写入一行 JSON 事件（flush 保证实时性）。"""
    print(json.dumps(data), flush=True)


def _run_download_mode():
    """
    下载模式：通过 snapshot_download(tqdm_class=...) 下载模型并输出 JSON 进度。

    不使用全局 tqdm patch，因为 huggingface_hub 在导入时已绑定 tqdm 引用，
    全局替换无法影响已导入的代码。直接传 tqdm_class 是唯一可靠方式。
    若模型已全部缓存，snapshot_download 不产生任何 tqdm 更新，直接输出 done。
    """
    state = {'downloaded': 0, 'total': 0}

    try:
        import tqdm as _tqdm
        _BaseTqdm = _tqdm.tqdm

        class _JsonTqdm(_BaseTqdm):
            """将每次 update 转换为 JSON 进度行，禁止终端输出。"""

            def __init__(self, *args, **kwargs):
                total = kwargs.get('total')
                kwargs['disable'] = True   # 不写终端
                super().__init__(*args, **kwargs)
                if total and total > 0:
                    state['total'] += total

            def update(self, n=1):
                # disable=True 时父类 update 是 no-op，直接手动累加
                state['downloaded'] += n
                _emit({'downloaded': state['downloaded'], 'total': state['total']})

            def __enter__(self):
                return self

            def __exit__(self, *_args):
                pass

        json_tqdm_class = _JsonTqdm

    except ImportError:
        # tqdm 不可用时静默下载，仅输出 done
        json_tqdm_class = None

    try:
        from huggingface_hub import snapshot_download
    except ImportError as e:
        _emit({'error': f'huggingface_hub not found: {e}'})
        sys.exit(2)

    try:
        kwargs = {'repo_id': 'intfloat/multilingual-e5-small'}
        if json_tqdm_class is not None:
            # tqdm_class 由 huggingface_hub 传入每个文件的下载调用，
            # 比全局 patch 更可靠。需要 huggingface_hub >= 0.14
            kwargs['tqdm_class'] = json_tqdm_class
        snapshot_download(**kwargs)
        _emit({'done': True})
    except Exception as e:
        _emit({'error': str(e)})
        sys.exit(1)


def main():
    if len(sys.argv) < 2:
        print('usage: embed.py <text>  |  embed.py --download', file=sys.stderr)
        sys.exit(1)

    if sys.argv[1] == '--download':
        _run_download_mode()
        return

    text = sys.argv[1]

    try:
        from sentence_transformers import SentenceTransformer
        import torch  # noqa: F401
    except ImportError as e:
        print(f'import error: {e}\nrun: pip install sentence-transformers torch', file=sys.stderr)
        sys.exit(2)

    # 模型首次使用时自动从 HuggingFace 下载，后续从本地缓存加载
    model = SentenceTransformer('intfloat/multilingual-e5-small')

    # normalize_embeddings=True 使向量已归一化，余弦相似度 = 内积
    embedding = model.encode(text, normalize_embeddings=True)

    print(json.dumps(embedding.tolist()))


if __name__ == '__main__':
    main()
